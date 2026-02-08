package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 8.1: Environment variables from config are passed to the process.
func TestConfig_EnvVars(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo "MY_VAR=$MY_VAR"; echo "OTHER=$OTHER"; sleep 60'
    env:
      MY_VAR: hello
      OTHER: world
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	var stdout string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stdout, _, err = f.Run("logs", "-n", "10")
		if err == nil && strings.Contains(stdout, "OTHER=world") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !strings.Contains(stdout, "MY_VAR=hello") {
		t.Errorf("expected MY_VAR=hello in logs, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "OTHER=world") {
		t.Errorf("expected OTHER=world in logs, got:\n%s", stdout)
	}
}

// 8.2: working_dir is used as the process's working directory.
func TestConfig_WorkingDir(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)

	// Create a subdirectory within the temp dir
	subDir := filepath.Join(f.TempDir, "myworkdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo "cwd=$(pwd)"; sleep 60'
    working_dir: myworkdir
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	var stdout string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stdout, _, err = f.Run("logs", "-n", "10")
		if err == nil && strings.Contains(stdout, "cwd=") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !strings.Contains(stdout, "cwd="+subDir) {
		t.Errorf("expected working directory to be %s, got:\n%s", subDir, stdout)
	}
}
