package e2e

import (
	"testing"
	"time"
)

// 3.1: Stops only the specified service; others remain running.
func TestStop_SpecificService(t *testing.T) {
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

	stdout, _, err := f.Run("stop", "app1")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"app1"}) {
		t.Errorf("expected app1 in stopped, got: %v", stopped)
	}

	status, err := f.GetServiceStatus("app2")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected app2 to still be running, got: %s", status.State)
	}
}

// 3.2: Daemon stays alive after stopping services (socket still exists).
func TestStop_DaemonStaysRunning(t *testing.T) {
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

	// Daemon should still be up (socket exists)
	if err := f.WaitForSocket(1 * time.Second); err != nil {
		t.Errorf("expected daemon to still be running after stop: %v", err)
	}
}

// 3.3: `stop` with no args stops all services; daemon stays alive.
func TestStop_AllServices(t *testing.T) {
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

	stdout, _, err := f.Run("stop")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"app1", "app2"}) {
		t.Errorf("expected all services in stopped list, got: %v", stopped)
	}

	// Daemon should still be up
	if err := f.WaitForSocket(1 * time.Second); err != nil {
		t.Errorf("expected daemon to still be running after stop: %v", err)
	}
}

// 3.4: Stopping db also stops api that depends on it.
func TestStop_StopsDependents(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  db:
    command: sleep 60
  api:
    command: sleep 60
    depends_on:
      - db
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	for _, svc := range []string{"db", "api"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Fatalf("WaitForState %s failed: %v", svc, err)
		}
	}

	stdout, _, err := f.Run("stop", "db")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"db", "api"}) {
		t.Errorf("expected both db and api to be stopped, got: %v", stopped)
	}
}

// 3.5: `stop svc1 svc2` stops multiple specified services.
func TestStop_MultipleServices(t *testing.T) {
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

	stdout, _, err := f.Run("stop", "app1", "app2")
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"app1", "app2"}) {
		t.Errorf("expected app1 and app2 in stopped list, got: %v", stopped)
	}

	status, err := f.GetServiceStatus("app3")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected app3 to still be running, got: %s", status.State)
	}
}

// 3.6: Stopping an already-stopped service succeeds with no error.
func TestStop_AlreadyStopped(t *testing.T) {
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
		t.Fatalf("first stop failed: %v", err)
	}

	err = f.WaitForState("app", "stopped", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState stopped failed: %v", err)
	}

	// Stop again - should succeed without error
	_, _, err = f.Run("stop", "app")
	if err != nil {
		t.Errorf("expected stop on already-stopped service to succeed, got: %v", err)
	}
}

// 3.7: Succeeds with no error when no daemon is running.
func TestStop_NoDaemon(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)

	_, _, err := f.Run("stop")
	if err != nil {
		t.Errorf("expected stop to succeed when no daemon, got error: %v", err)
	}
}
