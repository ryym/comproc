package daemon

import (
	"testing"
	"time"
)

func TestRingBuffer_Add(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Add(LogLine{Line: "a"})
	buf.Add(LogLine{Line: "b"})

	if buf.Len() != 2 {
		t.Errorf("expected length 2, got %d", buf.Len())
	}

	lines := buf.GetAll()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Line != "a" || lines[1].Line != "b" {
		t.Errorf("expected [a, b], got [%s, %s]", lines[0].Line, lines[1].Line)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Add(LogLine{Line: "a"})
	buf.Add(LogLine{Line: "b"})
	buf.Add(LogLine{Line: "c"})
	buf.Add(LogLine{Line: "d"})
	buf.Add(LogLine{Line: "e"})

	if buf.Len() != 3 {
		t.Errorf("expected length 3, got %d", buf.Len())
	}

	lines := buf.GetAll()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// Should have c, d, e (oldest 'a' and 'b' dropped)
	if lines[0].Line != "c" || lines[1].Line != "d" || lines[2].Line != "e" {
		t.Errorf("expected [c, d, e], got [%s, %s, %s]", lines[0].Line, lines[1].Line, lines[2].Line)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	buf := NewRingBuffer(3)

	if buf.Len() != 0 {
		t.Errorf("expected length 0, got %d", buf.Len())
	}

	lines := buf.GetAll()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestLogManager_Writer(t *testing.T) {
	mgr := NewLogManager(10)

	writer := mgr.Writer("api")
	writer.Write([]byte("line 1\n"))
	writer.Write([]byte("line 2\n"))

	lines := mgr.GetLines([]string{"api"}, 10)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Line != "line 1" {
		t.Errorf("expected 'line 1', got %q", lines[0].Line)
	}
	if lines[0].Service != "api" {
		t.Errorf("expected service 'api', got %q", lines[0].Service)
	}
}

func TestLogManager_WriterPartialLine(t *testing.T) {
	mgr := NewLogManager(10)

	writer := mgr.Writer("api")
	writer.Write([]byte("part"))
	writer.Write([]byte("ial\n"))

	lines := mgr.GetLines([]string{"api"}, 10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Line != "partial" {
		t.Errorf("expected 'partial', got %q", lines[0].Line)
	}
}

func TestLogManager_MultipleServices(t *testing.T) {
	mgr := NewLogManager(10)

	apiWriter := mgr.Writer("api")
	dbWriter := mgr.Writer("db")

	apiWriter.Write([]byte("api log\n"))
	dbWriter.Write([]byte("db log\n"))

	apiLines := mgr.GetLines([]string{"api"}, 10)
	if len(apiLines) != 1 || apiLines[0].Line != "api log" {
		t.Errorf("expected api log, got %v", apiLines)
	}

	dbLines := mgr.GetLines([]string{"db"}, 10)
	if len(dbLines) != 1 || dbLines[0].Line != "db log" {
		t.Errorf("expected db log, got %v", dbLines)
	}

	allLines := mgr.GetLines([]string{"api", "db"}, 10)
	if len(allLines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(allLines))
	}
}

func TestLogManager_Subscribe(t *testing.T) {
	mgr := NewLogManager(10)

	ch := mgr.Subscribe([]string{"api"})

	// Write in a goroutine to avoid blocking
	go func() {
		writer := mgr.Writer("api")
		writer.Write([]byte("new log\n"))
	}()

	// Wait for log
	select {
	case line := <-ch:
		if line.Line != "new log" {
			t.Errorf("expected 'new log', got %q", line.Line)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for log")
	}

	mgr.Unsubscribe(ch)
}

func TestLogManager_SubscribeFiltersByService(t *testing.T) {
	mgr := NewLogManager(10)

	ch := mgr.Subscribe([]string{"api"})

	go func() {
		apiWriter := mgr.Writer("api")
		dbWriter := mgr.Writer("db")
		dbWriter.Write([]byte("db log\n"))
		apiWriter.Write([]byte("api log\n"))
	}()

	select {
	case line := <-ch:
		if line.Service != "api" {
			t.Errorf("expected service 'api', got %q", line.Service)
		}
		if line.Line != "api log" {
			t.Errorf("expected 'api log', got %q", line.Line)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for log")
	}

	// Ensure no db log was received
	select {
	case line := <-ch:
		t.Errorf("unexpected log from service %q: %q", line.Service, line.Line)
	case <-time.After(50 * time.Millisecond):
		// Expected: no more messages
	}

	mgr.Unsubscribe(ch)
}

func TestLogManager_SubscribeAll(t *testing.T) {
	mgr := NewLogManager(10)

	ch := mgr.Subscribe(nil)

	go func() {
		mgr.Writer("api").Write([]byte("api log\n"))
		mgr.Writer("db").Write([]byte("db log\n"))
	}()

	received := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case line := <-ch:
			received[line.Service] = true
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for log")
		}
	}

	if !received["api"] || !received["db"] {
		t.Errorf("expected logs from both services, got %v", received)
	}

	mgr.Unsubscribe(ch)
}

func TestLogManager_GetLinesLimit(t *testing.T) {
	mgr := NewLogManager(10)
	writer := mgr.Writer("api")

	for i := 0; i < 5; i++ {
		writer.Write([]byte("line\n"))
	}

	lines := mgr.GetLines([]string{"api"}, 3)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (limited), got %d", len(lines))
	}
}
