package dockerx

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client
}

func NewClient(host string) (*Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	} else {
		opts = append(opts, client.FromEnv)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &Client{cli: cli}, nil
}

func (c *Client) List(ctx context.Context) ([]Container, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]Container, 0, len(containers))
	for _, summary := range containers {
		ins, err := c.cli.ContainerInspect(ctx, summary.ID)
		if err != nil {
			result = append(result, mapSummaryToContainer(summary))
			continue
		}
		result = append(result, mapInspectToContainer(ins))
	}
	return result, nil
}

func (c *Client) Inspect(ctx context.Context, id string) (Container, error) {
	ins, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return Container{}, fmt.Errorf("inspect container %s: %w", id, err)
	}
	return mapInspectToContainer(ins), nil
}

func (c *Client) Events(ctx context.Context) (<-chan Event, <-chan error) {
	eventChan := make(chan Event, 100)
	errChan := make(chan error, 1)

	f := filters.NewArgs()
	f.Add("type", "container")

	msgChan, dockerErrChan := c.cli.Events(ctx, events.ListOptions{Filters: f})

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-dockerErrChan:
				if !ok {
					return
				}
				select {
				case errChan <- err:
				case <-ctx.Done():
				}
				return
			case msg, ok := <-msgChan:
				if !ok {
					return
				}
				evt := Event{
					Type:       string(msg.Type),
					Action:     string(msg.Action),
					ActorID:    msg.Actor.ID,
					ActorName:  msg.Actor.Attributes["name"],
					Attributes: msg.Actor.Attributes,
					Time:       time.Unix(0, msg.TimeNano),
				}
				select {
				case eventChan <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return eventChan, errChan
}

func (c *Client) Stats(ctx context.Context, id string) (io.ReadCloser, error) {
	resp, err := c.cli.ContainerStats(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("container stats %s: %w", id, err)
	}
	return resp.Body, nil
}

func (c *Client) Logs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	options := container.LogsOptions{
		ShowStdout: opts.ShowStdout,
		ShowStderr: opts.ShowStderr,
		Timestamps: opts.Timestamps,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
	}
	if !opts.Since.IsZero() {
		options.Since = opts.Since.Format(time.RFC3339Nano)
	}
	if !opts.Until.IsZero() {
		options.Until = opts.Until.Format(time.RFC3339Nano)
	}

	reader, err := c.cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return nil, fmt.Errorf("container logs %s: %w", id, err)
	}
	return reader, nil
}

func (c *Client) Start(ctx context.Context, id string) error {
	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Stop(ctx context.Context, id string, timeout time.Duration) error {
	seconds := int(timeout.Seconds())
	stopOpts := container.StopOptions{Timeout: &seconds}
	if err := c.cli.ContainerStop(ctx, id, stopOpts); err != nil {
		return fmt.Errorf("stop container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Restart(ctx context.Context, id string, timeout time.Duration) error {
	seconds := int(timeout.Seconds())
	stopOpts := container.StopOptions{Timeout: &seconds}
	if err := c.cli.ContainerRestart(ctx, id, stopOpts); err != nil {
		return fmt.Errorf("restart container %s: %w", id, err)
	}
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("ping docker daemon: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func mapInspectToContainer(ins types.ContainerJSON) Container {
	name := ""
	if ins.ContainerJSONBase != nil {
		name = strings.TrimPrefix(ins.ContainerJSONBase.Name, "/")
	}

	ports := make([]Port, 0)
	if ins.NetworkSettings != nil {
		for p, bindings := range ins.NetworkSettings.Ports {
			for _, b := range bindings {
				var pub uint16
				fmt.Sscanf(b.HostPort, "%d", &pub)
				ports = append(ports, Port{
					IP:          b.HostIP,
					PrivatePort: uint16(p.Int()),
					PublicPort:  pub,
					Type:        p.Proto(),
				})
			}
		}
	}

	mounts := make([]string, 0, len(ins.Mounts))
	for _, m := range ins.Mounts {
		mounts = append(mounts, fmt.Sprintf("%s:%s", m.Source, m.Destination))
	}

	netMode := ""
	if ins.HostConfig != nil {
		netMode = string(ins.HostConfig.NetworkMode)
	}

	var created, startedAt, finishedAt time.Time
	var status string
	var running, paused, restarting, oomKilled, dead bool
	var pid, exitCode int
	var errStr string

	if ins.ContainerJSONBase != nil {
		created, _ = time.Parse(time.RFC3339Nano, ins.ContainerJSONBase.Created)
		if ins.ContainerJSONBase.State != nil {
			st := ins.ContainerJSONBase.State
			status = st.Status
			running = st.Running
			paused = st.Paused
			restarting = st.Restarting
			oomKilled = st.OOMKilled
			dead = st.Dead
			pid = st.Pid
			exitCode = st.ExitCode
			errStr = st.Error
			startedAt, _ = time.Parse(time.RFC3339Nano, st.StartedAt)
			finishedAt, _ = time.Parse(time.RFC3339Nano, st.FinishedAt)
		}
	}

	var image, imageID, command string
	var labels map[string]string
	var restartCount int

	if ins.Config != nil {
		image = ins.Config.Image
		command = strings.Join(ins.Config.Cmd, " ")
		labels = ins.Config.Labels
	}
	if ins.ContainerJSONBase != nil {
		imageID = ins.ContainerJSONBase.Image
		restartCount = ins.ContainerJSONBase.RestartCount
	}

	return Container{
		ID:      ins.ID,
		Name:    name,
		Image:   image,
		ImageID: imageID,
		Command: command,
		Created: created,
		State: State{
			Status:     status,
			Running:    running,
			Paused:     paused,
			Restarting: restarting,
			OOMKilled:  oomKilled,
			Dead:       dead,
			Pid:        pid,
			ExitCode:   exitCode,
			Error:      errStr,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
		},
		Status:       status,
		Ports:        ports,
		Labels:       labels,
		Mounts:       mounts,
		Network:      netMode,
		RestartCount: restartCount,
	}
}

func mapSummaryToContainer(sum types.Container) Container {
	name := ""
	if len(sum.Names) > 0 {
		name = strings.TrimPrefix(sum.Names[0], "/")
	}
	ports := make([]Port, 0, len(sum.Ports))
	for _, p := range sum.Ports {
		ports = append(ports, Port{
			IP:          p.IP,
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Type:        p.Type,
		})
	}
	mounts := make([]string, 0, len(sum.Mounts))
	for _, m := range sum.Mounts {
		mounts = append(mounts, fmt.Sprintf("%s:%s", m.Source, m.Destination))
	}

	return Container{
		ID:      sum.ID,
		Name:    name,
		Image:   sum.Image,
		ImageID: sum.ImageID,
		Command: sum.Command,
		Created: time.Unix(sum.Created, 0),
		State: State{
			Status:  sum.State,
			Running: sum.State == "running",
		},
		Status:       sum.Status,
		Ports:        ports,
		Labels:       sum.Labels,
		Mounts:       mounts,
		RestartCount: 0,
	}
}
