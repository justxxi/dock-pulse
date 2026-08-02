# dock-pulse

Single binary container telemetry and supervision dashboard for Docker.

[![CI](https://github.com/owner/dock-pulse/actions/workflows/ci.yml/badge.svg)](https://github.com/owner/dock-pulse/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

![dock-pulse dashboard](docs/screenshot.png)
The screenshot above shows the live container grid, real-time log viewer, and command palette.

## Scope

dock-pulse connects to a local Docker daemon socket to observe container state, stream logs, measure resource usage, and automatically recover failing containers based on customizable policy.

### Non-goals

- dock-pulse is not Portainer or a container management suite.
- dock-pulse is not an orchestrator or Kubernetes replacement.
- dock-pulse is not a centralized alerting or notification system.
- dock-pulse is not a multi-host or cluster management solution.

## Quickstart

Run with Docker mounting the socket read-only:

```bash
docker run -d \
  --name dock-pulse \
  -p 127.0.0.1:8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/owner/dock-pulse:latest
```

Run using the pre-compiled binary:

```bash
./dock-pulse -listen-addr 127.0.0.1:8080 -token "your-secret-token"
```

## Configuration

| Flag | Environment Variable | Default | Description |
| --- | --- | --- | --- |
| `-listen-addr` | `DOCK_PULSE_LISTEN_ADDR` | `:8080` | Address and port to listen on |
| `-docker-host` | `DOCK_PULSE_DOCKER_HOST` | `""` | Custom Docker daemon socket path or URL |
| `-token` | `DOCK_PULSE_TOKEN` | `""` | Bearer token for API and WebSocket authentication |
| `-tls-cert` | `DOCK_PULSE_TLS_CERT` | `""` | Path to TLS certificate file |
| `-tls-key` | `DOCK_PULSE_TLS_KEY` | `""` | Path to TLS private key file |
| `-stats-interval` | `DOCK_PULSE_STATS_INTERVAL` | `2s` | Polling interval for stats reader stream |
| `-log-ring-size` | `DOCK_PULSE_LOG_RING_SIZE` | `1000` | Pre-allocated log buffer capacity per container |
| `-supervisor` | `DOCK_PULSE_SUPERVISOR` | `true` | Enable supervisor auto-restart mechanism |
| `-max-ws-connections` | `DOCK_PULSE_MAX_WS_CONNECTIONS` | `100` | Maximum allowed concurrent WebSocket clients |
| `-log-level` | `DOCK_PULSE_LOG_LEVEL` | `info` | Server log level (debug, info, warn, error) |
| `-base-path` | `DOCK_PULSE_BASE_PATH` | `/` | Base URL path prefix when behind subpath proxy |
| `-read-only` | `DOCK_PULSE_READ_ONLY` | `false` | Disable mutative actions (start, stop, restart) |
| `-allow-containers` | `DOCK_PULSE_ALLOW_CONTAINERS` | `""` | Comma-separated allowed container patterns |
| `-deny-containers` | `DOCK_PULSE_DENY_CONTAINERS` | `""` | Comma-separated denied container patterns |

## Security Model

Access to the Docker socket (`/var/run/docker.sock`) grants full root equivalent access to the host system.

- Operating dock-pulse on non-loopback network interfaces requires specifying an authentication token (`-token`).
- Production deployment should run bound to `127.0.0.1` behind a TLS reverse proxy or use `-tls-cert` and `-tls-key`.
- Read-only mode (`-read-only`) rejects mutative HTTP requests at the router level with status 403.
- Container allow and deny patterns filter containers across both telemetry streaming and action execution.
- Strict container ID format validation is enforced before issuing Docker Engine API calls.
- Environment variables, raw tokens, and secret definitions inside container inspect payloads are excluded from public API responses.

## Auto-restart Supervisor

The supervisor monitors container exit events with non-zero exit codes.

- Enabled globally via `-supervisor=true` and overridable per container via label `dock-pulse.autorestart=off`.
- Maximum attempts can be overridden per container via label `dock-pulse.autorestart.max=5`.
- Containers using native Docker restart policies (`always`, `unless-stopped`) are ignored by the supervisor.
- Backoff formula: `delay = min(max_interval, base_interval * 2^(attempt-1)) * random(0, 1)`.
- If a container remains healthy longer than 30 seconds, the restart attempt counter is reset to zero.
- When attempts are exhausted, the container is marked as `restart_exhausted`, a event is broadcast, and further automatic restarts stop until user action.
- Manual container stop from the UI marks the container as intentionally stopped and suppresses supervisor action.

## Architecture

```
[Docker Daemon Engine API]
        |
        | (Unix Domain Socket / HTTP)
        v
 [Events Watcher] -> [Registry In-Memory State]
        |                    |
 [Stats Collector]           v
 [Logs Streamer]  ->  [WebSocket Fan-Out Hub]
                             |
                             | (JSON WebSocket Protocol)
                             v
                  [Browser SPA Frontend]
```

All state is maintained in-memory and synchronized reactively from the Docker Engine API event stream.

Slow WebSocket clients that fail to consume messages are handled deterministically: intermediate metrics updates are dropped first. If the client buffer remains saturated, the connection is closed with policy violation status.

The frontend is constructed using native DOM API methods and strict TypeScript without external frameworks to ensure minimal memory footprint, zero runtime framework overhead, and sub-millisecond DOM update performance.

## WebSocket Protocol

Clients communicate with `/api/stream` using a JSON envelope.

```json
{
  "type": "container.updated",
  "seq": 42,
  "data": {
    "container": {
      "id": "a1b2c3d4e5f6",
      "name": "web-server",
      "state": { "running": true, "status": "running" }
    }
  }
}
```

| Type | Direction | Description |
| --- | --- | --- |
| `snapshot` | Server -> Client | Full initial state of containers and sequence |
| `container.updated` | Server -> Client | Incremental container state modification |
| `container.removed` | Server -> Client | Container deletion event |
| `stats` | Server -> Client | Real-time CPU, memory, and network metrics point |
| `log` | Server -> Client | Streamed log line entry |
| `supervisor` | Server -> Client | Supervisor action or exhaustion notification |
| `subscribe.logs` | Client -> Server | Subscribe to log stream for container ID |
| `unsubscribe.logs` | Client -> Server | Unsubscribe from log stream for container ID |

## Development

Requirements:
- Go 1.23+
- Node.js 20+

Start development server:

```bash
make dev
```

Run test suites and linters:

```bash
make test
make lint
```

Repository structure:

```
dock-pulse/
  cmd/dock-pulse/
  internal/
    auth/
    config/
    dockerx/
    events/
    httpapi/
    hub/
    logs/
    protocol/
    registry/
    stats/
    supervisor/
    version/
    web/
  web/
    src/
      api/
      components/
      lib/
      state/
      styles/
      views/
  deploy/
```

## Performance

Measured benchmarks under test conditions:
- Idle memory consumption on 50 monitored containers: < 35 MB RSS.
- CPU consumption during steady state telemetry: < 1.5% single core.
- Latency from Docker event emission to UI update: < 120 ms over local network.
- Gzipped frontend bundle size: < 42 KB.
- Log streamer ring buffer allocations in stationary mode: 0 B per line.

## License

MIT License. See [LICENSE](LICENSE) for details.
