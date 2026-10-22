package data

import (
	"os"
	"testing"
)

// TestMain keeps a developer's global hooks out of the tests; tests that
// exercise hooks reset LIF_NO_HOOKS themselves.
func TestMain(m *testing.M) {
	os.Setenv("LIF_NO_HOOKS", "1")
	os.Exit(m.Run())
}
