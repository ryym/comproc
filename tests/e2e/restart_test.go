package e2e

import (
	"strings"
	"testing"
	"time"
)

// 4.1: PID changes after restart; state returns to running.
func TestRestart_SingleService(t *testing.T) {
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

	status1, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	originalPID := status1.PID

	stdout, _, err := f.Run("restart", "app")
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}

	restarted := ParseRestartedServices(stdout)
	if !ContainsAll(restarted, []string{"app"}) {
		t.Errorf("expected 'app' in restarted services, got: %v", restarted)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState after restart failed: %v", err)
	}

	status2, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus after restart failed: %v", err)
	}
	if status2.PID == originalPID {
		t.Errorf("expected PID to change after restart, but still %d", originalPID)
	}
}

// 4.2: `restart` with no args restarts all services.
func TestRestart_AllServices(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app1:
    command: sleep 60
  app2:
    command: sleep 60
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	for _, svc := range []string{"app1", "app2"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Fatalf("WaitForState %s failed: %v", svc, err)
		}
	}

	pids := make(map[string]int)
	for _, svc := range []string{"app1", "app2"} {
		s, err := f.GetServiceStatus(svc)
		if err != nil {
			t.Fatalf("GetServiceStatus %s failed: %v", svc, err)
		}
		pids[svc] = s.PID
	}

	stdout, _, err := f.Run("restart")
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}

	restarted := ParseRestartedServices(stdout)
	if !ContainsAll(restarted, []string{"app1", "app2"}) {
		t.Errorf("expected all services in restarted list, got: %v", restarted)
	}

	for _, svc := range []string{"app1", "app2"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Errorf("WaitForState %s after restart failed: %v", svc, err)
		}
		s, err := f.GetServiceStatus(svc)
		if err != nil {
			t.Fatalf("GetServiceStatus %s after restart failed: %v", svc, err)
		}
		if s.PID == pids[svc] {
			t.Errorf("expected %s PID to change, but still %d", svc, pids[svc])
		}
	}
}

// 4.3: `restart svc1 svc2` restarts only specified services.
func TestRestart_MultipleSpecific(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app1:
    command: sleep 60
  app2:
    command: sleep 60
  app3:
    command: sleep 60
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	for _, svc := range []string{"app1", "app2", "app3"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Fatalf("WaitForState %s failed: %v", svc, err)
		}
	}

	app3Status, err := f.GetServiceStatus("app3")
	if err != nil {
		t.Fatalf("GetServiceStatus app3 failed: %v", err)
	}
	app3PID := app3Status.PID

	stdout, _, err := f.Run("restart", "app1", "app2")
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}

	restarted := ParseRestartedServices(stdout)
	if !ContainsAll(restarted, []string{"app1", "app2"}) {
		t.Errorf("expected app1 and app2 in restarted list, got: %v", restarted)
	}

	for _, svc := range []string{"app1", "app2"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Errorf("WaitForState %s after restart failed: %v", svc, err)
		}
	}

	// app3 should not have been restarted
	app3After, err := f.GetServiceStatus("app3")
	if err != nil {
		t.Fatalf("GetServiceStatus app3 after failed: %v", err)
	}
	if app3After.PID != app3PID {
		t.Errorf("expected app3 PID to remain %d, got %d", app3PID, app3After.PID)
	}
}

// 4.4: Restarting a stopped service starts it (equivalent to `up`).
func TestRestart_AlreadyStopped(t *testing.T) {
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

	_, _, err = f.Run("restart", "app")
	if err != nil {
		t.Fatalf("restart failed: %v", err)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Errorf("expected app to be running after restart: %v", err)
	}
}

// 4.5: Succeeds with no error when no daemon is running.
func TestRestart_NoDaemon(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)

	stdout, _, err := f.Run("restart")
	if err != nil {
		t.Errorf("expected restart to succeed when no daemon, got error: %v", err)
	}
	if !strings.Contains(stdout, "No services running") {
		t.Errorf("expected 'No services running', got: %s", stdout)
	}
}
