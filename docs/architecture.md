# Architecture

comproc is a docker-compose-like CLI tool for managing multiple processes with a daemon architecture.

## Overview

```
┌─────────────────────────────────────────────────────────┐
│                      CLI Client                         │
│  (up, down, logs, status, restart)                      │
└─────────────────────┬───────────────────────────────────┘
                      │ Unix Socket (JSON-RPC)
┌─────────────────────▼───────────────────────────────────┐
│                      Daemon                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │ Process     │  │ Log         │  │ Config      │     │
│  │ Supervisor  │  │ Collector   │  │ Manager     │     │
│  └──────┬──────┘  └──────┬──────┘  └─────────────┘     │
└─────────┼────────────────┼──────────────────────────────┘
          │                │
    ┌─────▼─────┐    ┌─────▼─────┐
    │ Process A │    │ Log Files │
    │ Process B │    │ (per proc)│
    │ Process C │    └───────────┘
    └───────────┘
```

## Components

### CLI Client

The CLI client provides user-facing commands:

- `comproc up [-d] [service...]` - Start services
- `comproc down [service...]` - Stop services
- `comproc logs [-f] [service...]` - View logs
- `comproc status` - Show service status
- `comproc restart [service...]` - Restart services

### Daemon

The daemon is responsible for:

- Starting, stopping, and monitoring child processes
- Controlling startup order based on dependencies
- Detecting crashes and applying restart policies
- Collecting and buffering logs
- Processing requests from the CLI

### Communication

CLI and daemon communicate via Unix socket using JSON-RPC 2.0 protocol.

Socket path: `~/.comproc/comproc.sock` or `/tmp/comproc-{uid}/comproc.sock`

## Package Structure

```
comproc/
├── cmd/comproc/       # Entry point
├── internal/
│   ├── cli/           # CLI command implementations
│   ├── daemon/        # Daemon implementation
│   ├── config/        # Configuration file parsing
│   ├── process/       # Process management
│   └── protocol/      # Communication protocol definitions
└── docs/              # Documentation
```

## Process States

Each process can be in one of these states:

- `stopped` - Not running
- `starting` - Being started
- `running` - Running normally
- `stopping` - Being stopped
- `failed` - Crashed or failed to start

## Restart Policies

| Policy       | Behavior                               |
| ------------ | -------------------------------------- |
| `always`     | Always restart regardless of exit code |
| `on-failure` | Restart only if exit code is non-zero  |
| `never`      | Never restart                          |

Restarts use exponential backoff: 1s, 2s, 4s, ... up to 30s max.

## Dependency Resolution

1. Build dependency graph from configuration
2. Determine startup order via topological sort
3. Detect and report circular dependencies as errors
4. Start dependent services only after dependencies are `running`
