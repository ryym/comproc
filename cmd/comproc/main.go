// comproc is a docker-compose-like CLI for managing multiple processes.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryym/comproc/internal/cli"
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

	// Parse to find the subcommand
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		printUsage()
		return nil
	}

	socketPath := daemon.SocketPath()
	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "up":
		return runUp(socketPath, configPath, cmdArgs)
	case "down":
		return runDown(socketPath, cmdArgs)
	case "status", "ps":
		return cli.RunStatus(socketPath)
	case "restart":
		return runRestart(socketPath, cmdArgs)
	case "logs":
		return runLogs(socketPath, cmdArgs)
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

	// Resolve config path
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("invalid config path: %w", err)
	}

	return cli.RunUp(socketPath, absConfigPath, fs.Args(), *detach)
}

func runDown(socketPath string, args []string) error {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	fs.Parse(args)

	return cli.RunDown(socketPath, fs.Args())
}

func runRestart(socketPath string, args []string) error {
	fs := flag.NewFlagSet("restart", flag.ExitOnError)
	fs.Parse(args)

	return cli.RunRestart(socketPath, fs.Args())
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

  down [services...]    Stop services

  status, ps            Show service status

  restart [services...] Restart services

  logs [services...]    Show service logs
    -f                  Follow log output
    -n <lines>          Number of lines to show (default: 100)

Examples:
  comproc up                    Start all services
  comproc up api db             Start specific services
  comproc down                  Stop all services
  comproc status                Show status of all services
  comproc logs -f api           Follow logs for api service
  comproc restart api           Restart api service`)
}
