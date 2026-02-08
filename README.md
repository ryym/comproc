# Comproc

A docker-compose-like process manager for local development.
Run multiple processes in the background with a single command, and view their logs only when you need to.

## Motivation

During development you often need to run several processes at once — an API server, a frontend dev server, a database, a worker, etc.
Typical approaches have trade-offs:

- **Multiple terminal tabs**: manual, cluttered, easy to forget one
- **Foreground multiplexer (e.g. foreman)**: logs from all services flood a single terminal

Comproc takes the docker-compose approach: processes run in the background via a daemon, and you attach to their logs on demand.
Define your services in a YAML file, `comproc up`, and you're done.

## Quick Start

```bash
# Build
go build -o comproc ./cmd/comproc

# Define services in comproc.yaml
cat <<'EOF' > comproc.yaml
services:
  api:
    command: go run ./cmd/api
    depends_on:
      - db
  db:
    command: docker run -p 5432:5432 postgres
    restart: always
  frontend:
    command: npm run dev
    working_dir: ./frontend
    depends_on:
      - api
EOF

# Start all services in the background
./comproc up

# Check what's running
./comproc status

# Stream logs (Ctrl+C to detach — processes keep running)
./comproc logs -f

# Stop everything
./comproc down
```

## Commands

| Command                                 | Description                                        |
| --------------------------------------- | -------------------------------------------------- |
| `comproc up [service...]`               | Start services (launches daemon in the background) |
| `comproc up -f [service...]`            | Start services and follow logs                     |
| `comproc down`                          | Stop all services and shut down the daemon         |
| `comproc stop [service...]`             | Stop services without shutting down the daemon     |
| `comproc restart [service...]`          | Restart services                                   |
| `comproc logs [-f] [-n N] [service...]` | View logs                                          |
| `comproc status` / `ps`                 | Show service status                                |
| `comproc attach <service>`              | Attach to a service (forward stdin + stream logs)  |

When no services are specified, commands apply to all services.

## Configuration

Default config file: `comproc.yaml` (override with `-f path/to/file.yaml`).

```yaml
services:
  api:
    command: go run ./cmd/api # Required
    working_dir: ./backend # Optional (default: config file directory)
    env: # Optional
      PORT: "8080"
    restart: on-failure # Optional: never (default) | on-failure | always
    depends_on: # Optional
      - db
```

### Restart Policies

| Policy       | Behavior                      |
| ------------ | ----------------------------- |
| `never`      | Do not restart (default)      |
| `on-failure` | Restart only on non-zero exit |
| `always`     | Always restart                |

Restarts use exponential backoff (1s, 2s, 4s, ... up to 30s).

### Dependencies

Services listed in `depends_on` are started first.
When stopping a service, its dependents are stopped automatically.
Circular dependencies are detected and rejected at startup.

## How It Works

The first `comproc up` spawns a background daemon that manages all child processes.
Subsequent commands (`status`, `logs`, `stop`, ...) communicate with the daemon over a Unix socket using JSON-RPC.
Each config file gets its own socket, so multiple projects can run independently.

```
CLI ──── Unix Socket (JSON-RPC) ──── Daemon
                                       ├── Process A
                                       ├── Process B
                                       └── Process C
```

## Development

```bash
go build ./...
go test ./...
```

See [docs/](docs/) for detailed documentation.
