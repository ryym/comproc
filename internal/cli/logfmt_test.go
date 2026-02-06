package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogFormatter_AlignsPrefixes(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, []string{"api", "worker", "db"})
	formatter.SetColorEnabled(false)

	formatter.PrintLine("api", "started")
	formatter.PrintLine("worker", "processing")
	formatter.PrintLine("db", "connected")

	expected := "" +
		"api    | started\n" +
		"worker | processing\n" +
		"db     | connected\n"

	if buf.String() != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestLogFormatter_HandlesNewServiceName(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, []string{"api"})
	formatter.SetColorEnabled(false)

	formatter.PrintLine("api", "started")
	// A new service with a longer name appears
	formatter.PrintLine("longservice", "running")
	formatter.PrintLine("api", "done")

	expected := "" +
		"api | started\n" +
		"longservice | running\n" +
		"api         | done\n"

	if buf.String() != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestLogFormatter_EmptyServiceList(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, nil)
	formatter.SetColorEnabled(false)

	formatter.PrintLine("svc", "hello")

	expected := "svc | hello\n"

	if buf.String() != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestLogFormatter_ColorOutput(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, []string{"api", "worker"})

	formatter.PrintLine("api", "hello")
	output := buf.String()

	// Check that ANSI color codes are present
	if !strings.Contains(output, "\033[") {
		t.Errorf("expected color codes in output, got: %q", output)
	}
	if !strings.Contains(output, colorReset) {
		t.Errorf("expected color reset code in output, got: %q", output)
	}
}

func TestLogFormatter_AssignsConsistentColors(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, []string{"api", "worker"})

	formatter.PrintLine("api", "first")
	firstOutput := buf.String()
	buf.Reset()

	formatter.PrintLine("worker", "second")
	buf.Reset()

	formatter.PrintLine("api", "third")
	thirdOutput := buf.String()

	// Extract color code from outputs (format: \033[XXm)
	getColor := func(s string) string {
		start := strings.Index(s, "\033[")
		if start == -1 {
			return ""
		}
		end := strings.Index(s[start:], "m")
		if end == -1 {
			return ""
		}
		return s[start : start+end+1]
	}

	firstColor := getColor(firstOutput)
	thirdColor := getColor(thirdOutput)

	if firstColor != thirdColor {
		t.Errorf("same service should have consistent color: first=%q, third=%q", firstColor, thirdColor)
	}
}

func TestLogFormatter_AssignsDifferentColors(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, []string{"api", "worker", "db"})

	formatter.PrintLine("api", "line")
	apiOutput := buf.String()
	buf.Reset()

	formatter.PrintLine("worker", "line")
	workerOutput := buf.String()
	buf.Reset()

	formatter.PrintLine("db", "line")
	dbOutput := buf.String()

	getColor := func(s string) string {
		start := strings.Index(s, "\033[")
		if start == -1 {
			return ""
		}
		end := strings.Index(s[start:], "m")
		if end == -1 {
			return ""
		}
		return s[start : start+end+1]
	}

	apiColor := getColor(apiOutput)
	workerColor := getColor(workerOutput)
	dbColor := getColor(dbOutput)

	if apiColor == workerColor || workerColor == dbColor || apiColor == dbColor {
		t.Errorf("different services should have different colors: api=%q, worker=%q, db=%q",
			apiColor, workerColor, dbColor)
	}
}
