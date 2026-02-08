package e2e

import (
	"strings"
	"testing"
	"time"
)

func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
}

func TestDependencies_StopOrder(t *testing.T) {
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

// --- Error Handling Tests ---

func TestUp_InvalidConfig(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    restart: always
`)

	_, stderr, err := f.Run("up")

	if err == nil {
		t.Errorf("expected error for missing command")
	}
	if !strings.Contains(stderr, "command is required") {
		t.Errorf("expected 'command is required' error, got: %s", stderr)
	}
}

func TestUp_CircularDependency(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  a:
    command: sleep 60
    depends_on:
      - b
  b:
    command: sleep 60
    depends_on:
      - a
`)

	_, stderr, err := f.Run("up")

	if err == nil {
		t.Errorf("expected error for circular dependency")
	}
	if !strings.Contains(stderr, "circular dependency") {
		t.Errorf("expected 'circular dependency' error, got: %s", stderr)
	}
}
