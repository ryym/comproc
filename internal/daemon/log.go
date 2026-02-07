package daemon

import (
	"io"
	"strings"
	"sync"
	"time"
)

// LogLine represents a single log line.
type LogLine struct {
	Service   string
	Line      string
	Timestamp time.Time
	Stream    string // "stdout" or "stderr"
}

// subscriber represents a log subscription with an optional service filter.
type subscriber struct {
	ch       chan LogLine
	services map[string]bool // nil means all services
}

// LogManager manages log collection and distribution.
type LogManager struct {
	mu          sync.RWMutex
	buffers     map[string]*RingBuffer
	bufferSize  int
	subscribers map[<-chan LogLine]*subscriber
}

// NewLogManager creates a new log manager.
func NewLogManager(bufferSize int) *LogManager {
	return &LogManager{
		buffers:     make(map[string]*RingBuffer),
		bufferSize:  bufferSize,
		subscribers: make(map[<-chan LogLine]*subscriber),
	}
}

// Writer returns an io.Writer that captures output for the given service.
func (m *LogManager) Writer(service string) io.Writer {
	return &logWriter{
		mgr:     m,
		service: service,
		stream:  "stdout",
	}
}

// GetLines returns the most recent lines for the specified services.
func (m *LogManager) GetLines(services []string, count int) []LogLine {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []LogLine
	for _, svc := range services {
		if buf, ok := m.buffers[svc]; ok {
			result = append(result, buf.GetAll()...)
		}
	}

	// Sort by timestamp and return last N
	// Simple approach: already in chronological order within each service
	if len(result) > count {
		result = result[len(result)-count:]
	}

	return result
}

// Subscribe returns a channel that receives new log lines.
// If services is non-empty, only lines from those services are sent.
func (m *LogManager) Subscribe(services []string) <-chan LogLine {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub := &subscriber{
		ch: make(chan LogLine, 100),
	}
	if len(services) > 0 {
		sub.services = make(map[string]bool, len(services))
		for _, s := range services {
			sub.services[s] = true
		}
	}
	m.subscribers[sub.ch] = sub

	return sub.ch
}

// Unsubscribe removes a subscription.
func (m *LogManager) Unsubscribe(ch <-chan LogLine) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sub, ok := m.subscribers[ch]; ok {
		close(sub.ch)
		delete(m.subscribers, ch)
	}
}

// addLine adds a log line and notifies subscribers.
func (m *LogManager) addLine(line LogLine) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create buffer
	buf, ok := m.buffers[line.Service]
	if !ok {
		buf = NewRingBuffer(m.bufferSize)
		m.buffers[line.Service] = buf
	}
	buf.Add(line)

	// Notify subscribers (non-blocking)
	for _, sub := range m.subscribers {
		if sub.services != nil && !sub.services[line.Service] {
			continue
		}
		select {
		case sub.ch <- line:
		default:
			// Channel full, skip
		}
	}
}

// logWriter implements io.Writer for log capture.
type logWriter struct {
	mgr     *LogManager
	service string
	stream  string
	partial string // Incomplete line buffer
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	data := w.partial + string(p)
	lines := strings.Split(data, "\n")

	// Process complete lines
	for i := 0; i < len(lines)-1; i++ {
		if lines[i] != "" {
			w.mgr.addLine(LogLine{
				Service:   w.service,
				Line:      lines[i],
				Timestamp: time.Now(),
				Stream:    w.stream,
			})
		}
	}

	// Keep incomplete line for next write
	w.partial = lines[len(lines)-1]

	return len(p), nil
}

// RingBuffer is a fixed-size circular buffer for log lines.
type RingBuffer struct {
	mu    sync.RWMutex
	items []LogLine
	size  int
	head  int
	count int
}

// NewRingBuffer creates a new ring buffer.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		items: make([]LogLine, size),
		size:  size,
	}
}

// Add adds an item to the buffer.
func (b *RingBuffer) Add(item LogLine) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.items[b.head] = item
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

// GetAll returns all items in chronological order.
func (b *RingBuffer) GetAll() []LogLine {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]LogLine, b.count)
	start := 0
	if b.count == b.size {
		start = b.head
	}

	for i := 0; i < b.count; i++ {
		idx := (start + i) % b.size
		result[i] = b.items[idx]
	}

	return result
}

// Len returns the number of items in the buffer.
func (b *RingBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}
