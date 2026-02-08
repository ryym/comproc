package e2e

import (
	"strings"
	"testing"
	"time"
)

// 6.1: Retrieves recent log lines from a running service.
func TestLogs_RecentLines(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'echo "line1"; echo "line2"; echo "line3"; sleep 60'
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	var stdout string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stdout, _, err = f.Run("logs", "-n", "10")
		if err == nil && strings.Contains(stdout, "line3") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !strings.Contains(stdout, "line1") ||
		!strings.Contains(stdout, "line2") ||
		!strings.Contains(stdout, "line3") {
		t.Errorf("expected all log lines, got:\n%s", stdout)
	}
}

// 6.2: Filters logs to show only the specified service.
func TestLogs_ServiceFilter(t *testing.T) {
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
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	var stdout string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stdout, _, err = f.Run("logs", "-n", "10", "app1")
		if err == nil && strings.Contains(stdout, "from app1") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !strings.Contains(stdout, "from app1") {
		t.Errorf("expected app1 log, got:\n%s", stdout)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		plain := stripANSI(line)
		if plain != "" && !strings.HasPrefix(plain, "app1") {
			t.Errorf("expected only app1 logs, but got line: %s", line)
		}
	}
}

// 6.3: `-n 5` limits the number of returned lines.
func TestLogs_LineLimit(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do echo "line$i"; done; sleep 60'
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	var stdout string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stdout, _, err = f.Run("logs", "-n", "5")
		if err == nil && strings.Contains(stdout, "line10") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// Filter out empty lines
	var nonEmpty []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}

	if len(nonEmpty) != 5 {
		t.Errorf("expected 5 lines with -n 5, got %d:\n%s", len(nonEmpty), stdout)
	}
}

// 6.4: Returns empty output without error when no daemon runs.
func TestLogs_NoDaemon(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)

	stdout, _, err := f.Run("logs")
	if err != nil {
		t.Errorf("expected logs to succeed when no daemon, got error: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got: %s", stdout)
	}
}

// 6.5: `logs -f` streams new log lines in real time.
func TestLogs_FollowMode(t *testing.T) {
	skipIfShort(t)
	t.Parallel()

	f := NewFixture(t)
	f.WriteConfig(`
services:
  app:
    command: sh -c 'sleep 1; echo "delayed message"; sleep 60'
`)
	_, stderr, err := f.Run("up")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, stderr)
	}

	err = f.WaitForState("app", "running", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForState failed: %v", err)
	}

	cmd, outBuf, err := f.RunAsync("logs", "-f")
	if err != nil {
		t.Fatalf("RunAsync logs -f failed: %v", err)
	}

	err = WaitForContent(outBuf, "delayed message", 10*time.Second)
	if err != nil {
		t.Errorf("expected streamed log to contain 'delayed message': %v", err)
	}

	InterruptAndWait(cmd)
}
