package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Fixture provides an isolated test environment for each test.
type Fixture struct {
	t          *testing.T
	TempDir    string
	SocketPath string
	ConfigPath string

	daemonCmd *exec.Cmd
}

// NewFixture creates a new test fixture with isolated socket and temp directory.
func NewFixture(t *testing.T) *Fixture {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "comproc-e2e-fixture-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	socketPath := filepath.Join(tmpDir, "comproc.sock")

	f := &Fixture{
		t:          t,
		TempDir:    tmpDir,
		SocketPath: socketPath,
	}

	t.Cleanup(func() {
		f.Run("down")
		if f.daemonCmd != nil && f.daemonCmd.Process != nil {
			InterruptAndWait(f.daemonCmd)
		}
		os.RemoveAll(tmpDir)
	})

	return f
}

// WriteConfig writes a YAML config file to the temp directory.
func (f *Fixture) WriteConfig(yaml string) {
	f.t.Helper()

	configPath := filepath.Join(f.TempDir, "comproc.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		f.t.Fatalf("failed to write config: %v", err)
	}
	f.ConfigPath = configPath
}

// Run executes `comproc [-f <configPath>] <args...>` and waits for it to complete.
// The -f flag is prepended automatically when WriteConfig has been called.
func (f *Fixture) Run(args ...string) (stdout, stderr string, err error) {
	f.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullArgs := f.buildArgs(args...)
	cmd := exec.CommandContext(ctx, binPath, fullArgs...)
	cmd.Env = append(os.Environ(), "COMPROC_SOCKET="+f.SocketPath)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// RunAsync starts `comproc [-f <configPath>] <args...>` without waiting for completion.
// The command runs in its own process group so InterruptAndWait can simulate Ctrl-C.
// Returns the running command and a thread-safe output buffer.
func (f *Fixture) RunAsync(args ...string) (*exec.Cmd, *SyncBuffer, error) {
	f.t.Helper()

	fullArgs := f.buildArgs(args...)
	cmd := exec.Command(binPath, fullArgs...)
	cmd.Env = append(os.Environ(), "COMPROC_SOCKET="+f.SocketPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	outBuf := &SyncBuffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	f.daemonCmd = cmd
	return cmd, outBuf, nil
}

// buildArgs prepends `-f <configPath>` when a config has been written.
func (f *Fixture) buildArgs(args ...string) []string {
	if f.ConfigPath != "" {
		return append([]string{"-f", f.ConfigPath}, args...)
	}
	return args
}

// InterruptAndWait simulates Ctrl-C by sending SIGINT to the process group
// of the given command, then waits for it to exit. This requires the command
// to have been started with SysProcAttr.Setpgid = true.
func InterruptAndWait(cmd *exec.Cmd) error {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		return fmt.Errorf("failed to send SIGINT to process group: %w", err)
	}
	return cmd.Wait()
}

// WaitForSocket waits until the daemon socket is available.
func (f *Fixture) WaitForSocket(timeout time.Duration) error {
	f.t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", f.SocketPath, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket %s", f.SocketPath)
}

// WaitForSocketGone waits until the daemon socket is removed.
func (f *Fixture) WaitForSocketGone(timeout time.Duration) error {
	f.t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(f.SocketPath); os.IsNotExist(err) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket %s to be removed", f.SocketPath)
}

// ServiceStatus represents parsed status of a single service.
type ServiceStatus struct {
	Name     string
	State    string
	PID      int
	Restarts int
	Started  string
}

// GetStatus runs the status command and parses the output.
func (f *Fixture) GetStatus() ([]ServiceStatus, error) {
	f.t.Helper()

	stdout, _, err := f.Run("status")
	if err != nil {
		return nil, err
	}

	return parseStatusOutput(stdout), nil
}

// parseStatusOutput parses the tabular status output.
func parseStatusOutput(output string) []ServiceStatus {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil
	}

	// Skip header line
	var statuses []ServiceStatus
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pid := 0
		if fields[2] != "-" {
			pid, _ = strconv.Atoi(fields[2])
		}
		restarts, _ := strconv.Atoi(fields[3])

		statuses = append(statuses, ServiceStatus{
			Name:     fields[0],
			State:    fields[1],
			PID:      pid,
			Restarts: restarts,
			Started:  fields[4],
		})
	}

	return statuses
}

// WaitForState polls until the service reaches the specified state.
func (f *Fixture) WaitForState(service, state string, timeout time.Duration) error {
	f.t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		statuses, err := f.GetStatus()
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		for _, s := range statuses {
			if s.Name == service && s.State == state {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Get final status for error message
	statuses, _ := f.GetStatus()
	var currentState string
	for _, s := range statuses {
		if s.Name == service {
			currentState = s.State
			break
		}
	}

	return fmt.Errorf("timeout waiting for %s to reach state %s (current: %s)", service, state, currentState)
}

// GetServiceStatus returns the status of a specific service.
func (f *Fixture) GetServiceStatus(service string) (*ServiceStatus, error) {
	f.t.Helper()

	statuses, err := f.GetStatus()
	if err != nil {
		return nil, err
	}

	for _, s := range statuses {
		if s.Name == service {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("service %s not found", service)
}

// ParseStartedServices parses "Started: [svc1 svc2]" output.
func ParseStartedServices(output string) []string {
	return parseServiceList(output, "Started:")
}

// ParseStoppedServices parses "Stopped: [svc1 svc2]" output.
func ParseStoppedServices(output string) []string {
	return parseServiceList(output, "Stopped:")
}

// ParseRestartedServices parses "Restarted: [svc1 svc2]" output.
func ParseRestartedServices(output string) []string {
	return parseServiceList(output, "Restarted:")
}

// parseServiceList extracts services from output like "Prefix: [svc1 svc2]".
func parseServiceList(output, prefix string) []string {
	re := regexp.MustCompile(prefix + `\s*\[([^\]]*)\]`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 || matches[1] == "" {
		return nil
	}
	return strings.Fields(matches[1])
}

// stripANSI removes ANSI escape sequences from a string.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// ContainsAll checks if all expected items are present in actual.
func ContainsAll(actual, expected []string) bool {
	set := make(map[string]bool)
	for _, s := range actual {
		set[s] = true
	}
	for _, s := range expected {
		if !set[s] {
			return false
		}
	}
	return true
}

// WaitForContent polls the buffer until it contains the expected substring.
func WaitForContent(buf *SyncBuffer, substr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q in output (got: %s)", substr, buf.String())
}

// SyncBuffer is a thread-safe buffer for capturing command output.
type SyncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer.
func (b *SyncBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the buffer contents as a string.
func (b *SyncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
