# comproc

A docker-compose-like process manager CLI.

## Documentation

- [docs/architecture.md](docs/architecture.md) - Architecture overview
- [docs/config-spec.md](docs/config-spec.md) - Configuration file specification
- [docs/commands.md](docs/commands.md) - CLI command reference

## Quick Start

```bash
go build -o comproc ./cmd/comproc
./comproc -f examples/helloworld/comproc.yaml up
```

## Development

```bash
# Build
go build ./...

# Test
go test ./...
```

## Development Guidelines

- Write all code, comments, and documentation in English
- Make small, focused commits (one logical change per commit)
- Write unit tests for testable logic (config parsing, algorithms, protocol serialization, etc.)
- Ensure all tests pass before committing

## Package Structure

| Package             | Description                                 |
| ------------------- | ------------------------------------------- |
| `cmd/comproc`       | CLI entry point                             |
| `internal/cli`      | CLI commands, daemon communication          |
| `internal/daemon`   | Daemon, process supervision, log collection |
| `internal/config`   | Config file parsing and validation          |
| `internal/process`  | Child process start/stop                    |
| `internal/protocol` | JSON-RPC protocol definitions               |
