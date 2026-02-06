package cli

import (
	"bytes"
	"testing"
)

func TestLogFormatter_AlignsPrefixes(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewLogFormatter(&buf, []string{"api", "worker", "db"})

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

	formatter.PrintLine("svc", "hello")

	expected := "svc | hello\n"

	if buf.String() != expected {
		t.Errorf("unexpected output:\ngot:\n%s\nwant:\n%s", buf.String(), expected)
	}
}
