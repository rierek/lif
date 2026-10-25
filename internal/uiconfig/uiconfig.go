// Package uiconfig feeds the process-wide tables of internal/ui from a
// config. internal/ui must not import pkg/config, so this is the one place
// where the config-derived values cross into it. The CLI and the TUI share it.
package uiconfig

import (
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
)

// Apply puts the theme and the type, status and priority tables of c in force.
// The theme is process wide and must be in force before anything renders.
func Apply(c *config.Config) {
	ui.SetTheme(c.GetTheme())
	feedTypeTables(c)
	feedStatusPriorityTables(c)
}

// feedTypeTables fills internal/ui's process-wide type-short and
// type-column-width tables from the merged type list.
func feedTypeTables(c *config.Config) {
	shorts := make(map[string]string)
	longest := 0
	for _, t := range c.TypeList() {
		shorts[t.Name] = c.ShortOf(t.Name)
		// Cells, not bytes: this value becomes a column width, and a type
		// named "Änderung" is 8 cells in 9 bytes.
		if w := ui.DisplayWidth(t.Name); w > longest {
			longest = w
		}
	}
	ui.SetTypeShorts(shorts)
	// One trailing space keeps the column from touching the next one, which is
	// what the old literal 10 did for "milestone" at nine characters.
	ui.SetTypeColumnWidths(3, longest+1)
}

// feedStatusPriorityTables fills internal/ui's process-wide status-short and
// priority-symbol tables from the merged status/priority lists.
func feedStatusPriorityTables(c *config.Config) {
	shorts := make(map[string]string, len(c.StatusList()))
	for _, s := range c.StatusList() {
		shorts[s.Name] = c.ShortOfStatus(s.Name)
	}
	ui.SetStatusShorts(shorts)

	symbols := make(map[string]string, len(c.PriorityList()))
	for _, p := range c.PriorityList() {
		symbols[p.Name] = p.Symbol
	}
	ui.SetPrioritySymbols(symbols)
}
