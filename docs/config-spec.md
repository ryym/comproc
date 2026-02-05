# Configuration File Specification

comproc uses a YAML configuration file (default: `comproc.yaml`) to define services.

## File Structure

```yaml
services:
  <service-name>:
    command: <command>
    working_dir: <directory>
    env:
      <KEY>: <value>
    restart: <policy>
    depends_on:
      - <service-name>
```

## Fields

### services (required)

A map of service definitions. Each key is the service name used in CLI commands.

### command (required)

The command to run. Can be a simple command or a shell command.

Examples:

```yaml
command: go run ./cmd/api
command: npm run dev
command: docker run -p 5432:5432 postgres
```

### working_dir (optional)

The working directory for the command. Relative paths are resolved from the configuration file location.

Default: The directory containing the configuration file.

### env (optional)

Environment variables to set for the process.

Example:

```yaml
env:
  PORT: "8080"
  DEBUG: "true"
```

### restart (optional)

The restart policy for the service.

| Value        | Description                                     |
| ------------ | ----------------------------------------------- |
| `never`      | Never restart (default)                         |
| `on-failure` | Restart only if the process exits with non-zero |
| `always`     | Always restart regardless of exit code          |

Restarts use exponential backoff: 1s, 2s, 4s, ... up to 30s maximum.

### depends_on (optional)

List of service names that must be running before this service starts.

Example:

```yaml
services:
  api:
    command: go run ./cmd/api
    depends_on:
      - db
  db:
    command: docker run postgres
```

In this example, `db` will start first, and `api` will only start after `db` is running.

## Validation Rules

1. At least one service must be defined
2. Each service must have a `command`
3. `restart` must be one of: `never`, `on-failure`, `always`
4. All services in `depends_on` must exist
5. Circular dependencies are not allowed

## Example Configuration

```yaml
services:
  api:
    command: go run ./cmd/api
    working_dir: ./backend
    env:
      PORT: "8080"
      DATABASE_URL: "postgres://localhost:5432/mydb"
    restart: on-failure
    depends_on:
      - db

  db:
    command: docker run -p 5432:5432 postgres
    restart: always

  frontend:
    command: npm run dev
    working_dir: ./frontend
    env:
      VITE_API_URL: "http://localhost:8080"
    depends_on:
      - api
```
