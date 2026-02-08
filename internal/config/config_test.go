package config

import (
	"strings"
	"testing"
)

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
services:
  api:
    command: go run ./cmd/api
    working_dir: ./backend
    env:
      PORT: "8080"
    restart: on-failure
    depends_on:
      - db
  db:
    command: docker run postgres
    restart: always
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Services))
	}

	api := cfg.Services["api"]
	if api == nil {
		t.Fatal("api service not found")
	}
	if api.Name != "api" {
		t.Errorf("expected name 'api', got %q", api.Name)
	}
	if api.Command != "go run ./cmd/api" {
		t.Errorf("expected command 'go run ./cmd/api', got %q", api.Command)
	}
	if api.WorkingDir != "./backend" {
		t.Errorf("expected working_dir './backend', got %q", api.WorkingDir)
	}
	if api.Env["PORT"] != "8080" {
		t.Errorf("expected env PORT='8080', got %q", api.Env["PORT"])
	}
	if api.Restart != RestartOnFailure {
		t.Errorf("expected restart 'on-failure', got %q", api.Restart)
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "db" {
		t.Errorf("expected depends_on ['db'], got %v", api.DependsOn)
	}

	db := cfg.Services["db"]
	if db == nil {
		t.Fatal("db service not found")
	}
	if db.Restart != RestartAlways {
		t.Errorf("expected restart 'always', got %q", db.Restart)
	}
}

func TestParse_EmptyServices(t *testing.T) {
	yaml := `services: {}`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty services")
	}
	if !strings.Contains(err.Error(), "no services defined") {
		t.Errorf("expected 'no services defined' error, got: %v", err)
	}
}

func TestParse_MissingCommand(t *testing.T) {
	yaml := `
services:
  api:
    working_dir: ./backend
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Errorf("expected 'command is required' error, got: %v", err)
	}
}

func TestParse_InvalidRestartPolicy(t *testing.T) {
	yaml := `
services:
  api:
    command: go run ./cmd/api
    restart: invalid
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid restart policy")
	}
	if !strings.Contains(err.Error(), "invalid restart policy") {
		t.Errorf("expected 'invalid restart policy' error, got: %v", err)
	}
}

func TestParse_UnknownDependency(t *testing.T) {
	yaml := `
services:
  api:
    command: go run ./cmd/api
    depends_on:
      - unknown
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "unknown dependency") {
		t.Errorf("expected 'unknown dependency' error, got: %v", err)
	}
}

func TestParse_CircularDependency(t *testing.T) {
	yaml := `
services:
  a:
    command: echo a
    depends_on:
      - b
  b:
    command: echo b
    depends_on:
      - c
  c:
    command: echo c
    depends_on:
      - a
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("expected 'circular dependency' error, got: %v", err)
	}
}

func TestParse_SelfDependency(t *testing.T) {
	yaml := `
services:
  api:
    command: echo api
    depends_on:
      - api
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for self dependency")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("expected 'circular dependency' error, got: %v", err)
	}
}

func TestParse_PreservesServiceOrder(t *testing.T) {
	yaml := `
services:
  frontend:
    command: npm run dev
  api:
    command: go run ./cmd/api
  db:
    command: docker run postgres
  cache:
    command: redis-server
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"frontend", "api", "db", "cache"}
	names := cfg.ServiceNames()
	if len(names) != len(expected) {
		t.Fatalf("expected %d service names, got %d", len(expected), len(names))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ServiceNames()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestGetRestartPolicy_Default(t *testing.T) {
	s := &Service{Command: "echo test"}
	if s.GetRestartPolicy() != RestartNever {
		t.Errorf("expected default restart policy 'never', got %q", s.GetRestartPolicy())
	}
}

func TestTopologicalSort(t *testing.T) {
	yaml := `
services:
  frontend:
    command: npm run dev
    depends_on:
      - api
  api:
    command: go run ./cmd/api
    depends_on:
      - db
  db:
    command: docker run postgres
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sorted, err := cfg.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 services, got %d", len(sorted))
	}

	// Check order: db -> api -> frontend
	indexOf := func(name string) int {
		for i, s := range sorted {
			if s.Name == name {
				return i
			}
		}
		return -1
	}

	dbIdx := indexOf("db")
	apiIdx := indexOf("api")
	frontendIdx := indexOf("frontend")

	if dbIdx > apiIdx {
		t.Errorf("db should come before api, got db=%d, api=%d", dbIdx, apiIdx)
	}
	if apiIdx > frontendIdx {
		t.Errorf("api should come before frontend, got api=%d, frontend=%d", apiIdx, frontendIdx)
	}
}

func TestTopologicalSort_NoDependencies(t *testing.T) {
	yaml := `
services:
  a:
    command: echo a
  b:
    command: echo b
  c:
    command: echo c
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sorted, err := cfg.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 3 {
		t.Errorf("expected 3 services, got %d", len(sorted))
	}
}
