package e2e

import (
	"os"
	"strings"
	"testing"
	"time"
)

func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
}

// --- Basic Lifecycle Tests ---

func TestUp_Detached(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Verify service is running
	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Errorf("WaitForState failed: %v", err)
	}

	status, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	if status.PID == 0 {
		t.Errorf("expected non-zero PID, got 0")
	}
}

func TestUp_Foreground(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sh -c 'echo "hello from app"; sleep 60'
`
	cmd, outBuf, err := f.StartDaemonForeground(config)
	if err != nil {
		t.Fatalf("StartDaemonForeground failed: %v", err)
	}

	// Wait a bit for log output
	time.Sleep(500 * time.Millisecond)

	output := outBuf.String()
	if !strings.Contains(output, "Started:") {
		t.Errorf("expected 'Started:' in output, got: %s", output)
	}

	// Stop daemon
	cmd.Process.Signal(os.Interrupt)
	cmd.Wait()
}

func TestDown(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for service to start
	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	// Stop all services and shut down daemon
	output, err := f.Down()
	if err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	stopped := ParseStoppedServices(output)
	if !ContainsAll(stopped, []string{"app"}) {
		t.Errorf("expected 'app' in stopped services, got: %v", stopped)
	}

	// Verify daemon shut down (socket gone)
	err = f.WaitForSocketGone(5 * time.Second)
	if err != nil {
		t.Errorf("expected socket to be removed after down: %v", err)
	}
}

// --- Dependency Tests ---

func TestDependencies_StartOrder(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  db:
    command: sh -c 'echo "[db] started"; sleep 60'
  api:
    command: sh -c 'echo "[api] started"; sleep 60'
    depends_on:
      - db
  frontend:
    command: sh -c 'echo "[frontend] started"; sleep 60'
    depends_on:
      - api
`
	cmd, outBuf, err := f.StartDaemonForeground(config)
	if err != nil {
		t.Fatalf("StartDaemonForeground failed: %v", err)
	}

	// Wait for all services to start
	err = f.WaitForState("frontend", "running", 10*time.Second)
	if err != nil {
		t.Fatalf("WaitForState frontend failed: %v", err)
	}

	output := outBuf.String()

	// Check that all services started
	if !strings.Contains(output, "[db] started") {
		t.Errorf("expected db to start")
	}
	if !strings.Contains(output, "[api] started") {
		t.Errorf("expected api to start")
	}
	if !strings.Contains(output, "[frontend] started") {
		t.Errorf("expected frontend to start")
	}

	// Clean up
	cmd.Process.Signal(os.Interrupt)
	cmd.Wait()
}

func TestDependencies_StopOrder(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  db:
    command: sleep 60
  api:
    command: sleep 60
    depends_on:
      - db
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for services to start
	err = f.WaitForState("api", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState api failed: %v", err)
	}

	// Stop db - api should also stop since it depends on db
	stdout, _, _ := f.Run("stop", "db")

	// api should be stopped because it depends on db
	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"db", "api"}) {
		t.Errorf("expected both db and api to be stopped, got: %v", stopped)
	}
}

// --- Restart Policy Tests ---

func TestRestartPolicy_Never(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sh -c 'echo done; exit 0'
    restart: never
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for process to exit
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

func TestRestartPolicy_OnFailure(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sh -c 'echo failing; exit 1'
    restart: on-failure
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for at least one restart
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := f.GetServiceStatus("app")
		if err == nil && status.Restarts >= 1 {
			return // Success
		}
		time.Sleep(500 * time.Millisecond)
	}

	status, _ := f.GetServiceStatus("app")
	t.Errorf("expected at least 1 restart, got %d", status.Restarts)
}

func TestRestartPolicy_Always(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sh -c 'echo exiting; exit 0'
    restart: always
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for at least one restart (exit 0 should still trigger restart with always)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := f.GetServiceStatus("app")
		if err == nil && status.Restarts >= 1 {
			return // Success
		}
		time.Sleep(500 * time.Millisecond)
	}

	status, _ := f.GetServiceStatus("app")
	t.Errorf("expected at least 1 restart with 'always' policy, got %d", status.Restarts)
}

// --- Status Command Tests ---

func TestStatus_Format(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	stdout, _, err := f.Run("status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	// Check header
	if !strings.Contains(stdout, "NAME") ||
		!strings.Contains(stdout, "STATE") ||
		!strings.Contains(stdout, "PID") ||
		!strings.Contains(stdout, "RESTARTS") ||
		!strings.Contains(stdout, "STARTED") {
		t.Errorf("expected status header columns, got:\n%s", stdout)
	}

	// Check service row
	if !strings.Contains(stdout, "app") ||
		!strings.Contains(stdout, "running") {
		t.Errorf("expected app service info in output, got:\n%s", stdout)
	}
}

func TestStatus_NoDaemon(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)

	// No daemon running, no config file
	stdout, _, _ := f.Run("status")

	if !strings.Contains(stdout, "No services defined") {
		t.Errorf("expected 'No services defined', got: %s", stdout)
	}
}

func TestStatus_NoDaemon_WithConfig(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sleep 60
  db:
    command: sleep 60
`
	f.WriteConfig(config)

	// No daemon running, but config exists
	stdout, _, _ := f.Run("-f", f.ConfigPath, "status")

	if !strings.Contains(stdout, "app") || !strings.Contains(stdout, "stopped") {
		t.Errorf("expected services shown as stopped, got: %s", stdout)
	}
	if !strings.Contains(stdout, "db") {
		t.Errorf("expected db service in output, got: %s", stdout)
	}
	if strings.Contains(stdout, "Daemon") || strings.Contains(stdout, "daemon") {
		t.Errorf("should not contain the word 'daemon', got: %s", stdout)
	}
}

// --- Logs Command Tests ---

func TestLogs_RecentLines(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sh -c 'echo "line1"; echo "line2"; echo "line3"; sleep 60'
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for logs
	time.Sleep(1 * time.Second)

	logs, err := f.Logs(10)
	if err != nil {
		t.Fatalf("Logs failed: %v", err)
	}

	if !strings.Contains(logs, "line1") ||
		!strings.Contains(logs, "line2") ||
		!strings.Contains(logs, "line3") {
		t.Errorf("expected log lines, got:\n%s", logs)
	}
}

func TestLogs_ServiceFilter(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app1:
    command: sh -c 'echo "from app1"; sleep 60'
  app2:
    command: sh -c 'echo "from app2"; sleep 60'
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for logs
	time.Sleep(1 * time.Second)

	// Get only app1 logs
	logs, err := f.Logs(10, "app1")
	if err != nil {
		t.Fatalf("Logs failed: %v", err)
	}

	if !strings.Contains(logs, "from app1") {
		t.Errorf("expected app1 log, got:\n%s", logs)
	}

	// When filtering, we should only see app1's logs
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "app1 |") {
			t.Errorf("expected only app1 logs, but got line: %s", line)
		}
	}
}

// --- Restart Command Tests ---

func TestRestart_SingleService(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	// Get original PID
	status1, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	originalPID := status1.PID

	// Restart
	output, err := f.Restart("app")
	if err != nil {
		t.Fatalf("Restart failed: %v", err)
	}

	restarted := ParseRestartedServices(output)
	if !ContainsAll(restarted, []string{"app"}) {
		t.Errorf("expected 'app' in restarted services, got: %v", restarted)
	}

	// Wait for new process
	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState after restart failed: %v", err)
	}

	// Verify PID changed
	status2, err := f.GetServiceStatus("app")
	if err != nil {
		t.Fatalf("GetServiceStatus after restart failed: %v", err)
	}
	if status2.PID == originalPID {
		t.Errorf("expected PID to change after restart, but still %d", originalPID)
	}
}

// --- Error Handling Tests ---

func TestUp_InvalidConfig(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	// Missing command
	config := `
services:
  app:
    restart: always
`
	f.WriteConfig(config)

	// Use foreground mode to catch config errors immediately
	_, stderr, err := f.RunWithTimeout(5*time.Second, "-f", f.ConfigPath, "up")

	if err == nil {
		t.Errorf("expected error for missing command")
	}
	if !strings.Contains(stderr, "command is required") {
		t.Errorf("expected 'command is required' error, got: %s", stderr)
	}
}

func TestUp_CircularDependency(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  a:
    command: sleep 60
    depends_on:
      - b
  b:
    command: sleep 60
    depends_on:
      - a
`
	f.WriteConfig(config)

	// Use foreground mode to catch config errors immediately
	_, stderr, err := f.RunWithTimeout(5*time.Second, "-f", f.ConfigPath, "up")

	if err == nil {
		t.Errorf("expected error for circular dependency")
	}
	if !strings.Contains(stderr, "circular dependency") {
		t.Errorf("expected 'circular dependency' error, got: %s", stderr)
	}
}

// --- Multiple Services Tests ---

func TestUp_MultipleServices(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app1:
    command: sleep 60
  app2:
    command: sleep 60
  app3:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for all services
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

func TestDown_AllServices(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app1:
    command: sleep 60
  app2:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	// Wait for services
	for _, svc := range []string{"app1", "app2"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Fatalf("WaitForState %s failed: %v", svc, err)
		}
	}

	// Stop all and shut down daemon
	output, err := f.Down()
	if err != nil {
		t.Fatalf("Down failed: %v", err)
	}

	stopped := ParseStoppedServices(output)
	if !ContainsAll(stopped, []string{"app1", "app2"}) {
		t.Errorf("expected all services stopped, got: %v", stopped)
	}

	// Verify daemon shut down
	err = f.WaitForSocketGone(5 * time.Second)
	if err != nil {
		t.Errorf("expected socket to be removed after down: %v", err)
	}
}

func TestStop_SpecificService(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)
	config := `
services:
  app1:
    command: sleep 60
  app2:
    command: sleep 60
`
	err := f.StartDaemon(config)
	if err != nil {
		t.Fatalf("StartDaemon failed: %v", err)
	}

	for _, svc := range []string{"app1", "app2"} {
		err = f.WaitForState(svc, "running", 5*time.Second)
		if err != nil {
			t.Fatalf("WaitForState %s failed: %v", svc, err)
		}
	}

	// Stop only app1
	output, err := f.Stop("app1")
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stopped := ParseStoppedServices(output)
	if !ContainsAll(stopped, []string{"app1"}) {
		t.Errorf("expected app1 in stopped, got: %v", stopped)
	}

	// app2 should still be running
	status, err := f.GetServiceStatus("app2")
	if err != nil {
		t.Fatalf("GetServiceStatus failed: %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected app2 to still be running, got: %s", status.State)
	}

	// Daemon should still be up (socket exists)
	if err := f.WaitForSocket(1 * time.Second); err != nil {
		t.Errorf("expected daemon to still be running after stop")
	}
}

func TestDown_NoDaemon(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)

	// down with no daemon should succeed silently
	stdout, _, err := f.Run("down")
	if err != nil {
		t.Errorf("expected down to succeed when no daemon, got error: %v", err)
	}
	// Should produce no output (or empty)
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got: %s", stdout)
	}
}

func TestRestart_NoDaemon(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)

	stdout, _, err := f.Run("restart")
	if err != nil {
		t.Errorf("expected restart to succeed when no daemon, got error: %v", err)
	}
	if !strings.Contains(stdout, "No services running") {
		t.Errorf("expected 'No services running', got: %s", stdout)
	}
}

func TestLogs_NoDaemon(t *testing.T) {
	skipIfShort(t)

	f := NewFixture(t)

	stdout, _, err := f.Run("logs")
	if err != nil {
		t.Errorf("expected logs to succeed when no daemon, got error: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got: %s", stdout)
	}
}
