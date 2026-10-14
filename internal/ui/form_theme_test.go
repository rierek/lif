package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// colourSeq matches any SGR colour sequence: 38/48 TrueColor or 256-colour,
// and the 16-colour codes 30-37, 40-47, 90-97, 100-107.
var colourSeq = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// renderThemedForm renders one form per field kind so every themed part
// shows: title, description, select with filter line, multi-select, input
// with validation error, confirm buttons, and the help line.
func renderThemedForm(t *testing.T, th Theme) string {
	t.Helper()
	opts := huh.NewOptions("alpha", "beta")
	var s string
	var ms []string
	fields := []huh.Field{
		huh.NewSelect[string]().Title("Pick").Description("desc").Options(opts...).Filtering(true).Value(&s),
		huh.NewMultiSelect[string]().Title("Many").Options(opts...).Value(&ms),
		huh.NewInput().Title("Name").Placeholder("hint").Validate(func(string) error { return errTest }),
		huh.NewConfirm().Title("Sure?"),
	}
	var out string
	for _, fld := range fields {
		f := huh.NewForm(huh.NewGroup(fld)).
			WithTheme(FormTheme(FlavourFormPalette(th))).WithWidth(60).WithHeight(30)
		f.Init()
		m, _ := f.Update(tea.KeyMsg{Type: tea.KeyEnter}) // fires the validation error
		f = m.(*huh.Form)
		out += f.View() + f.Help().ShortHelpView(f.KeyBinds())
	}
	return out
}

type testErr string

func (e testErr) Error() string { return string(e) }

const errTest = testErr("bad")

func TestFormThemeOnlyFlavourTones(t *testing.T) {
	withTrueColor(t)
	for _, name := range []string{"mocha", "latte"} {
		th, _ := ThemeByName(name)
		out := renderThemedForm(t, th)
		found := 0
		for _, m := range colourSeq.FindAllStringSubmatch(out, -1) {
			p := strings.TrimPrefix(strings.TrimPrefix(m[1], "1;"), "7;") // bold/reverse may precede the colour
			switch {
			case strings.HasPrefix(p, "38;2;"), strings.HasPrefix(p, "48;2;"):
				found++
				if !isTone(th, p[5:]) {
					t.Errorf("%s: colour %q is no tone of the flavour", name, m[1])
				}
			case p == "" || p == "0" || p == "1" || p == "7" || p == "22" || p == "4" || p == "24":
				// reset and attribute codes carry no colour
			default:
				t.Errorf("%s: non-TrueColor or unknown sequence %q", name, m[1])
			}
		}
		if found == 0 {
			t.Fatalf("%s: no colour sequence rendered, test is vacuous", name)
		}
	}
}

func TestFormThemeFlavoursDiffer(t *testing.T) {
	withTrueColor(t)
	mocha, _ := ThemeByName("mocha")
	latte, _ := ThemeByName("latte")
	if renderThemedForm(t, mocha) == renderThemedForm(t, latte) {
		t.Fatal("mocha and latte render the same form output")
	}
}

func TestFlavourFormPaletteRolesAreTones(t *testing.T) {
	for name, th := range Themes() {
		p := FlavourFormPalette(th)
		for role, c := range map[string]string{"Accent": string(p.Accent), "Focus": string(p.Focus), "Background": string(p.Background),
			"Error": string(p.Error), "Text": string(p.Text), "Muted": string(p.Muted), "Surface": string(p.Surface)} {
			found := false
			for _, hex := range th.Colors {
				found = found || hex == c
			}
			if !found {
				t.Errorf("%s %s = %q is no tone", name, role, c)
			}
		}
	}
}

// isTone reports whether the "r;g;b" triple is a tone of th. termenv rounds
// its hex conversion, so each channel may deviate by one.
func isTone(th Theme, rgb string) bool {
	var r, g, b int
	if _, err := fmt.Sscanf(rgb, "%d;%d;%d", &r, &g, &b); err != nil {
		return false
	}
	near := func(a, b int) bool { return a-b <= 1 && b-a <= 1 }
	for _, hex := range th.Colors {
		var hr, hg, hb int
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &hr, &hg, &hb); err != nil {
			panic(err)
		}
		if near(r, hr) && near(g, hg) && near(b, hb) {
			return true
		}
	}
	return false
}
