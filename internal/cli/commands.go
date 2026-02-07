package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/ryym/comproc/internal/config"
	"github.com/ryym/comproc/internal/daemon"
	"github.com/ryym/comproc/internal/protocol"
)

// RunUp executes the 'up' command (foreground mode).
func RunUp(socketPath string, configPath string, services []string) error {
	// Check if daemon is running
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		// Daemon not running, start it in foreground
		return runDaemonForeground(configPath, socketPath, services)
	}
	defer client.Close()

	// Daemon is running, send up request
	result, err := client.Up(services)
	if err != nil {
		return fmt.Errorf("up failed: %w", err)
	}

	if len(result.Started) > 0 {
		fmt.Printf("Started: %v\n", result.Started)
	}
	if len(result.Failed) > 0 {
		fmt.Printf("Failed: %v\n", result.Failed)
		return fmt.Errorf("some services failed to start")
	}

	// Foreground mode: follow logs until interrupted
	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("status failed: %w", err)
	}
	var serviceNames []string
	for _, svc := range status.Services {
		serviceNames = append(serviceNames, svc.Name)
	}
	formatter := NewLogFormatter(os.Stdout, serviceNames)

	logsResult, err := client.Logs(services, 100, true)
	if err != nil {
		return fmt.Errorf("logs failed: %w", err)
	}

	for _, entry := range logsResult.Lines {
		printLogEntry(formatter, &entry)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		notification, err := client.ReadNotification()
		if err != nil {
			return nil
		}

		if notification.Method == protocol.MethodLog {
			var entry protocol.LogEntry
			if err := notification.ParseParams(&entry); err == nil {
				printLogEntry(formatter, &entry)
			}
		}
	}
}

// RunDown executes the 'down' command — stops all services and shuts down the daemon.
func RunDown(socketPath string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		// Daemon not running, nothing to do
		return nil
	}
	defer client.Close()

	result, err := client.Shutdown()
	if err != nil {
		return fmt.Errorf("down failed: %w", err)
	}

	if len(result.Stopped) > 0 {
		fmt.Printf("Stopped: %v\n", result.Stopped)
	}

	return nil
}

// RunStop executes the 'stop' command — stops specified services without shutting down the daemon.
func RunStop(socketPath string, services []string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		fmt.Println("No services running")
		return nil
	}
	defer client.Close()

	result, err := client.Down(services)
	if err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	if len(result.Stopped) > 0 {
		fmt.Printf("Stopped: %v\n", result.Stopped)
	}

	return nil
}

// RunStatus executes the 'status' command.
func RunStatus(socketPath, configPath string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		return showOfflineStatus(configPath)
	}
	defer client.Close()

	result, err := client.Status()
	if err != nil {
		return fmt.Errorf("status failed: %w", err)
	}

	if len(result.Services) == 0 {
		fmt.Println("No services")
		return nil
	}

	printStatusTable(result.Services)
	return nil
}

// showOfflineStatus loads the config file and shows all services as stopped.
func showOfflineStatus(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Println("No services defined")
		return nil
	}

	var services []protocol.ServiceStatus
	for name := range cfg.Services {
		services = append(services, protocol.ServiceStatus{
			Name:  name,
			State: "stopped",
		})
	}

	printStatusTable(services)
	return nil
}

func printStatusTable(services []protocol.ServiceStatus) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tPID\tRESTARTS\tSTARTED")
	for _, svc := range services {
		pid := "-"
		if svc.PID > 0 {
			pid = fmt.Sprintf("%d", svc.PID)
		}
		started := "-"
		if svc.StartedAt != "" {
			started = svc.StartedAt
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", svc.Name, svc.State, pid, svc.Restarts, started)
	}
	w.Flush()
}

// RunRestart executes the 'restart' command.
func RunRestart(socketPath string, services []string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		fmt.Println("No services running")
		return nil
	}
	defer client.Close()

	result, err := client.Restart(services)
	if err != nil {
		return fmt.Errorf("restart failed: %w", err)
	}

	if len(result.Restarted) > 0 {
		fmt.Printf("Restarted: %v\n", result.Restarted)
	}
	if len(result.Failed) > 0 {
		fmt.Printf("Failed: %v\n", result.Failed)
		return fmt.Errorf("some services failed to restart")
	}

	return nil
}

// RunLogs executes the 'logs' command.
func RunLogs(socketPath string, services []string, lines int, follow bool) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		return nil
	}
	defer client.Close()

	// Get all service names for proper alignment
	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("status failed: %w", err)
	}
	var serviceNames []string
	for _, svc := range status.Services {
		serviceNames = append(serviceNames, svc.Name)
	}
	formatter := NewLogFormatter(os.Stdout, serviceNames)

	result, err := client.Logs(services, lines, follow)
	if err != nil {
		return fmt.Errorf("logs failed: %w", err)
	}

	// Print initial logs
	for _, entry := range result.Lines {
		printLogEntry(formatter, &entry)
	}

	if !follow {
		return nil
	}

	// Follow mode: read notifications
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		notification, err := client.ReadNotification()
		if err != nil {
			return nil
		}

		if notification.Method == protocol.MethodLog {
			var entry protocol.LogEntry
			if err := notification.ParseParams(&entry); err == nil {
				printLogEntry(formatter, &entry)
			}
		}
	}
}

func printLogEntry(formatter *LogFormatter, entry *protocol.LogEntry) {
	formatter.PrintLine(entry.Service, entry.Line)
}

// runDaemonForeground runs the daemon in the foreground and starts services.
func runDaemonForeground(configPath string, socketPath string, services []string) error {
	d, err := daemon.New(configPath)
	if err != nil {
		return err
	}

	// Create formatter with all service names
	formatter := NewLogFormatter(os.Stdout, d.ServiceNames())

	// Subscribe to logs before starting services
	logCh := d.SubscribeLogs(nil)
	go func() {
		for line := range logCh {
			formatter.PrintLine(line.Service, line.Line)
		}
	}()

	// Start services
	started, failed := d.StartServices(services)
	if len(started) > 0 {
		fmt.Printf("Started: %v\n", started)
	}
	if len(failed) > 0 {
		fmt.Printf("Failed to start: %v\n", failed)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		d.Shutdown()
	}()

	// Run the daemon (this blocks)
	return d.Run(socketPath)
}

// RunDaemon runs the daemon (used by detached mode).
func RunDaemon(socketPath, configPath string, services []string) error {
	d, err := daemon.New(configPath)
	if err != nil {
		return err
	}

	// Start services
	d.StartServices(services)

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		d.Shutdown()
	}()

	// Run the daemon (this blocks)
	return d.Run(socketPath)
}
