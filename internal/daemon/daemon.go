// Package daemon implements the comproc daemon that manages processes.
package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ryym/comproc/internal/config"
	"github.com/ryym/comproc/internal/process"
)

// Daemon manages processes and handles RPC requests.
type Daemon struct {
	mu sync.RWMutex

	config     *config.Config
	configPath string
	processes  map[string]*process.Process
	logMgr     *LogManager

	server *Server
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new daemon instance.
func New(configPath string) (*Daemon, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute config path: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		config:     cfg,
		configPath: absConfigPath,
		processes:  make(map[string]*process.Process),
		logMgr:     NewLogManager(1000), // Keep last 1000 lines per service
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize processes
	for name, svc := range cfg.Services {
		// Resolve working directory relative to config file
		if svc.WorkingDir != "" && !filepath.IsAbs(svc.WorkingDir) {
			svc.WorkingDir = filepath.Join(filepath.Dir(absConfigPath), svc.WorkingDir)
		} else if svc.WorkingDir == "" {
			svc.WorkingDir = filepath.Dir(absConfigPath)
		}
		d.processes[name] = process.New(svc)
	}

	return d, nil
}

// SocketPath returns the path to the Unix socket.
func SocketPath() string {
	// Use XDG_RUNTIME_DIR if available, otherwise fall back to tmp
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "comproc.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("comproc-%d.sock", os.Getuid()))
}

// Run starts the daemon and blocks until it's shut down.
func (d *Daemon) Run(socketPath string) error {
	d.server = NewServer(d, socketPath)
	return d.server.Run(d.ctx)
}

// Shutdown gracefully shuts down the daemon.
func (d *Daemon) Shutdown() error {
	d.cancel()
	return d.StopAll()
}

// StartServices starts the specified services (or all if none specified).
func (d *Daemon) StartServices(services []string) (started, failed []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	toStart := services
	if len(toStart) == 0 {
		// Start all services in dependency order
		sorted, err := d.config.TopologicalSort()
		if err != nil {
			return nil, []string{"all"}
		}
		for _, svc := range sorted {
			toStart = append(toStart, svc.Name)
		}
	} else {
		// Resolve dependencies for specified services
		toStart = d.resolveDependencies(services)
	}

	for _, name := range toStart {
		proc, ok := d.processes[name]
		if !ok {
			failed = append(failed, name)
			continue
		}

		if proc.GetState() == process.StateRunning {
			// Already running
			continue
		}

		// Set up log capture
		logWriter := d.logMgr.Writer(name)
		proc.SetOutput(logWriter, logWriter)

		if err := proc.Start(d.ctx); err != nil {
			failed = append(failed, name)
		} else {
			started = append(started, name)
		}
	}

	return started, failed
}

// StopServices stops the specified services (or all if none specified).
func (d *Daemon) StopServices(services []string) (stopped []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	toStop := services
	if len(toStop) == 0 {
		// Stop all services in reverse dependency order
		sorted, _ := d.config.TopologicalSort()
		for i := len(sorted) - 1; i >= 0; i-- {
			toStop = append(toStop, sorted[i].Name)
		}
	} else {
		// Also stop dependents
		toStop = d.resolveDependents(services)
	}

	for _, name := range toStop {
		proc, ok := d.processes[name]
		if !ok {
			continue
		}

		if proc.GetState() == process.StateStopped || proc.GetState() == process.StateFailed {
			continue
		}

		if err := proc.Stop(gracefulTimeout); err == nil {
			stopped = append(stopped, name)
		}
	}

	return stopped
}

// StopAll stops all services.
func (d *Daemon) StopAll() error {
	d.StopServices(nil)
	return nil
}

// RestartServices restarts the specified services.
func (d *Daemon) RestartServices(services []string) (restarted, failed []string) {
	stopped := d.StopServices(services)
	started, startFailed := d.StartServices(stopped)
	return started, startFailed
}

// GetStatus returns the status of all services.
func (d *Daemon) GetStatus() []ServiceStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var statuses []ServiceStatus
	for name, proc := range d.processes {
		status := ServiceStatus{
			Name:     name,
			State:    string(proc.GetState()),
			PID:      proc.PID(),
			Restarts: proc.GetRestarts(),
			ExitCode: proc.GetExitCode(),
		}
		if !proc.GetStartedAt().IsZero() {
			status.StartedAt = proc.GetStartedAt().Format("2006-01-02 15:04:05")
		}
		statuses = append(statuses, status)
	}

	return statuses
}

// GetLogs returns recent logs for the specified services.
func (d *Daemon) GetLogs(services []string, lines int) []LogLine {
	if len(services) == 0 {
		for name := range d.processes {
			services = append(services, name)
		}
	}

	return d.logMgr.GetLines(services, lines)
}

// SubscribeLogs subscribes to log updates.
func (d *Daemon) SubscribeLogs(services []string) <-chan LogLine {
	if len(services) == 0 {
		for name := range d.processes {
			services = append(services, name)
		}
	}

	return d.logMgr.Subscribe(services)
}

// UnsubscribeLogs unsubscribes from log updates.
func (d *Daemon) UnsubscribeLogs(ch <-chan LogLine) {
	d.logMgr.Unsubscribe(ch)
}

// resolveDependencies returns services with their dependencies in startup order.
func (d *Daemon) resolveDependencies(services []string) []string {
	visited := make(map[string]bool)
	var result []string

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		if svc, ok := d.config.Services[name]; ok {
			for _, dep := range svc.DependsOn {
				visit(dep)
			}
		}
		result = append(result, name)
	}

	for _, name := range services {
		visit(name)
	}

	return result
}

// resolveDependents returns services with their dependents in shutdown order.
func (d *Daemon) resolveDependents(services []string) []string {
	// Build reverse dependency map
	dependents := make(map[string][]string)
	for name, svc := range d.config.Services {
		for _, dep := range svc.DependsOn {
			dependents[dep] = append(dependents[dep], name)
		}
	}

	visited := make(map[string]bool)
	var result []string

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		// First stop dependents
		for _, dep := range dependents[name] {
			visit(dep)
		}
		result = append(result, name)
	}

	for _, name := range services {
		visit(name)
	}

	return result
}

// ServiceStatus represents the status of a service (used internally).
type ServiceStatus struct {
	Name      string
	State     string
	PID       int
	Restarts  int
	StartedAt string
	ExitCode  int
}
