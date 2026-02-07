// comproc is a docker-compose-like CLI for managing multiple processes.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ryym/comproc/internal/cli"
	"github.com/ryym/comproc/internal/config"
	"github.com/ryym/comproc/internal/daemon"
)

const defaultConfigFile = "comproc.yaml"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Global flags
	var configPath string
	flag.StringVar(&configPath, "f", defaultConfigFile, "Path to config file")
	flag.StringVar(&configPath, "file", defaultConfigFile, "Path to config file")
	flag.Usage = printUsage

	// Parse to find the subcommand
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		printUsage()
		return nil
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}

	socketPath := daemon.SocketPath(absConfigPath)
	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "up":
		return runUp(socketPath, absConfigPath, cmdArgs)
	case "down":
		return cli.RunDown(socketPath)
	case "stop":
		return runStop(socketPath, cmdArgs)
	case "status", "ps":
		return cli.RunStatus(socketPath, absConfigPath)
	case "restart":
		return runRestart(socketPath, cmdArgs)
	case "logs":
		return runLogs(socketPath, cmdArgs)
	case "attach":
		return runAttach(socketPath, cmdArgs)
	case "__daemon":
		// Internal command: runs the daemon process
		return runDaemon(socketPath, absConfigPath)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runUp(socketPath, configPath string, args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow log output after starting")
	fs.Parse(args)

	services := fs.Args()

	// Ensure daemon is running (spawn if needed, wait for socket)
	if err := ensureDaemon(configPath, socketPath); err != nil {
		return err
	}

	// Connect and send Up RPC
	client := cli.NewClient(socketPath)
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

	if *follow {
		return cli.RunLogs(socketPath, services, 100, true)
	}

	return nil
}

// ensureDaemon ensures a daemon process is running and its socket is ready.
// If no daemon is running, it validates the config, spawns a background
// daemon process, and waits for the socket to become available.
func ensureDaemon(configPath, socketPath string) error {
	// Check if daemon is already running
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return nil
	}

	// Validate config before spawning to catch errors immediately
	if _, err := config.Load(configPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Start daemon process
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(exe, "-f", configPath, "__daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Don't wait for the process - it runs in background
	cmd.Process.Release()

	// Wait for socket to be ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for daemon to start")
}

// runDaemon runs as the background daemon process.
func runDaemon(socketPath, configPath string) error {
	return cli.RunDaemon(socketPath, configPath)
}

func runStop(socketPath string, args []string) error {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	fs.Parse(args)

	return cli.RunStop(socketPath, fs.Args())
}

func runRestart(socketPath string, args []string) error {
	fs := flag.NewFlagSet("restart", flag.ExitOnError)
	fs.Parse(args)

	return cli.RunRestart(socketPath, fs.Args())
}

func runAttach(socketPath string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("attach requires exactly one service name")
	}
	return cli.RunAttach(socketPath, args[0])
}

func runLogs(socketPath string, args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow log output")
	lines := fs.Int("n", 100, "Number of lines to show")
	fs.Parse(args)

	return cli.RunLogs(socketPath, fs.Args(), *lines, *follow)
}

func printUsage() {
	fmt.Println(`comproc - Process manager

Usage:
  comproc [options] <command> [args]

Options:
  -f, --file <path>   Path to config file (default: comproc.yaml)

Commands:
  up [services...]      Start services (daemon runs in background)
    -f                  Follow log output after starting

  down                  Stop all services and shut down

  stop [services...]    Stop services (without shutting down)

  status, ps            Show service status

  restart [services...] Restart services

  logs [services...]    Show service logs
    -f                  Follow log output
    -n <lines>          Number of lines to show (default: 100)

  attach <service>      Attach to a service (forward stdin, stream logs)

Examples:
  comproc up                    Start all services
  comproc up api db             Start specific services
  comproc up -f                 Start all services and follow logs
  comproc stop api              Stop specific services
  comproc down                  Stop all services and shut down
  comproc status                Show status of all services
  comproc logs -f api           Follow logs for api service
  comproc restart api           Restart api service`)
}
