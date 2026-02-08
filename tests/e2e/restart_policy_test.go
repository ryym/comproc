package e2e

import (
	"testing"
	"time"
)

// 7.1: Process exits with 0; not restarted, restarts=0.
func TestRestartPolicy_Never(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo done; exit 0'
    restart: never
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "stopped", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState stopped failed: %v", err)
	}

	// Give some time to verify it doesn't restart
	time.Sleep(500 * time.Millisecond)

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	if status.Restarts != 0 {
		t.Errorf("expected 0 restarts, got %d", status.Restarts)
	}
}

// 7.2: Process exits with 1; restarted (restarts >= 1).
func TestRestartPolicy_OnFailure_NonZeroExit(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo failing; exit 1'
    restart: on-failure
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := f.GetServiceStatus("app")
		if err == nil && status.Restarts >= 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	status, _ := f.GetServiceStatus("app")
	t.Errorf("expected at least 1 restart, got %d", status.Restarts)
}

// 7.3: Process exits with 0; not restarted under on-failure policy.
func TestRestartPolicy_OnFailure_ZeroExit(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo ok; exit 0'
    restart: on-failure
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "stopped", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState stopped failed: %v", err)
	}

	// Give time to confirm no restart
	time.Sleep(500 * time.Millisecond)

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	if status.Restarts != 0 {
		t.Errorf("expected 0 restarts under on-failure with exit 0, got %d", status.Restarts)
	}
}

// 7.4: Process exits with 0; still restarted under always policy.
func TestRestartPolicy_Always(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo exiting; exit 0'
    restart: always
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := f.GetServiceStatus("app")
		if err == nil && status.Restarts >= 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	status, _ := f.GetServiceStatus("app")
	t.Errorf("expected at least 1 restart with 'always' policy, got %d", status.Restarts)
}

// 7.5: Restarts counter increases with each restart.
func TestRestartPolicy_CounterIncrements(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'exit 1'
    restart: on-failure
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	// Wait for restarts counter to reach at least 2
	deadline := time.Now().Add(10 * time.Second)
	var prevRestarts int
	seen := false
	for time.Now().Before(deadline) {
		status, err := f.GetServiceStatus("app")
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if status.Restarts >= 1 && !seen {
			prevRestarts = status.Restarts
			seen = true
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if seen && status.Restarts > prevRestarts {
			return // Counter is incrementing
		}
		time.Sleep(100 * time.Millisecond)
	}

	status, _ := f.GetServiceStatus("app")
	t.Errorf("expected restarts counter to increment beyond %d, got %d", prevRestarts, status.Restarts)
}
