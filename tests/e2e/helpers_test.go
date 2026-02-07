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
		// Try graceful shutdown first
		f.Run("down")
		// Stop daemon process if still running
		if f.daemonCmd != nil && f.daemonCmd.Process != nil {
			f.daemonCmd.Process.Signal(os.Interrupt)
			f.daemonCmd.Wait()
		}
		// Clean up temp dir
		os.RemoveAll(tmpDir)
	})

	return f
}

// WriteConfig writes a YAML config file to the temp directory.
func (f *Fixture) WriteConfig(yaml string) string {
	f.t.Helper()

	configPath := filepath.Join(f.TempDir, "comproc.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
		f.t.Fatalf("failed to write config: %v", err)
	}
	f.ConfigPath = configPath
	return configPath
}

// Run executes a comproc command and waits for it to complete.
func (f *Fixture) Run(args ...string) (stdout, stderr string, err error) {
	return f.RunWithTimeout(30*time.Second, args...)
}

// RunWithTimeout executes a comproc command with a custom timeout.
func (f *Fixture) RunWithTimeout(timeout time.Duration, args ...string) (stdout, stderr string, err error) {
	f.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = append(os.Environ(), "COMPROC_SOCKET="+f.SocketPath)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// StartDaemon starts the daemon in background and waits for it to be ready.
func (f *Fixture) StartDaemon(config string) error {
	f.t.Helper()

	f.WriteConfig(config)

	// Start daemon with up (always runs in background)
	stdout, stderr, err := f.RunWithTimeout(10*time.Second, "-f", f.ConfigPath, "up")
	if err != nil {
		return fmt.Errorf("failed to start daemon: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Wait for socket to be available
	if err := f.WaitForSocket(5 * time.Second); err != nil {
		return fmt.Errorf("socket not ready: %v", err)
	}

	return nil
}

// StartDaemonWithLogs starts services with log following (non-blocking).
// The returned command runs `up -f` in the background, streaming logs to outBuf.
func (f *Fixture) StartDaemonWithLogs(config string) (*exec.Cmd, *SyncBuffer, error) {
	f.t.Helper()

	f.WriteConfig(config)

	cmd := exec.Command(binPath, "-f", f.ConfigPath, "up", "-f")
	cmd.Env = append(os.Environ(), "COMPROC_SOCKET="+f.SocketPath)

	outBuf := &SyncBuffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	f.daemonCmd = cmd

	// Wait for socket to be available
	if err := f.WaitForSocket(10 * time.Second); err != nil {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
		return nil, nil, fmt.Errorf("socket not ready: %v", err)
	}

	return cmd, outBuf, nil
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

	var args []string
	if f.ConfigPath != "" {
		args = append(args, "-f", f.ConfigPath)
	}
	args = append(args, "status")

	stdout, _, err := f.Run(args...)
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

// Down stops all services and shuts down the daemon.
func (f *Fixture) Down() (string, error) {
	f.t.Helper()

	stdout, stderr, err := f.Run("down")
	if err != nil {
		return "", fmt.Errorf("down failed: %v\nstderr: %s", err, stderr)
	}
	return stdout, nil
}

// Stop stops specific services without shutting down the daemon.
func (f *Fixture) Stop(services ...string) (string, error) {
	f.t.Helper()

	args := append([]string{"stop"}, services...)
	stdout, stderr, err := f.Run(args...)
	if err != nil {
		return "", fmt.Errorf("stop failed: %v\nstderr: %s", err, stderr)
	}
	return stdout, nil
}

// Restart restarts services.
func (f *Fixture) Restart(services ...string) (string, error) {
	f.t.Helper()

	args := append([]string{"restart"}, services...)
	stdout, stderr, err := f.Run(args...)
	if err != nil {
		return "", fmt.Errorf("restart failed: %v\nstderr: %s", err, stderr)
	}
	return stdout, nil
}

// Logs gets logs for services.
func (f *Fixture) Logs(lines int, services ...string) (string, error) {
	f.t.Helper()

	args := []string{"logs", "-n", strconv.Itoa(lines)}
	args = append(args, services...)
	stdout, stderr, err := f.Run(args...)
	if err != nil {
		return "", fmt.Errorf("logs failed: %v\nstderr: %s", err, stderr)
	}
	return stdout, nil
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
