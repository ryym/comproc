// comproc is a docker-compose-like CLI for managing multiple processes.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
		// Internal command for detached mode
		return runDaemon(socketPath, absConfigPath, cmdArgs)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runUp(socketPath, configPath string, args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	detach := fs.Bool("d", false, "Run in detached mode (background)")
	fs.Parse(args)

	if *detach {
		return startDetached(configPath, socketPath, fs.Args())
	}

	return cli.RunUp(socketPath, configPath, fs.Args())
}

// startDetached starts the daemon in a background process.
func startDetached(configPath, socketPath string, services []string) error {
	// Check if daemon is already running
	client := cli.NewClient(socketPath)
	if err := client.Connect(); err == nil {
		defer client.Close()
		// Daemon already running, just send up request
		result, err := client.Up(services)
		if err != nil {
			return err
		}
		if len(result.Started) > 0 {
			fmt.Printf("Started: %v\n", result.Started)
		}
		return nil
	}

	// Load config to resolve service names
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	targetServices := services
	if len(targetServices) == 0 {
		for name := range cfg.Services {
			targetServices = append(targetServices, name)
		}
	}

	// Start daemon process
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build arguments for daemon process
	daemonArgs := []string{"-f", configPath, "__daemon"}
	daemonArgs = append(daemonArgs, services...)

	cmd := exec.Command(exe, daemonArgs...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Detach from parent process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("Started in background (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("Services: %v\n", targetServices)

	// Don't wait for the process - it runs in background
	// Release the process so it doesn't become a zombie
	cmd.Process.Release()

	return nil
}

// runDaemon runs as the background daemon process.
func runDaemon(socketPath, configPath string, services []string) error {
	return cli.RunDaemon(socketPath, configPath, services)
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
  up [services...]      Start services
    -d                  Run in detached mode (background)

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
  comproc stop api              Stop specific services
  comproc down                  Stop all services and shut down
  comproc status                Show status of all services
  comproc logs -f api           Follow logs for api service
  comproc restart api           Restart api service`)
}
