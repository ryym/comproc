package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/ryym/comproc/internal/daemon"
	"github.com/ryym/comproc/internal/protocol"
)

// RunUp executes the 'up' command.
func RunUp(socketPath string, configPath string, services []string, detach bool) error {
	// Check if daemon is running
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		// Daemon not running, start it
		if detach {
			return startDaemonDetached(configPath, socketPath)
		}
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

	return nil
}

// RunDown executes the 'down' command.
func RunDown(socketPath string, services []string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		fmt.Println("Daemon is not running")
		return nil
	}
	defer client.Close()

	result, err := client.Down(services)
	if err != nil {
		return fmt.Errorf("down failed: %w", err)
	}

	if len(result.Stopped) > 0 {
		fmt.Printf("Stopped: %v\n", result.Stopped)
	}

	return nil
}

// RunStatus executes the 'status' command.
func RunStatus(socketPath string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		fmt.Println("Daemon is not running")
		return nil
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tPID\tRESTARTS\tSTARTED")
	for _, svc := range result.Services {
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

	return nil
}

// RunRestart executes the 'restart' command.
func RunRestart(socketPath string, services []string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("daemon is not running")
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
		return fmt.Errorf("daemon is not running")
	}
	defer client.Close()

	result, err := client.Logs(services, lines, follow)
	if err != nil {
		return fmt.Errorf("logs failed: %w", err)
	}

	// Print initial logs
	for _, entry := range result.Lines {
		printLogEntry(&entry)
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
				printLogEntry(&entry)
			}
		}
	}
}

func printLogEntry(entry *protocol.LogEntry) {
	fmt.Printf("%s | %s\n", entry.Service, entry.Line)
}

// runDaemonForeground runs the daemon in the foreground and starts services.
func runDaemonForeground(configPath string, socketPath string, services []string) error {
	d, err := daemon.New(configPath)
	if err != nil {
		return err
	}

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

// startDaemonDetached starts the daemon in the background.
func startDaemonDetached(configPath string, socketPath string) error {
	// For now, we don't support true detached mode
	// The user should run `comproc up` without -d in the background
	return fmt.Errorf("detached mode not yet implemented; run 'comproc up' without -d or use '&'")
}
