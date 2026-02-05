package daemon

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		failures int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second}, // capped at maxBackoff
		{7, 30 * time.Second},
		{10, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := calculateBackoff(tt.failures)
			if got != tt.expected {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.failures, got, tt.expected)
			}
		})
	}
}
