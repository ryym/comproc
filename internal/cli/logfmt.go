package cli

import (
	"fmt"
	"io"
	"strings"
)

// LogFormatter formats log lines with aligned service name prefixes.
type LogFormatter struct {
	maxNameLen int
	out        io.Writer
}

// NewLogFormatter creates a new LogFormatter with the given service names.
func NewLogFormatter(out io.Writer, serviceNames []string) *LogFormatter {
	maxLen := 0
	for _, name := range serviceNames {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	return &LogFormatter{
		maxNameLen: maxLen,
		out:        out,
	}
}

// PrintLine prints a log line with aligned prefix.
func (f *LogFormatter) PrintLine(service, line string) {
	// Update max length if we see a longer service name
	if len(service) > f.maxNameLen {
		f.maxNameLen = len(service)
	}

	// Pad service name to align the separator
	padded := service + strings.Repeat(" ", f.maxNameLen-len(service))
	fmt.Fprintf(f.out, "%s | %s\n", padded, line)
}
