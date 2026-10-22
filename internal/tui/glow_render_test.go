package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestGlowRenderEmptyBodyReturnsEmpty guards glowRender's empty-input
// short-circuit (port devd editor.go:102-126).
func TestGlowRenderEmptyBodyReturnsEmpty(t *testing.T) {
	if got := glowRender("", 80); got != "" {
		t.Errorf("glowRender(\"\", 80) = %q, want empty string", got)
	}
}

// TestGlowRenderAsciiProfileProducesNoEscapeSequences guards Golden-
// Determinism for future Detail goldens: under the Ascii color profile,
// glowRender's glamour output must never contain a raw ESC[ sequence (notty
// style).
func TestGlowRenderAsciiProfileProducesNoEscapeSequences(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	out := glowRender("# Heading\n\nSome **bold** body text.", 80)
	if strings.Contains(out, "\x1b[") {
		t.Errorf("glowRender under Ascii profile must contain no escape sequences, got: %q", out)
	}
	if !strings.Contains(out, "Heading") {
		t.Errorf("glowRender output missing rendered content: %q", out)
	}
}
