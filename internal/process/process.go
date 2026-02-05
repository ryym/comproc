// Package process handles starting, stopping, and monitoring processes.
package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/ryym/comproc/internal/config"
)

// State represents the current state of a process.
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateFailed   State = "failed"
)

// Process represents a managed process.
type Process struct {
	mu sync.RWMutex

	Service *config.Service
	State   State

	cmd       *exec.Cmd
	startedAt time.Time
	exitCode  int
	restarts  int

	stdout io.Writer
	stderr io.Writer

	// done is closed when the process exits
	done chan struct{}
	// cancel cancels the process context
	cancel context.CancelFunc
}

// New creates a new process for the given service.
func New(svc *config.Service) *Process {
	return &Process{
		Service: svc,
		State:   StateStopped,
	}
}

// SetOutput sets the stdout and stderr writers for the process.
func (p *Process) SetOutput(stdout, stderr io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stdout = stdout
	p.stderr = stderr
}

// Start starts the process.
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.State == StateRunning || p.State == StateStarting {
		return fmt.Errorf("process already running")
	}

	p.State = StateStarting

	// Create a cancellable context
	procCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})

	// Build the command
	cmd := exec.CommandContext(procCtx, "sh", "-c", p.Service.Command)
	cmd.Dir = p.Service.WorkingDir

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range p.Service.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Set process group so we can kill all children
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Set output
	if p.stdout != nil {
		cmd.Stdout = p.stdout
	}
	if p.stderr != nil {
		cmd.Stderr = p.stderr
	}

	p.cmd = cmd

	if err := cmd.Start(); err != nil {
		p.State = StateFailed
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.startedAt = time.Now()
	p.State = StateRunning

	// Monitor the process in a goroutine
	go p.monitor()

	return nil
}

// monitor waits for the process to exit and updates state.
func (p *Process) monitor() {
	err := p.cmd.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd.ProcessState != nil {
		p.exitCode = p.cmd.ProcessState.ExitCode()
	}

	if p.State == StateStopping {
		p.State = StateStopped
	} else if err != nil {
		p.State = StateFailed
	} else {
		p.State = StateStopped
	}

	close(p.done)
}

// Stop stops the process gracefully, or forcefully after timeout.
func (p *Process) Stop(timeout time.Duration) error {
	p.mu.Lock()

	if p.State != StateRunning && p.State != StateStarting {
		p.mu.Unlock()
		return nil
	}

	p.State = StateStopping
	done := p.done
	cmd := p.cmd
	p.mu.Unlock()

	// Send SIGTERM to process group
	if cmd.Process != nil {
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			syscall.Kill(-pgid, syscall.SIGTERM)
		}
	}

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		// Force kill
		if cmd.Process != nil {
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, syscall.SIGKILL)
			}
		}
		<-done
		return nil
	}
}

// Wait waits for the process to exit.
func (p *Process) Wait() <-chan struct{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.done
}

// GetState returns the current state of the process.
func (p *Process) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// GetExitCode returns the exit code of the last run.
func (p *Process) GetExitCode() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitCode
}

// GetStartedAt returns when the process was started.
func (p *Process) GetStartedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.startedAt
}

// GetRestarts returns the number of times this process has been restarted.
func (p *Process) GetRestarts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.restarts
}

// IncrementRestarts increments the restart counter.
func (p *Process) IncrementRestarts() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.restarts++
}

// ResetRestarts resets the restart counter.
func (p *Process) ResetRestarts() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.restarts = 0
}

// PID returns the process ID, or 0 if not running.
func (p *Process) PID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}
