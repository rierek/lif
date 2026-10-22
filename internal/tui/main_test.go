package tui

import (
	"os"
	"testing"
)

// The box-form detail is the product default; the pre-existing suite asserts
// the classic accordion, so pin it here. Tests for box-form opt in via
func TestMain(m *testing.M) {
	// The TUI drives the lif CLI: a developer's global hooks must not fire in tests.
	os.Setenv("LIF_NO_HOOKS", "1")
	// Colours and glyphs come from the config: start every run with the
	// built-in defaults applied, as tests that reset via applyRepoConfig(nil) do.
	applyRepoConfig(nil)
	os.Exit(m.Run())
}
