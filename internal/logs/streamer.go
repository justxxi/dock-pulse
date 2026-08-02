package logs

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/justxxi/dock-pulse/internal/dockerx"
)

type Streamer struct {
	mu           sync.RWMutex
	engine       dockerx.Engine
	logger       *slog.Logger
	ringCapacity int
	buffers      map[string]*RingBuffer
	subscribers  map[string]int
	cancels      map[string]context.CancelFunc
	onLine       func(containerID string, line LogLine)
}

func NewStreamer(engine dockerx.Engine, logger *slog.Logger, ringCapacity int, onLine func(containerID string, line LogLine)) *Streamer {
	if ringCapacity <= 0 {
		ringCapacity = 1000
	}
	return &Streamer{
		engine:       engine,
		logger:       logger,
		ringCapacity: ringCapacity,
		buffers:      make(map[string]*RingBuffer),
		subscribers:  make(map[string]int),
		cancels:      make(map[string]context.CancelFunc),
		onLine:       onLine,
	}
}

func (s *Streamer) GetRingBuffer(containerID string) *RingBuffer {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, ok := s.buffers[containerID]
	if !ok {
		buf = NewRingBuffer(s.ringCapacity)
		s.buffers[containerID] = buf
	}
	return buf
}

func (s *Streamer) Subscribe(ctx context.Context, containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.buffers[containerID]; !ok {
		s.buffers[containerID] = NewRingBuffer(s.ringCapacity)
	}

	s.subscribers[containerID]++
	if s.subscribers[containerID] == 1 {
		sCtx, cancel := context.WithCancel(context.Background())
		s.cancels[containerID] = cancel
		go s.streamLogs(sCtx, containerID)
	}
}

func (s *Streamer) Unsubscribe(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count, ok := s.subscribers[containerID]; ok && count > 0 {
		s.subscribers[containerID]--
		if s.subscribers[containerID] == 0 {
			go func(id string) {
				time.Sleep(2 * time.Second)
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.subscribers[id] == 0 {
					if cancel, ok := s.cancels[id]; ok {
						cancel()
						delete(s.cancels, id)
					}
				}
			}(containerID)
		}
	}
}

func (s *Streamer) streamLogs(ctx context.Context, containerID string) {
	reader, err := s.engine.Logs(ctx, containerID, dockerx.LogOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     true,
		Tail:       "200",
	})
	if err != nil {
		s.logger.Error("failed to start log stream", "container_id", containerID, "error", err)
		return
	}
	defer reader.Close()

	bufReader := bufio.NewReader(reader)
	header := make([]byte, 8)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err := io.ReadFull(bufReader, header)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				s.logger.Debug("log stream ended", "container_id", containerID, "error", err)
			}
			return
		}

		streamType := "stdout"
		switch header[0] {
		case 1:
			streamType = "stdout"
		case 2:
			streamType = "stderr"
		}

		count := binary.BigEndian.Uint32(header[4:8])
		payload := make([]byte, count)
		_, err = io.ReadFull(bufReader, payload)
		if err != nil {
			return
		}

		lines := strings.Split(string(payload), "\n")
		for _, lineText := range lines {
			if lineText == "" {
				continue
			}

			ts := time.Now()
			text := lineText

			parts := strings.SplitN(lineText, " ", 2)
			if len(parts) == 2 {
				if parsedTs, parseErr := time.Parse(time.RFC3339Nano, parts[0]); parseErr == nil {
					ts = parsedTs
					text = parts[1]
				}
			}

			ring := s.GetRingBuffer(containerID)
			line := ring.Push(ts, streamType, text)

			if s.onLine != nil {
				s.onLine(containerID, line)
			}
		}
	}
}
