package e2e

import (
	"strings"
	"testing"
	"time"
)

// 2.1: Stops all services and shuts down daemon (socket removed).
func TestDown_StopsAllAndShutsDaemon(t *testing.T) {
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

	stdout, _, err := f.Run("down")
	if err != nil {
		t.Fatalf("down failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"app"}) {
		t.Errorf("expected 'app' in stopped services, got: %v", stopped)
	}

	err = f.WaitForSocketGone(5 * time.Second)
	if err != nil {
		t.Errorf("expected socket to be removed after down: %v", err)
	}
}

// 2.2: All running services appear in the stopped list.
func TestDown_MultipleRunningServices(t *testing.T) {
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

	stdout, _, err := f.Run("down")
	if err != nil {
		t.Fatalf("down failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"app1", "app2", "app3"}) {
		t.Errorf("expected all services in stopped list, got: %v", stopped)
	}

	err = f.WaitForSocketGone(5 * time.Second)
	if err != nil {
		t.Errorf("expected socket to be removed after down: %v", err)
	}
}

// 2.3: Succeeds silently when no daemon is running.
func TestDown_NoDaemon(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)

	stdout, _, err := f.Run("down")
	if err != nil {
		t.Errorf("expected down to succeed when no daemon, got error: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got: %s", stdout)
	}
}

// 2.4: Services with dependencies are stopped in correct (reverse) order.
func TestDown_WithDependencies(t *testing.T) {
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
  frontend:
    command: sleep 60
    depends_on:
      - api
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	for _, svc := range []string{"db", "api", "frontend"} {
		err = f.WaitForState(svc, "running", 10*time.Second)
		if err != nil {
			t.Fatalf("WaitForState %s failed: %v", svc, err)
		}
	}

	stdout, _, err := f.Run("down")
	if err != nil {
		t.Fatalf("down failed: %v", err)
	}

	stopped := ParseStoppedServices(stdout)
	if !ContainsAll(stopped, []string{"db", "api", "frontend"}) {
		t.Errorf("expected all services in stopped list, got: %v", stopped)
	}

	// Verify reverse dependency order: frontend before api, api before db.
	frontendIdx, apiIdx, dbIdx := -1, -1, -1
	for i, s := range stopped {
		switch s {
		case "frontend":
			frontendIdx = i
		case "api":
			apiIdx = i
		case "db":
			dbIdx = i
		}
	}
	if frontendIdx > apiIdx {
		t.Errorf("expected frontend to be stopped before api, got order: %v", stopped)
	}
	if apiIdx > dbIdx {
		t.Errorf("expected api to be stopped before db, got order: %v", stopped)
	}

	err = f.WaitForSocketGone(5 * time.Second)
	if err != nil {
		t.Errorf("expected socket to be removed after down: %v", err)
	}
}
