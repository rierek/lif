package graph

import (
	"context"
	"slices"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

func TestQueryTheme(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	got, err := resolver.Query().Theme(ctx)
	if err != nil {
		t.Fatalf("Theme() error = %v", err)
	}
	if got != "mocha" {
		t.Errorf("Theme() = %q, want %q while display.theme is unset", got, "mocha")
	}

	core.Config().Display.Theme = "frappe"
	got, err = resolver.Query().Theme(ctx)
	if err != nil {
		t.Fatalf("Theme() error = %v", err)
	}
	if got != "frappe" {
		t.Errorf("Theme() = %q, want %q from display.theme", got, "frappe")
	}
}

// The web UI takes its status, type and priority lists and their colours from
// these queries: config order, custom entries appended, colours normalised.
func TestQueryConfigValues(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()
	cfg := core.Config()
	cfg.Statuses = []config.StatusOverride{
		{Name: "todo", Color: "#AABBCC"},
		{Name: "to-review", Color: "Sky"},
		{Name: "odd", Color: "nope"},
	}
	cfg.Types = []config.TypeOverride{{Name: "bug", Color: "purple"}}
	cfg.Priorities = []config.PriorityOverride{{Name: "urgent", Color: "#F00"}}

	flat := func(vals []*model.ConfigValue, err error) []string {
		t.Helper()
		if err != nil {
			t.Fatalf("query error = %v", err)
		}
		out := make([]string, len(vals))
		for i, v := range vals {
			out[i] = v.Name + ":" + v.Color
		}
		return out
	}

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{"statuses", flat(resolver.Query().Statuses(ctx)), []string{
			"in-progress:peach", "todo:#aabbcc", "draft:overlay2", "completed:overlay1",
			"scrapped:surface2", "to-review:sky", "odd:",
		}},
		{"types", flat(resolver.Query().Types(ctx)), []string{
			"milestone:mauve", "epic:blue", "feature:sapphire", "bug:mauve", "task:",
		}},
		{"priorities", flat(resolver.Query().Priorities(ctx)), []string{
			"critical:red", "high:yellow", "normal:", "low:overlay0", "deferred:overlay0", "urgent:#f00",
		}},
	}
	for _, tt := range tests {
		if !slices.Equal(tt.got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}
