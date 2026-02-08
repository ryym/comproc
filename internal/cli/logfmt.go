package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// ANSI color codes for service name coloring.
// Colors chosen to be distinct and readable on both light and dark terminals.
var serviceColors = []string{
	"\033[36m", // Cyan
	"\033[33m", // Yellow
	"\033[32m", // Green
	"\033[34m", // Blue
	"\033[35m", // Magenta
	"\033[91m", // Bright Red
	"\033[96m", // Bright Cyan
	"\033[93m", // Bright Yellow
}

const colorReset = "\033[0m"

// LogFormatter formats log lines with aligned service name prefixes and colors.
type LogFormatter struct {
	mu           sync.Mutex
	maxNameLen   int
	out          io.Writer
	colorEnabled bool
	serviceColor map[string]string
	nextColor    int
}

// NewLogFormatter creates a new LogFormatter with the given service names.
func NewLogFormatter(out io.Writer, serviceNames []string) *LogFormatter {
	maxLen := 0
	for _, name := range serviceNames {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	f := &LogFormatter{
		maxNameLen:   maxLen,
		out:          out,
		colorEnabled: true,
		serviceColor: make(map[string]string),
	}

	// Pre-assign colors to known services
	for _, name := range serviceNames {
		f.assignColor(name)
	}

	return f
}

// SetColorEnabled enables or disables color output.
func (f *LogFormatter) SetColorEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.colorEnabled = enabled
}

// assignColor assigns a color to a service (must be called with lock held).
func (f *LogFormatter) assignColor(service string) string {
	if color, ok := f.serviceColor[service]; ok {
		return color
	}
	color := serviceColors[f.nextColor%len(serviceColors)]
	f.serviceColor[service] = color
	f.nextColor++
	return color
}

// PrintLine prints a log line with aligned and colored prefix.
func (f *LogFormatter) PrintLine(service, line string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Update max length if we see a longer service name
	if len(service) > f.maxNameLen {
		f.maxNameLen = len(service)
	}

	// Get or assign color for this service
	color := f.assignColor(service)

	// Pad service name to align the separator
	padded := service + strings.Repeat(" ", f.maxNameLen-len(service))

	if f.colorEnabled {
		fmt.Fprintf(f.out, "%s%s |%s %s\n", color, padded, colorReset, line)
	} else {
		fmt.Fprintf(f.out, "%s | %s\n", padded, line)
	}
}
