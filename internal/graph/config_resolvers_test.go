package graph

import (
	"context"
	"testing"
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
