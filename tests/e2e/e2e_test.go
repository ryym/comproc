package e2e

import "testing"

func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
}
