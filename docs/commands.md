# CLI Commands

comproc provides a set of commands for managing services.

## Global Options

| Option         | Description                                   |
| -------------- | --------------------------------------------- |
| `-f`, `--file` | Path to config file (default: `comproc.yaml`) |

## Commands

### up

Start services.

```
comproc up [options] [service...]
```

**Options:**

| Option | Description                       |
| ------ | --------------------------------- |
| `-d`   | Run in detached mode (background) |

**Examples:**

```bash
# Start all services in foreground
comproc up

# Start specific services
comproc up api db

# Start in background
comproc up -d
```

When run without `-d`, comproc runs in the foreground and can be stopped with Ctrl+C.

### down

Stop all services and shut down.

```
comproc down
```

This command takes no arguments. It stops all running services and shuts down the background process.
If no background process is running, the command succeeds silently.

**Examples:**

```bash
# Stop everything and shut down
comproc down
```

### stop

Stop specific services without shutting down.

```
comproc stop [service...]
```

When stopping a service, its dependents are also stopped automatically.
The background process remains running so other services can continue.

**Examples:**

```bash
# Stop all services (background process stays)
comproc stop

# Stop specific services
comproc stop api
```

### status (or ps)

Show the status of all services.

```
comproc status
comproc ps
```

**Output columns:**

| Column   | Description             |
| -------- | ----------------------- |
| NAME     | Service name            |
| STATE    | Current state           |
| PID      | Process ID (if running) |
| RESTARTS | Number of restarts      |
| STARTED  | Start time (if running) |

**Example output:**

```
NAME      STATE    PID    RESTARTS  STARTED
api       running  12345  0         2024-01-15 10:30:00
db        running  12340  0         2024-01-15 10:29:55
frontend  stopped  -      0         -
```

### restart

Restart services.

```
comproc restart [service...]
```

**Examples:**

```bash
# Restart all services
comproc restart

# Restart specific services
comproc restart api
```

### logs

Show service logs.

```
comproc logs [options] [service...]
```

**Options:**

| Option     | Description                            |
| ---------- | -------------------------------------- |
| `-f`       | Follow log output                      |
| `-n <num>` | Number of lines to show (default: 100) |

**Examples:**

```bash
# Show recent logs for all services
comproc logs

# Show logs for specific services
comproc logs api db

# Follow logs
comproc logs -f

# Show last 50 lines and follow
comproc logs -n 50 -f api
```

**Output format:**

```
api | Server started on :8080
api | Received request GET /users
db  | Connection established
```

## Service States

| State    | Description                        |
| -------- | ---------------------------------- |
| stopped  | Service is not running             |
| starting | Service is being started           |
| running  | Service is running normally        |
| stopping | Service is being stopped           |
| failed   | Service crashed or failed to start |

## Exit Codes

| Code | Description                       |
| ---- | --------------------------------- |
| 0    | Success                           |
| 1    | Error (details printed to stderr) |
