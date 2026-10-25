package tui

// box_form_scroll_layout_test.go — box-form scroll bounds and pgdn in
// layoutDetail, whose single full-width pane is a different accW budget than
// the split's (box_form_scroll_test.go covers the split).

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/pkg/item"
)

// boxFormWrapSensitiveBeans is boxFormLongBodyBeans plus ONE long prose
// paragraph, so the total line count differs between the split's accW and
// layoutDetail's: a bounds test against the wrong geometry cannot pass
// vacuously (an 80-item list renders one line per item at any width).
func boxFormWrapSensitiveBeans() []*item.Item {
	var body strings.Builder
	body.WriteString(strings.TrimSpace(strings.Repeat("wrapword ", 60)) + "\n\n")
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&body, "- BODYLINE%02d\n", i)
	}
	beans := fixtureBeans()
	for i := range beans {
		if beans[i].ID == "tk-1" || beans[i].ID == "tk-2" {
			beans[i].Body = body.String()
		}
	}
	return beans
}

// layoutDetailBoxFormModel is a 100x30 model on tk-2, in layoutDetail.
func layoutDetailBoxFormModel(t *testing.T, beans []*item.Item) model {
	t.Helper()
	m := fixtureModel(t, beans)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = focusBean(m, "tk-2")
	m = m.setLayout(layoutDetail)
	return m
}

func TestBoxFormScrollBoundsUseLayoutDetailWidth(t *testing.T) {
	m := layoutDetailBoxFormModel(t, boxFormWrapSensitiveBeans())
	b := m.focusedBean()
	full, _ := boxFormScrollBounds(m, b)

	split := m
	split.layout = layoutFilterDetail
	narrow, _ := boxFormScrollBounds(split, b)

	if full >= narrow {
		t.Fatalf("total lines: layoutDetail = %d, split = %d; the wider pane must wrap to fewer lines", full, narrow)
	}
}

func TestBoxFormPageDownInLayoutDetailReachesBodyEnd(t *testing.T) {
	m := layoutDetailBoxFormModel(t, boxFormLongBodyBeans())
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)

	for range total {
		m = step(t, m, keyMsg(tea.KeyPgDown))
	}
	if want := total - height; m.boxFormScroll != want {
		t.Fatalf("boxFormScroll after paging past the end = %d, want the ceiling %d", m.boxFormScroll, want)
	}
	if !strings.Contains(ansi.Strip(m.View()), "BODYLINE80") {
		t.Fatal("paging to the ceiling must reveal the Body's last line in layoutDetail")
	}
}
