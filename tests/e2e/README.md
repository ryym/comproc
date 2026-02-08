# E2E Tests

End-to-end tests for Comproc. These tests verify the application's behavior from the user's perspective by running the actual CLI binary as a subprocess.

## Goals

- Ensure all commands work correctly with their options, especially process start/stop behavior.
- Catch regressions when refactoring internal implementation.
- Serve as a living specification of Comproc's behavior.

## Design Principles

### Black-box testing

Tests must not depend on internal implementation details. They interact with Comproc only through its CLI interface: running commands, checking stdout/stderr, and observing side effects (e.g., process state via `status`). This ensures the tests remain valid even when internals are refactored.

### Test case inventory

[TEST_CASES.md](TEST_CASES.md) maintains a comprehensive list of all e2e test cases. This file must always be kept in sync with the actual test code:

- When adding a new test, add it to TEST_CASES.md first.
- When removing a test, remove it from TEST_CASES.md as well.
- Test function names in code must match those listed in TEST_CASES.md.

### Scope

The primary focus is on **command behavior and process lifecycle**:

- Commands work correctly with all their options
- Processes start, stop, and restart as expected
- Dependency resolution is correct
- Restart policies are honored

Lower-priority concerns (not currently covered):

- Config file validation errors
- Exact output formatting / column alignment
- ANSI color output

## How It Works

1. `setup_test.go` builds the Comproc binary once before all tests via `TestMain`.
2. Each test creates an isolated `Fixture` with its own temp directory and Unix socket (via `COMPROC_SOCKET` env var), so tests do not interfere with each other.
3. Tests write a YAML config, start the daemon, run commands, and assert on the results.

## File Organization

| File              | Contents                                             |
| ----------------- | ---------------------------------------------------- |
| `setup_test.go`   | Binary build in `TestMain`                           |
| `helpers_test.go` | `Fixture`, helpers (`StartDaemon`, `Up`, etc.)       |
| `up_test.go`      | Tests for `up` command                               |
| `down_test.go`    | Tests for `down` command                             |
| `stop_test.go`    | Tests for `stop` command                             |
| `restart_test.go` | Tests for `restart` command                          |
| `status_test.go`  | Tests for `status` / `ps` command                    |
| `logs_test.go`    | Tests for `logs` command                             |
| `TEST_CASES.md`   | Authoritative list of all test cases                 |
| `e2e_test.go`     | Legacy tests (to be migrated and eventually removed) |

## Running Tests

```bash
# Run all e2e tests
go test ./tests/e2e/ -timeout 120s

# Run tests for a specific command
go test ./tests/e2e/ -run '^TestUp_' -timeout 120s

# Skip e2e tests in short mode
go test ./tests/e2e/ -short
```
