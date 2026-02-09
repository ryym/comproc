package e2e

import (
	"strings"
	"testing"
	"time"
)

// 5.1: Shows correct NAME, STATE=running, PID, RESTARTS for live service.
func TestStatus_RunningServices(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sleep 60
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}

	if status.Name != "app" {
		t.Errorf("expected NAME=app, got %s", status.Name)
	}
	if status.State != "running" {
		t.Errorf("expected STATE=running, got %s", status.State)
	}
	if status.PID == 0 {
		t.Errorf("expected non-zero PID, got 0")
	}
	if status.Restarts != 0 {
		t.Errorf("expected RESTARTS=0, got %d", status.Restarts)
	}
}

// 5.2: Stopped service shows STATE=stopped, PID="-".
func TestStatus_AfterStop(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sleep 60
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	_, _, err = f.Run("stop", "app")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	err = f.WaitForState("app", "stopped", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState stopped failed: %v", err)
	}

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}

	if status.State != "stopped" {
		t.Errorf("expected STATE=stopped, got %s", status.State)
	}
	if status.PID != 0 {
		t.Errorf("expected PID='-' (parsed as 0), got %d", status.PID)
	}
}

// 5.3: `ps` produces the same output as `status`.
func TestStatus_PsAlias(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sleep 60
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	statusOut, _, err := f.Run("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	psOut, _, err := f.Run("ps")
	if err != nil {
		t.Fatalf("ps failed: %v", err)
	}

	if statusOut != psOut {
		t.Errorf("expected ps and status to produce same output\nstatus:\n%s\nps:\n%s", statusOut, psOut)
	}
}

// 5.4: Without daemon but with config, all services shown as stopped.
func TestStatus_NoDaemonWithConfig(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sleep 60
  db:
    command: sleep 60
`)

	stdout, _, _ := f.Run("status")

	statuses := parseStatusOutput(stdout)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 services, got %d\noutput:\n%s", len(statuses), stdout)
	}

	for _, s := range statuses {
		if s.State != "stopped" {
			t.Errorf("expected %s to be stopped, got %s", s.Name, s.State)
		}
	}
}

// 5.5: Without daemon or config, prints "No services defined".
func TestStatus_NoDaemonNoConfig(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)

	stdout, _, _ := f.Run("status")

	if !strings.Contains(stdout, "No services defined") {
		t.Errorf("expected 'No services defined', got: %s", stdout)
	}
}

// 5.6: Process exits with 0 (restart:never) -> state=stopped.
func TestStatus_NormalExit(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: "true"
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

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}

	if status.State != "stopped" {
		t.Errorf("expected state=stopped after normal exit, got %s", status.State)
	}
}

// 5.7: Process exits with 1 (restart:never) -> state=failed.
func TestStatus_FailedExit(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: "false"
    restart: never
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "failed", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}

	if status.State != "failed" {
		t.Errorf("expected state=failed after non-zero exit, got %s", status.State)
	}
}
