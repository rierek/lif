package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestStyleFormGolden pins the TUI form colours: styleForm's theme comes from
// the shared builder in internal/ui, and the render must stay byte-identical
// to the recording made when the theme was built inside this package.
// Regenerate with: go test ./internal/tui/ -run TestStyleFormGolden -update
func TestStyleFormGolden(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })

	var title, pick, confirm string
	var multi []string
	opts := huh.NewOptions("alpha", "beta", "gamma")
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Title").Description("a description").Placeholder("type here").Value(&title).
				Validate(func(s string) error {
					if s == "" {
						return os.ErrInvalid
					}
					return nil
				}),
		),
		huh.NewGroup(huh.NewSelect[string]().Title("Pick").Options(opts...).Value(&pick)),
		huh.NewGroup(huh.NewMultiSelect[string]().Title("Many").Options(opts...).Value(&multi)),
		huh.NewGroup(huh.NewConfirm().Title("Sure?").Value(new(bool))),
	)
	_ = confirm
	f = model{width: 100, height: 40}.styleForm(f)
	f.Init()

	out := f.View()
	f = driveForm(f, enterMsg()) // empty title: validation error shows
	out += "\n----\n" + f.View()
	title = "x"
	f = driveForm(f, enterMsg())
	f = driveForm(f, enterMsg())
	out += "\n----\n" + f.View()

	path := filepath.Join("testdata", "form_theme.golden")
	if *update {
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (%s): %v", path, err)
	}
	if out != string(want) {
		t.Errorf("TUI form render differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, out, want)
	}
}
