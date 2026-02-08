package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/ryym/comproc/internal/config"
	"github.com/ryym/comproc/internal/daemon"
	"github.com/ryym/comproc/internal/protocol"
)

// RunUp executes the 'up' command — starts services and optionally follows logs.
func RunUp(socketPath string, services []string, follow bool) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer client.Close()

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

	if follow {
		return streamLogs(client, services, 100, true)
	}

	return nil
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
	for _, name := range cfg.ServiceNames() {
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

	return streamLogs(client, services, lines, follow)
}

// streamLogs fetches and displays logs, optionally following new output until interrupted.
func streamLogs(client *Client, services []string, lines int, follow bool) error {
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

	for _, entry := range result.Lines {
		formatter.PrintLine(entry.Service, entry.Line)
	}

	if !follow {
		return nil
	}

	// Handle Ctrl-C by closing the connection to unblock ReadNotification.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		client.Close()
	}()

	for {
		notification, err := client.ReadNotification()
		if err != nil {
			return nil
		}

		if notification.Method == protocol.MethodLog {
			var entry protocol.LogEntry
			if err := notification.ParseParams(&entry); err == nil {
				formatter.PrintLine(entry.Service, entry.Line)
			}
		}
	}
}

// RunAttach executes the 'attach' command.
func RunAttach(socketPath string, service string) error {
	client := NewClient(socketPath)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("daemon is not running")
	}
	defer client.Close()

	// Get all service names for log formatting
	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("status failed: %w", err)
	}
	var serviceNames []string
	for _, svc := range status.Services {
		serviceNames = append(serviceNames, svc.Name)
	}
	formatter := NewLogFormatter(os.Stdout, serviceNames)

	// Attach to the service
	result, err := client.Attach(service)
	if err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	// Display initial logs
	for _, entry := range result.Lines {
		formatter.PrintLine(entry.Service, entry.Line)
	}

	// Read stdin and send to daemon in a goroutine
	stdinDone := make(chan struct{})
	go func() {
		defer close(stdinDone)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			if err := client.SendStdin(line); err != nil {
				return
			}
		}
	}()

	// Handle Ctrl-C by closing the connection to detach
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			client.Close()
		case <-stdinDone:
			client.Close()
		}
	}()

	// Read log notifications from daemon
	for {
		notification, err := client.ReadNotification()
		if err != nil {
			return nil
		}

		if notification.Method == protocol.MethodLog {
			var entry protocol.LogEntry
			if err := notification.ParseParams(&entry); err == nil {
				formatter.PrintLine(entry.Service, entry.Line)
			}
		}
	}
}

// RunDaemon runs the daemon process.
func RunDaemon(socketPath, configPath string) error {
	d, err := daemon.New(configPath)
	if err != nil {
		return err
	}

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
