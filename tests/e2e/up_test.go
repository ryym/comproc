package e2e

import (
	"testing"
	"time"
)

// 1.1: Start a single service; verify state=running and PID is assigned.
func TestUp_SingleService(t *testing.T) {
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
	if status.PID == 0 {
		t.Errorf("expected non-zero PID, got 0")
	}
}

// 1.2: Start multiple services at once; all become running.
func TestUp_MultipleServices(t *testing.T) {
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
			t.Errorf("WaitForState %s failed: %v", svc, err)
		}
	}

	statuses, err := f.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if len(statuses) != 3 {
		t.Errorf("expected 3 services, got %d", len(statuses))
	}
}

// 1.3: `up svc1 svc2` starts only specified services; others remain stopped.
func TestUp_SpecificServices(t *testing.T) {
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
	_, stderr, err := f.Run("up", "app1", "app2")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	for _, svc := range []string{"app1", "app2"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Errorf("WaitForState %s failed: %v", svc, err)
		}
	}

	status, err := f.GetServiceStatus("app3")
	if err != nil {
		t.Fatalf("GetServiceStatus app3 failed: %v", err)
	}
	if status.State != "stopped" {
		t.Errorf("expected app3 to be stopped, got %s", status.State)
	}
}

// 1.4: `up api` auto-starts its dependency (db) as well.
func TestUp_SpecificServiceWithDeps(t *testing.T) {
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
	_, stderr, err := f.Run("up", "api")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	for _, svc := range []string{"db", "api"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Errorf("WaitForState %s failed: %v", svc, err)
		}
	}
}

// 1.5: Running `up` again while daemon is active does not disrupt existing services.
func TestUp_AlreadyRunning(t *testing.T) {
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

	// Run up again while service is already running
	_, stderr, err = f.Run("up")
	if err != nil {
		t.Fatalf("second up failed: %v\n%s", err, stderr)
	}

	status2, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus after re-up failed: %v", err)
	}
	if status2.State != "running" {
		t.Errorf("expected running, got %s", status2.State)
	}
	if status2.PID != originalPID {
		t.Errorf("expected PID to remain %d, got %d", originalPID, status2.PID)
	}
}

// 1.6: After `stop svc`, `up svc` restarts it.
func TestUp_StartStoppedService(t *testing.T) {
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
		t.Fatalf("WaitForState running failed: %v", err)
	}

	_, stderr, err = f.Run("stop", "app")
	if err != nil {
		t.Fatalf("stop failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "stopped", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState stopped failed: %v", err)
	}

	_, stderr, err = f.Run("up", "app")
	if err != nil {
		t.Fatalf("up app failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Errorf("expected app to be running after re-up: %v", err)
	}
}

// 1.7: `up -f` streams logs; Ctrl-C disconnects but daemon keeps running.
func TestUp_FollowLogs(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo "hello from app"; sleep 60'
`)
	cmd, outBuf, err := f.RunAsync("up", "-f")
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	if err := f.WaitForSocket(10 * time.Second); err != nil {
		t.Fatalf("WaitForSocket failed: %v", err)
	}

	if err := WaitForContent(outBuf, "hello from app", 5*time.Second); err != nil {
		t.Errorf("expected log output to contain 'hello from app': %v", err)
	}

	// Ctrl-C on `up -f` should NOT stop the daemon
	InterruptAndWait(cmd)

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Errorf("expected daemon to still be running after Ctrl-C: %v", err)
	}
}

// 1.8: `up -f svc1` starts only svc1 and follows its logs.
func TestUp_FollowLogsSpecificServices(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app1:
    command: sh -c 'echo "from app1"; sleep 60'
  app2:
    command: sh -c 'echo "from app2"; sleep 60'
`)
	cmd, outBuf, err := f.RunAsync("up", "-f", "app1")
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	defer InterruptAndWait(cmd)

	if err := f.WaitForSocket(10 * time.Second); err != nil {
		t.Fatalf("WaitForSocket failed: %v", err)
	}

	err = f.WaitForState("app1", "running", 5*time.Second)
	if err != nil {
		t.Errorf("expected app1 to be running: %v", err)
	}

	status, err := f.GetServiceStatus("app2")
	if err != nil {
		t.Fatalf("GetServiceStatus app2 failed: %v", err)
	}
	if status.State != "stopped" {
		t.Errorf("expected app2 to be stopped, got %s", status.State)
	}

	if err := WaitForContent(outBuf, "from app1", 5*time.Second); err != nil {
		t.Errorf("expected log output to contain 'from app1': %v", err)
	}
}

// 1.9: While daemon runs, `up newSvc` starts only the not-yet-running service.
func TestUp_StartsOnlyNewServices(t *testing.T) {
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
	_, stderr, err := f.Run("up", "app1")
	if err != nil {
		t.Fatalf("up app1 failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app1", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState app1 failed: %v", err)
	}

	status1, err := f.GetServiceStatus("app1")
	if err != nil {
		t.Fatalf("GetServiceStatus app1 failed: %v", err)
	}
	app1PID := status1.PID

	// Start app2 while app1 is already running
	_, stderr, err = f.Run("up", "app2")
	if err != nil {
		t.Fatalf("up app2 failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app2", "running", 5*time.Second)
	if err != nil {
		t.Errorf("WaitForState app2 failed: %v", err)
	}

	// app1 should not have been restarted
	status1After, err := f.GetServiceStatus("app1")
	if err != nil {
		t.Fatalf("GetServiceStatus app1 after failed: %v", err)
	}
	if status1After.PID != app1PID {
		t.Errorf("expected app1 PID to remain %d, got %d", app1PID, status1After.PID)
	}
}
