package daemon

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/ryym/comproc/internal/config"
	"github.com/ryym/comproc/internal/process"
)

const (
	minBackoff = 1 * time.Second
	maxBackoff = 30 * time.Second
)

// Supervisor monitors processes and handles restarts according to policy.
type Supervisor struct {
	mu sync.Mutex

	daemon   *Daemon
	monitors map[string]context.CancelFunc
}

// NewSupervisor creates a new supervisor.
func NewSupervisor(d *Daemon) *Supervisor {
	return &Supervisor{
		daemon:   d,
		monitors: make(map[string]context.CancelFunc),
	}
}

// StartMonitoring starts monitoring a process for restarts.
func (s *Supervisor) StartMonitoring(ctx context.Context, name string, proc *process.Process, svc *config.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing monitor if any
	if cancel, ok := s.monitors[name]; ok {
		cancel()
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	s.monitors[name] = cancel

	go s.monitor(monitorCtx, name, proc, svc)
}

// StopMonitoring stops monitoring a process.
func (s *Supervisor) StopMonitoring(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.monitors[name]; ok {
		cancel()
		delete(s.monitors, name)
	}
}

// monitor watches a process and restarts it according to policy.
func (s *Supervisor) monitor(ctx context.Context, name string, proc *process.Process, svc *config.Service) {
	policy := svc.GetRestartPolicy()
	consecutiveFailures := 0

	for {
		// Wait for process to exit
		select {
		case <-ctx.Done():
			return
		case <-proc.Wait():
			// Process exited
		}

		state := proc.GetState()
		exitCode := proc.GetExitCode()

		// Check if we should restart
		shouldRestart := false
		switch policy {
		case config.RestartAlways:
			shouldRestart = true
		case config.RestartOnFailure:
			shouldRestart = exitCode != 0 || state == process.StateFailed
		case config.RestartNever:
			shouldRestart = false
		}

		if !shouldRestart {
			return
		}

		// Calculate backoff
		consecutiveFailures++
		backoff := calculateBackoff(consecutiveFailures)

		// Wait before restart
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Restart the process
		proc.IncrementRestarts()
		logWriter := s.daemon.logMgr.Writer(name)
		proc.SetOutput(logWriter, logWriter)

		if err := proc.Start(ctx); err != nil {
			// Failed to restart, will try again
			continue
		}

		// Reset failure count on successful start
		// (we'll increment again if it fails quickly)
	}
}

// calculateBackoff returns the backoff duration using exponential backoff.
func calculateBackoff(failures int) time.Duration {
	// 1s, 2s, 4s, 8s, 16s, 30s (capped)
	backoff := float64(minBackoff) * math.Pow(2, float64(failures-1))
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}
	return time.Duration(backoff)
}
