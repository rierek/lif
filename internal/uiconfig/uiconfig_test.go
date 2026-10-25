package uiconfig

import (
	"testing"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
)

// TestApplySetsTypeColumnWidthFromLongestTypeName drives the actual
// "follows the longest name" computation (longest+1), which internal/ui's own
// setter/getter test cannot reach: internal/ui must not import pkg/config, so
// the config-derived fixture has to live here.
// "extraordinarily-long-type-name" is 30 characters, one longer than any
// DefaultTypes entry, so it must drive the merged type list's longest name.
func TestApplySetsTypeColumnWidthFromLongestTypeName(t *testing.T) {
	t.Cleanup(func() {
		ui.SetTheme("mocha")
		ui.SetTypeColumnWidths(3, 10)
		// Apply also mutates internal/ui's process-wide type-shorts table;
		// SetTypeShorts(nil) resets it to the built-in baseline, the same
		// restore the styles_test.go tests in internal/ui use.
		ui.SetTypeShorts(nil)
		ui.SetStatusShorts(nil)
		ui.SetPrioritySymbols(nil)
	})

	const longName = "extraordinarily-long-type-name"
	c := &config.Config{Types: []config.TypeOverride{{Name: longName}}}

	Apply(c)

	want := len(longName) + 1
	if got := ui.FullTypeColumnWidth(); got != want {
		t.Errorf("FullTypeColumnWidth() = %d, want %d (len(%q)+1)", got, want, longName)
	}
}

// TestApplyReflectsConfiguredStatusAndPriorityOverrides pins the beans-krej
// fix end to end: a status or priority added or overridden through .beans.yml
// must actually reach internal/ui's process-wide tables, not just the merge
// helpers in pkg/config. Without Apply wiring this in,
// ShortStatus/GetPrioritySymbol would keep rendering "?" / nothing for
// anything not in the hardcoded built-in tables forever.
func TestApplyReflectsConfiguredStatusAndPriorityOverrides(t *testing.T) {
	t.Cleanup(func() {
		ui.SetTheme("mocha")
		ui.SetTypeColumnWidths(3, 10)
		ui.SetTypeShorts(nil)
		ui.SetStatusShorts(nil)
		ui.SetPrioritySymbols(nil)
	})

	c := &config.Config{
		Statuses:   []config.StatusOverride{{Name: "blocked", Short: "X"}},
		Priorities: []config.PriorityOverride{{Name: "urgent", Symbol: "@"}},
	}

	Apply(c)

	if got := ui.ShortStatus("blocked"); got != "X" {
		t.Errorf(`ShortStatus("blocked") = %q, want "X"`, got)
	}
	// A built-in status untouched by the override must still resolve from
	// the merged defaults, not disappear because Statuses was non-empty.
	if got := ui.ShortStatus("todo"); got != "T" {
		t.Errorf(`ShortStatus("todo") = %q, want "T"`, got)
	}
	if got := ui.GetPrioritySymbol("urgent"); got != "@" {
		t.Errorf(`GetPrioritySymbol("urgent") = %q, want "@"`, got)
	}
}
