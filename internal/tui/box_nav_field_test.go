package tui

// box_nav_field_test.go — TDD coverage for the box-form FIELD CURSOR (bean
// bt-1o4g, epic bt-vy1q, PO-Nebenbefund N8 "keyboard-first"): while
// the Detail pane is focused (tab), the arrow keys must
// step between detailBoxForm's boxes (box_detail_form.go) instead of doing
// nothing, the cursored box must render with dropdownBox/panelBox's
// pre-existing focused=true Mauve frame, and enter must open the SAME editor
// the box's hotkey (and a mouse click, mouse.go) already opens.
//
// Mirrors box_form_scroll_test.go's fixture/drive pattern (boxFormScrollModel,
// step()/keyMsg()) -- the scroll interplay is part of THIS bean's contract
// ("a focused field that scrolls out of view would be a bug"), so the two
// suites deliberately share the long-Body fixture.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// boxFormFieldModel is boxFormScrollModel with the Detail pane focused --
// the state a real `tab` press produces (update.go's keyTree Focus branch),
// set directly here the same way box_form_scroll_test.go's own tests do.
func boxFormFieldModel(t *testing.T) model {
	t.Helper()
	m := boxFormScrollModel(t)
	m.detailFocus = true
	return m
}

// fieldIdx resolves a boxFormFieldOrder name to its index so the tests below
// read as field NAMES rather than magic numbers (and fail loudly if the order
// table is ever renamed out from under them).
func fieldIdx(t *testing.T, name string) int {
	t.Helper()
	for i, f := range boxFormFieldOrder {
		if f.name == name {
			return i
		}
	}
	t.Fatalf("boxFormFieldOrder has no field named %q", name)
	return -1
}

// --- Order table / geometry single source ---

// TestBoxFormFieldOrderMatchesClickGeometry guards the bean's own
// architecture constraint: the field ORDER must be derived from the same
// hit-test geometry the mouse already uses (detailBoxFormClickRow/gridColAt,
// mouse.go), never a second independent list that can drift. Every field's
// (row, col) must round-trip back to itself through boxFormFieldAt.
func TestBoxFormFieldOrderMatchesClickGeometry(t *testing.T) {
	const accW = 90
	for i, f := range boxFormFieldOrder {
		if f.row > boxFormRowScalarsB {
			continue // panels: unbounded height, no per-row hit-test
		}
		// Middle line of the field's 3-line box, middle column of its cell.
		widths := gridColWidths(boxFormRowCols(f.row), accW)
		col := 0
		for c := 0; c < f.col; c++ {
			col += widths[c] + detailBoxFormGap
		}
		col += widths[f.col] / 2
		row := f.row*3 + 1

		if got := boxFormFieldAt(row, col, accW); got != i {
			t.Fatalf("boxFormFieldAt(row=%d, col=%d) = %d, want %d (%s)", row, col, got, i, f.name)
		}
	}
}

// --- Rendering: the cursored box gets the Mauve frame ---

// TestDetailBoxFormRendersCursoredFieldFocused guards Ziel 3: the cursored
// field renders through dropdownBox/panelBox's pre-existing focused=true
// branch (Mauve frame) -- and NOTHING else changes, i.e. exactly the cursored
// box's own lines differ from the cursor-less render.
func TestDetailBoxFormRendersCursoredFieldFocused(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	b := detailBoxFormFixture()
	idx := detailBoxFormIndex()
	const w = 90

	plain := detailBoxForm(idx, b, w, -1)
	for i, f := range boxFormFieldOrder {
		got := detailBoxForm(idx, b, w, i)
		if got == plain {
			t.Fatalf("detailBoxForm(cursor=%d, %s) is byte-identical to the cursor-less render -- the focused frame is not wired through", i, f.name)
		}

		blocks := boxFormBlocks(idx, b, w, -1)
		start, height := boxFormFieldSpan(blocks, i)
		gotLines := strings.Split(got, "\n")
		plainLines := strings.Split(plain, "\n")
		if len(gotLines) != len(plainLines) {
			t.Fatalf("cursor=%d (%s) changed the form's line count: %d vs %d", i, f.name, len(gotLines), len(plainLines))
		}
		for ln := range gotLines {
			inside := ln >= start && ln < start+height
			if !inside && gotLines[ln] != plainLines[ln] {
				t.Fatalf("cursor=%d (%s) changed line %d, which lies OUTSIDE the field's own span [%d,%d)", i, f.name, ln, start, start+height)
			}
		}
	}
}

// TestDetailBoxFormNoCursorIsUnchanged guards the inert default: cursor -1
// must render exactly what the pre-cursor code produced, i.e. every box with
// focused=false -- the flag-on goldens (browse_boxform.golden) depend on it.
func TestDetailBoxFormNoCursorIsUnchanged(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	b := detailBoxFormFixture()
	idx := detailBoxFormIndex()
	// bean bt-oox1 (#1): the Title box's label now carries the bean ID
	// alongside the word "Title". The invariant under test is the FOCUS
	// state (cursor -1 => every box unfocused), not the label text.
	want := dropdownBox("Title · "+b.ID, b.Title, "e", 90, false)
	got := detailBoxForm(idx, b, 90, -1)
	if !strings.HasPrefix(got, want) {
		t.Fatal("detailBoxForm(cursor=-1) must start with the UNFOCUSED Title box (flag-on goldens depend on it)")
	}
}

// TestBoxFormTabRendersFieldCursorInRealView is the PO's own acceptance
// sentence, driven end to end through the REAL pipeline: pressing tab (focus
// into the Detail pane) must make a field VISIBLY focused in m.View() -- a
// Mauve frame that was not on screen a keypress earlier.
func TestBoxFormTabRendersFieldCursorInRealView(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := boxFormScrollModel(t) // Tree-focused
	b := m.focusedBean()
	accW, _ := boxFormPaneMetrics(m, b)

	// The Title box (first field, the cursor's default) in both frame colors:
	// its value line carries the Mauve/Overlay border glyphs, so it is the
	// discriminator -- other Mauve on screen (pane focus borders, filter
	// chips) can never match this exact rendered line.
	focusedLine := strings.Split(dropdownBox("Title", b.Title, "e", accW, true), "\n")[1]
	plainLine := strings.Split(dropdownBox("Title", b.Title, "e", accW, false), "\n")[1]

	before := m.View()
	if !strings.Contains(before, plainLine) {
		t.Fatal("setup: the UNFOCUSED Title box is not on screen while the Tree has focus")
	}
	if strings.Contains(before, focusedLine) {
		t.Fatal("setup: the Title box already renders focused while the TREE has focus -- the cursor must only show inside the Detail pane")
	}

	m = tabIntoDetail(t, m)
	if !m.detailFocus {
		t.Fatal("tab did not move focus into the Detail pane")
	}
	after := m.View()
	if !strings.Contains(after, focusedLine) {
		t.Fatal("after tab the Title box does not render with the Mauve focus frame -- the field cursor is invisible (PO-Nebenbefund N8)")
	}
	if strings.Contains(after, plainLine) {
		t.Fatal("after tab the Title box still renders UNFOCUSED -- the field cursor did not take effect")
	}
}

// --- Keyboard navigation ---

// TestBoxFormGridMoveWalksTheLayout guards the GRID geometry of the field
// table (boxFormMoveField): up/down between rows keeping the column where the
// target row has one, left/right within a grid row, edges are no-ops.
//
// bean bt-8d35 (PO-Entscheidung 3): this is no longer driven by the ARROW
// KEYS -- those went back to plain viewport scrolling and tab/shift+tab took
// over the field cursor (linear order + wrap, see TestBoxFormTabWalksFields
// WithWrap below). The grid mover itself is deliberately NOT torn out
// ("kein Voll-Rueckbau", bt-8d35's own wording) and stays covered here at the
// boxFormNav level, one rung below the keymap.
func TestBoxFormGridMoveWalksTheLayout(t *testing.T) {
	m := boxFormFieldModel(t)
	b := m.focusedBean()

	steps := []struct {
		dir  string
		want string
	}{
		{"down", "status"}, // Title -> row A, col 0
		{"right", "type"},  // within row A
		{"right", "priority"},
		{"right", "priority"}, // right edge: no-op
		{"down", "tags"},      // row B has only 2 cols -> col clamps to 1
		{"left", "parent"},
		{"left", "parent"}, // left edge: no-op
		{"up", "status"},   // back into row A at col 0
		{"up", "title"},
		{"up", "title"}, // top edge: no-op
	}
	for i, s := range steps {
		m = m.boxFormNav(b, s.dir)
		if got := boxFormFieldOrder[m.boxFormCursor].name; got != s.want {
			t.Fatalf("step %d (%s): cursor on %q, want %q", i, s.dir, got, s.want)
		}
	}
}

// --- up/down own the field cursor inside the Detail region ---

// TestBoxFormDownWalksFieldsWithWrap: down steps through detailBoxForm's
// fields in RENDER order and WRAPS at the end instead of falling back into
// the Tree ("sonst verliert man den Fokus versehentlich beim
// Durchsteppen").
func TestBoxFormDownWalksFieldsWithWrap(t *testing.T) {
	m := boxFormFieldModel(t)

	want := make([]string, 0, len(boxFormFieldOrder))
	for _, f := range boxFormFieldOrder[1:] {
		want = append(want, f.name)
	}
	want = append(want, boxFormFieldOrder[0].name) // wrap back to Title

	for i, w := range want {
		m = step(t, m, keyMsg(tea.KeyDown))
		if got := boxFormFieldOrder[m.boxFormCursor].name; got != w {
			t.Fatalf("down #%d: cursor on %q, want %q", i+1, got, w)
		}
		if !m.detailFocus {
			t.Fatalf("down #%d left the Detail region -- down must move WITHIN it", i+1)
		}
	}
}

// TestBoxFormUpWalksFieldsBackwardsWithWrap is the reverse step: from the
// FIRST field up wraps to the last rather than exiting the region (that is
// esc's job).
func TestBoxFormUpWalksFieldsBackwardsWithWrap(t *testing.T) {
	m := boxFormFieldModel(t)

	m = step(t, m, keyMsg(tea.KeyUp))
	if got, want := boxFormFieldOrder[m.boxFormCursor].name, boxFormFieldOrder[len(boxFormFieldOrder)-1].name; got != want {
		t.Fatalf("up on the first field: cursor on %q, want the LAST field %q (wrap)", got, want)
	}
	if !m.detailFocus {
		t.Fatal("up left the Detail region -- only esc may do that")
	}

	m = step(t, m, keyMsg(tea.KeyUp))
	if got, want := boxFormFieldOrder[m.boxFormCursor].name, boxFormFieldOrder[len(boxFormFieldOrder)-2].name; got != want {
		t.Fatalf("second up: cursor on %q, want %q", got, want)
	}
}

// TestBoxFormEscLeavesTheDetailRegion pins the other half of bt-8d35's rule
// -- "esc VERLAESST die Region" -- now that shift+tab no longer does.
func TestBoxFormEscLeavesTheDetailRegion(t *testing.T) {
	m := boxFormFieldModel(t)
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.detailFocus {
		t.Fatal("esc did not leave the Detail region")
	}
}

// TestBoxFormTabFromTreeEntersAtTheFirstField guards Flow #2's entry step:
// tab out of the Tree focuses the Detail region AT TITLE, even when a stale
// cursor from a previous visit to the same bean is still on the model.
func TestBoxFormTabFromTreeEntersAtTheFirstField(t *testing.T) {
	m := boxFormScrollModel(t) // Tree-focused
	b := m.focusedBean()
	m.boxFormCursor, m.boxFormCursorBean = fieldIdx(t, "tags"), b.ID

	m = tabIntoDetail(t, m)
	if !m.detailFocus {
		t.Fatal("tab did not focus the Detail region")
	}
	if got := boxFormEffectiveCursor(m, b); got != 0 {
		t.Fatalf("tab-in landed on field %q, want the first field %q", boxFormFieldOrder[got].name, boxFormFieldOrder[0].name)
	}
}

// TestBoxFormDownWalksFieldsInTheReadyToo is bt-8d35's "Reichweite" check:
// tab is a GLOBAL binding, so the Fokus-Modell must land identically in every
// view that renders the box form -- the Ready shares the very same Detail
// pane and routes through the same keyDetailFocus (handleKey's detailFocus
// dispatch), so this pins that it really does, rather than only Browse.
func TestBoxFormDownWalksFieldsInTheReadyToo(t *testing.T) {
	m := readyBeansModel(t)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.view = viewReady

	m = tabIntoDetail(t, m) // into the Detail region
	if !m.detailFocus {
		t.Fatal("setup: tab did not focus the Ready's Detail pane")
	}
	if got := boxFormEffectiveCursor(m, m.focusedBean()); got != 0 {
		t.Fatalf("tab-in landed on field %q, want Title", boxFormFieldOrder[got].name)
	}

	m = step(t, m, keyMsg(tea.KeyDown))
	if got := boxFormFieldOrder[m.boxFormCursor].name; got != boxFormFieldOrder[1].name {
		t.Fatalf("second down in the Ready: cursor on %q, want %q", got, boxFormFieldOrder[1].name)
	}
	if !m.detailFocus {
		t.Fatal("tab swapped the Ready's panes instead of moving within the Detail region")
	}
}

// --- Scroll interplay (the bean's explicit "must not scroll out of view") ---

// TestBoxFormCursorStaysVisibleWhileNavigating is the headline interplay
// criterion: after EVERY tab press the cursored field's own line span must
// intersect the visible window [boxFormScroll, boxFormScroll+height) -- a
// focused field that scrolled out of the pane would be a bug.
//
// bean bt-8d35: unchanged in substance, driven by tab instead of down (the
// Scroll-Mitnahme moved with the field cursor onto tab, "Springt tab auf ein
// Feld ausserhalb des sichtbaren Bereichs, muss der Viewport nachziehen").
func TestBoxFormCursorStaysVisibleWhileNavigating(t *testing.T) {
	m := boxFormFieldModel(t)
	b := m.focusedBean()
	requireOverflow(t, m, b)

	accW, height := boxFormPaneMetrics(m, b)
	blocks := boxFormBlocks(m.idx, b, accW, -1)

	for i := 0; i < 300; i++ {
		m = step(t, m, keyMsg(tea.KeyDown))
		off := boxFormEffectiveScroll(m, b)
		start, h := boxFormFieldSpan(blocks, m.boxFormCursor)
		if start >= off+height || start+h <= off {
			t.Fatalf("tab #%d: field %q spans [%d,%d) but the visible window is [%d,%d) -- the cursor scrolled out of view",
				i, boxFormFieldOrder[m.boxFormCursor].name, start, start+h, off, off+height)
		}
	}
}

// --- enter opens the field's editor (the SAME path a click takes) ---

// TestBoxFormEnterOpensFieldEditor guards Ziel 4 for every field kind: enter
// on the cursored box fires the EXACT action mouseBoxFormDetailClick fires
// for the same box (activateBoxFormTarget, mouse.go) -- one dispatch, no
// parallel path.
func TestBoxFormEnterOpensFieldEditor(t *testing.T) {
	cases := []struct {
		field string
		check func(t *testing.T, m model, beanID string)
	}{
		{"status", func(t *testing.T, m model, _ string) {
			if m.overlay != overlayValueMenu || !reflect.DeepEqual(m.menuItems, buildValueMenuItems("status")) {
				t.Fatalf("enter on Status: overlay=%v, want the status value menu", m.overlay)
			}
		}},
		{"type", func(t *testing.T, m model, _ string) {
			if m.overlay != overlayValueMenu || !reflect.DeepEqual(m.menuItems, buildValueMenuItems("type")) {
				t.Fatalf("enter on Type: overlay=%v, want the type value menu", m.overlay)
			}
		}},
		{"priority", func(t *testing.T, m model, _ string) {
			if m.overlay != overlayValueMenu || !reflect.DeepEqual(m.menuItems, buildValueMenuItems("priority")) {
				t.Fatalf("enter on Priority: overlay=%v, want the priority value menu", m.overlay)
			}
		}},
		{"parent", func(t *testing.T, m model, _ string) {
			if m.overlay != overlayParentPicker {
				t.Fatalf("enter on Parent: overlay=%v, want overlayParentPicker", m.overlay)
			}
		}},
		{"tags", func(t *testing.T, m model, _ string) {
			if m.overlay != overlayTagPicker {
				t.Fatalf("enter on Tags: overlay=%v, want overlayTagPicker", m.overlay)
			}
		}},
		{"title", func(t *testing.T, m model, id string) {
			if m.editorTarget != id {
				t.Fatalf("enter on Title: editorTarget=%q, want %q ($EDITOR on the whole bean)", m.editorTarget, id)
			}
		}},
		{"body", func(t *testing.T, m model, id string) {
			if m.editorTarget != id {
				t.Fatalf("enter on Body: editorTarget=%q, want %q", m.editorTarget, id)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			m := boxFormFieldModel(t)
			b := m.focusedBean()
			m.boxFormCursor = fieldIdx(t, tc.field)
			m.boxFormCursorBean = b.ID

			m = step(t, m, keyMsg(tea.KeyEnter))
			tc.check(t, m, b.ID)
		})
	}
}

// --- Reset on bean change ---

// TestBoxFormCursorResetsWhenSelectedBeanChanges guards "Cursor resettet bei
// Bean-Wechsel": a cursor recorded for one bean must never carry over into a
// different bean's render.
func TestBoxFormCursorResetsWhenSelectedBeanChanges(t *testing.T) {
	m := boxFormFieldModel(t)
	b := m.focusedBean()

	m.boxFormCursor = fieldIdx(t, "tags")
	m.boxFormCursorBean = b.ID

	m = focusBean(m, "tk-1")
	other := m.focusedBean()
	if other == nil || other.ID == b.ID {
		t.Fatalf("setup: expected a different focused bean, got %v", other)
	}
	if got := boxFormEffectiveCursor(m, other); got != 0 {
		t.Fatalf("boxFormEffectiveCursor(new bean) = %d, want 0 (stale cursor must not leak)", got)
	}
}

// --- Flag off: accordion mode is untouched ---

// --- Slice A (bt-f0y9, D09 revidiert): forward field->screen-rect geometry ---

// boxFormRectModel builds a fixtureModel with tk-2 focused, at
// the given terminal width (80/120, S6 grounding's own boundary widths) --
// unscrolled (fixtureBeans' tk-2 has no long Body, unlike
// boxFormLongBodyBeans), so Status/Type/Priority sit at their unscrolled
// render position and boxFormEffectiveScroll(m, b) is 0.
func boxFormRectModel(t *testing.T, width int) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: width, Height: 30})
	m = focusBean(m, "tk-2")
	return m
}

// TestBoxFormFieldRectMatchesRenderedBoxCorners is the RED-first, render-
// grounded proof for boxFormFieldRect (Slice A): for each of Status/Type/
// Priority, at BOTH 80 and 120 columns, the returned (x, y, w, h) rect's
// TOP-LEFT corner must be the box's own "╭" corner in the REAL rendered
// View() (never hand-computed against the same formula the function under
// test uses -- mouse_test.go's screenLines/ansi.Cut precedent), and a
// left-click smack in the middle of that rect must resolve back to the SAME
// field via detailBoxFormClickRow (mouse.go) -- the forward rect and the
// pre-existing inverse click resolution can never independently drift
// (Golden-Rule-Drift-Schutz, the Slice-Vorschlag's own cross-check ask).
func TestBoxFormFieldRectMatchesRenderedBoxCorners(t *testing.T) {
	cases := []struct {
		field, hotkey, target string
	}{
		{"status", "(s)", boxFormTargetStatus},
		{"type", "(t)", boxFormTargetType},
		{"priority", "(p)", boxFormTargetPriority},
	}
	widths := []int{80, 120}

	for _, width := range widths {
		width := width
		for _, tc := range cases {
			tc := tc
			t.Run(fmt.Sprintf("w=%d/%s", width, tc.field), func(t *testing.T) {
				m := boxFormRectModel(t, width)
				b := m.focusedBean()
				if b == nil {
					t.Fatalf("setup: focusedBean() is nil")
				}

				x, y, w, h, ok := boxFormFieldRect(m, b, fieldIdx(t, tc.field))
				if !ok {
					t.Fatalf("boxFormFieldRect(%s) ok=false, want true", tc.field)
				}
				if w <= 0 || h <= 0 {
					t.Fatalf("boxFormFieldRect(%s) = (%d,%d,%d,%d), want positive w/h", tc.field, x, y, w, h)
				}

				lines := screenLines(m)
				if y < 0 || y >= len(lines) {
					t.Fatalf("boxFormFieldRect(%s) y=%d out of the rendered View()'s %d lines", tc.field, y, len(lines))
				}
				corner := ansi.Cut(lines[y], x, x+1)
				if corner != "╭" {
					t.Fatalf("boxFormFieldRect(%s) top-left = %q at (x=%d,y=%d), want the box's own \"╭\" corner\nrow: %q", tc.field, corner, x, y, lines[y])
				}
				// bottomBorder row: badge lives on it, confirms h spans the
				// real 3-line dropdownBox and never drifts from boxFormFieldSpan.
				bottomRow := y + h - 1
				if bottomRow < 0 || bottomRow >= len(lines) {
					t.Fatalf("boxFormFieldRect(%s) bottom row %d out of range", tc.field, bottomRow)
				}
				if !strings.Contains(lines[bottomRow], tc.hotkey) {
					t.Fatalf("boxFormFieldRect(%s) bottom row %q does not contain badge %q", tc.field, lines[bottomRow], tc.hotkey)
				}

				// Reciprocity: a click in the middle of the returned rect must
				// resolve back through detailBoxFormClickRow to the SAME target
				// (the Slice-Vorschlag's own cross-check, forward vs. the
				// pre-existing inverse).
				msg := tea.MouseMsg{X: x + w/2, Y: y + h/2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
				target, ok := detailBoxFormClickRow(m, msg)
				if !ok || target != tc.target {
					t.Fatalf("detailBoxFormClickRow(rect center of %s) = (%q,%v), want (%q,true)", tc.field, target, ok, tc.target)
				}
			})
		}
	}
}

// TestBoxFormFieldRectNilBeanNotOk guards the defensive nil-bean/out-of-range
// contract: no garbage zero-rect, a clean ok=false.
func TestBoxFormFieldRectNilBeanNotOk(t *testing.T) {
	m := boxFormRectModel(t, 100)
	if _, _, _, _, ok := boxFormFieldRect(m, nil, fieldIdx(t, "status")); ok {
		t.Fatal("boxFormFieldRect(nil bean) ok=true, want false")
	}
	b := m.focusedBean()
	if _, _, _, _, ok := boxFormFieldRect(m, b, -1); ok {
		t.Fatal("boxFormFieldRect(field=-1) ok=true, want false")
	}
	if _, _, _, _, ok := boxFormFieldRect(m, b, len(boxFormFieldOrder)); ok {
		t.Fatal("boxFormFieldRect(field=out-of-range) ok=true, want false")
	}
}
