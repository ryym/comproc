package process

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ryym/comproc/internal/config"
)

func TestProcess_StartAndStop(t *testing.T) {
	svc := &config.Service{
		Name:    "test",
		Command: "sleep 10",
	}

	proc := New(svc)
	if proc.GetState() != StateStopped {
		t.Errorf("expected initial state to be stopped, got %s", proc.GetState())
	}

	ctx := context.Background()
	err := proc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait a bit for the process to start
	time.Sleep(50 * time.Millisecond)

	if proc.GetState() != StateRunning {
		t.Errorf("expected state to be running, got %s", proc.GetState())
	}

	if proc.PID() == 0 {
		t.Error("expected non-zero PID")
	}

	err = proc.Stop(time.Second)
	if err != nil {
		t.Fatalf("failed to stop process: %v", err)
	}

	if proc.GetState() != StateStopped {
		t.Errorf("expected state to be stopped, got %s", proc.GetState())
	}
}

func TestProcess_CaptureOutput(t *testing.T) {
	svc := &config.Service{
		Name:    "test",
		Command: "echo hello",
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	proc := New(svc)
	proc.SetOutput(&stdout, &stderr)

	ctx := context.Background()
	err := proc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait for process to complete
	<-proc.Wait()

	if proc.GetState() != StateStopped {
		t.Errorf("expected state to be stopped, got %s", proc.GetState())
	}

	output := stdout.String()
	if output != "hello\n" {
		t.Errorf("expected stdout 'hello\\n', got %q", output)
	}
}

func TestProcess_ExitCode(t *testing.T) {
	svc := &config.Service{
		Name:    "test",
		Command: "exit 42",
	}

	proc := New(svc)

	ctx := context.Background()
	err := proc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Wait for process to complete
	<-proc.Wait()

	if proc.GetState() != StateFailed {
		t.Errorf("expected state to be failed, got %s", proc.GetState())
	}

	if proc.GetExitCode() != 42 {
		t.Errorf("expected exit code 42, got %d", proc.GetExitCode())
	}
}

func TestProcess_WorkingDir(t *testing.T) {
	svc := &config.Service{
		Name:       "test",
		Command:    "pwd",
		WorkingDir: "/tmp",
	}

	var stdout bytes.Buffer
	proc := New(svc)
	proc.SetOutput(&stdout, nil)

	ctx := context.Background()
	err := proc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	<-proc.Wait()

	output := stdout.String()
	// /tmp may be a symlink to /private/tmp on macOS
	if output != "/tmp\n" && output != "/private/tmp\n" {
		t.Errorf("expected working dir to be /tmp, got %q", output)
	}
}

func TestProcess_Environment(t *testing.T) {
	svc := &config.Service{
		Name:    "test",
		Command: "echo $TEST_VAR",
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	var stdout bytes.Buffer
	proc := New(svc)
	proc.SetOutput(&stdout, nil)

	ctx := context.Background()
	err := proc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	<-proc.Wait()

	output := stdout.String()
	if output != "test_value\n" {
		t.Errorf("expected env var output 'test_value\\n', got %q", output)
	}
}

func TestProcess_DoubleStart(t *testing.T) {
	svc := &config.Service{
		Name:    "test",
		Command: "sleep 10",
	}

	proc := New(svc)

	ctx := context.Background()
	err := proc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	defer proc.Stop(time.Second)

	// Try to start again
	err = proc.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running process")
	}
}

func TestProcess_RestartCounter(t *testing.T) {
	svc := &config.Service{
		Name:    "test",
		Command: "echo test",
	}

	proc := New(svc)

	if proc.GetRestarts() != 0 {
		t.Errorf("expected initial restart count to be 0, got %d", proc.GetRestarts())
	}

	proc.IncrementRestarts()
	if proc.GetRestarts() != 1 {
		t.Errorf("expected restart count to be 1, got %d", proc.GetRestarts())
	}

	proc.IncrementRestarts()
	if proc.GetRestarts() != 2 {
		t.Errorf("expected restart count to be 2, got %d", proc.GetRestarts())
	}

	proc.ResetRestarts()
	if proc.GetRestarts() != 0 {
		t.Errorf("expected restart count to be 0 after reset, got %d", proc.GetRestarts())
	}
}
