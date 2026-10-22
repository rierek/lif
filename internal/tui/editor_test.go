package tui

// editor_test.go — TDD coverage for the whole-bean $EDITOR (`e`/`ctrl+e`,
// D01, design-spec.md §15 PF-17, bean bt-z4b1, supersedes E8-B10's
// section-context-sensitive Body-only editor): editorBinary's env-cascade
// (design decision c, unchanged), prepareEditor/readEditorResult's tempfile
// round-trip (Port devd editor.go, unchanged, testable without a tea
// runtime), parseRawBean's frontmatter/body split, and applyEditorFinished's
// parse -> diff -> ONE combined Store.UpdateWhole call dispatch
// (changed-only, vanished-target guard, parse-error recovery, F2 Review-
// Runde 2: etag CAPTURED AT OPEN, never a fresh m.beanETag(id) re-read at
// submit -- see applyEditorFinished's doc-stamp, update.go).
//
// openBeanEditor's own dispatch (keyNodeAction's e/ctrl+e branch, EVERY
// entry point: Tree/Backlog/Detail, every section/field level) is covered
// end-to-end in update_test.go's TestKeyNodeActionEditorAlwaysOpensBeanEditor
// (Lessons-Learned Forward-Guard #3: cascades need every entry point tested,
// not just one) -- this file covers the machinery BELOW that dispatch.

import (
	"errors"
	"github.com/rierek/lif/pkg/item"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
	"gopkg.in/yaml.v3"
)

// rawEditorText builds a whole-bean $EDITOR raw text (the SAME shape
// Store.Raw's seed / a submitted editorFinishedMsg.content carry)
// from a rawBeanFrontmatter + explicit body, via yaml.Marshal against the
// SAME struct parseRawBean unmarshals into -- guarantees the two stay in
// sync without hand-formatting YAML in every test. Deliberately does NOT
// add a leading newline to body itself (unlike item.Item.Body's own
// CLI-JSON convention, which always carries one, verified in
// internal/data/client_mut_test.go) -- tests compare parseRawBean's
// extracted body against whatever they set on a *item.Item snapshot
// directly, so staying consistent within this file's own tests is what
// matters, not mirroring the CLI's exact byte convention (that convention
// is separately pinned by TestParseRawBeanRoundTrip below, against a REAL
// captured `beans show --raw` shape).
func rawEditorText(t *testing.T, fm rawBeanFrontmatter, body string) string {
	t.Helper()
	out, err := yaml.Marshal(fm)
	if err != nil {
		t.Fatalf("yaml.Marshal(rawBeanFrontmatter) error = %v", err)
	}
	return "---\n" + string(out) + "---\n" + body
}

// TestEditorBinaryResolvesVisualThenEditorThenVi guards design decision c's
// env cascade: $VISUAL wins over $EDITOR, both win over the "vi" fallback
// (POSIX default -- portable everywhere, unlike devd's "nvim" assumption).
// Whitespace-only values are treated as unset (mirrors devd's own
// strings.TrimSpace guard).
func TestEditorBinaryResolvesVisualThenEditorThenVi(t *testing.T) {
	cases := []struct {
		name   string
		visual string
		editor string
		want   []string
	}{
		{"neither set", "", "", []string{"vi"}},
		{"EDITOR only", "", "nano", []string{"nano"}},
		{"VISUAL wins over EDITOR", "code -w", "nano", []string{"code", "-w"}},
		{"whitespace-only falls back to vi", "   ", "  ", []string{"vi"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VISUAL", tc.visual)
			t.Setenv("EDITOR", tc.editor)
			got := editorBinary()
			if len(got) != len(tc.want) {
				t.Fatalf("editorBinary() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("editorBinary() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestEditorBinaryPrefersConfiguredEditorOverEnv guards design decision c's
// TOP layer (E5 Task 5, bean bt-0l8c): a non-empty configuredEditor
// (Settings.Editor, box_form_settings.go/app.go's config.LoadSettings)
// STRICTLY wins over $VISUAL/$EDITOR, even when both env vars are set --
// Settings > $VISUAL > $EDITOR > vi (the bean's own "design decision c,
// PFLICHT" wording). configuredEditor is reset via t.Cleanup so this test
// can never leak into TestEditorBinaryResolvesVisualThenEditorThenVi (or any
// other test in this package) regardless of run order -- that E3 test stays
// UNCHANGED and green with configuredEditor at its zero-value default "".
func TestEditorBinaryPrefersConfiguredEditorOverEnv(t *testing.T) {
	orig := configuredEditor
	t.Cleanup(func() { configuredEditor = orig })

	configuredEditor = "code -w"
	t.Setenv("VISUAL", "vim")
	t.Setenv("EDITOR", "nano")

	got := editorBinary()
	want := []string{"code", "-w"}
	if len(got) != len(want) {
		t.Fatalf("editorBinary() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("editorBinary() = %v, want %v", got, want)
		}
	}
}

// TestPrepareEditorWritesInitialContentToTempFile guards prepareEditor's
// tempfile setup -- testable WITHOUT a tea runtime (Port devd editor.go
// doc-comment, cmd is built but never run here).
func TestPrepareEditorWritesInitialContentToTempFile(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true") // any resolvable no-op binary; prepareEditor never RUNS cmd

	path, cmd, err := prepareEditor("hello body", ".md")
	if err != nil {
		t.Fatalf("prepareEditor() error = %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != "hello body" {
		t.Fatalf("temp file content = %q, want %q", got, "hello body")
	}
	if !strings.HasSuffix(path, ".md") {
		t.Errorf("temp file path = %q, want a .md suffix", path)
	}
	if cmd == nil {
		t.Fatal("prepareEditor() cmd is nil")
	}
	if len(cmd.Args) < 2 || cmd.Args[0] != "true" || cmd.Args[len(cmd.Args)-1] != path {
		t.Errorf("cmd.Args = %v, want [\"true\" ... %q]", cmd.Args, path)
	}
}

// TestReadEditorResultDetectsChangeAndCleansUp guards the changed-flag +
// tempfile-cleanup contract (Port devd editor.go:70-83).
func TestReadEditorResultDetectsChangeAndCleansUp(t *testing.T) {
	f, err := os.CreateTemp("", "beans-tui-readresult-*.md")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	path := f.Name()
	if _, err := f.WriteString("edited content"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	f.Close()

	msg := readEditorResult(path, "original content", nil)
	if msg.err != nil {
		t.Fatalf("readEditorResult() err = %v, want nil", msg.err)
	}
	if !msg.changed {
		t.Error("changed = false, want true (content differs from initial)")
	}
	if msg.content != "edited content" {
		t.Errorf("content = %q, want %q", msg.content, "edited content")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("readEditorResult did not clean up the temp file")
	}

	// Unchanged content -> changed must be false. errBoom (update_test.go)
	// covers runErr propagation implicitly via the vanished-target test
	// below (a runErr short-circuits before the file is even read).
	f2, err := os.CreateTemp("", "beans-tui-readresult-unchanged-*.md")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	path2 := f2.Name()
	if _, err := f2.WriteString("same"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	f2.Close()
	msg2 := readEditorResult(path2, "same", nil)
	if msg2.changed {
		t.Error("changed = true, want false (content identical to initial)")
	}
}

// --- parseRawBean ---

// TestParseRawBeanRoundTrip guards the split-at-second-"---" + yaml.v3
// unmarshal contract (D01, design-spec.md §15 PF-17) against a REAL
// captured `beans show --raw` shape (this exact text is byte-for-byte the
// tt-epic fixture testrepo_test.go writes to disk, independently verified
// against a live `beans show --raw` run, internal/data's
// TestShowRawReturnsFileFormat) -- "ShowRaw-Format rein -> gleiche Felder
// raus" (bean bt-z4b1's own TDD-Schritte wording).
func TestParseRawBeanRoundTrip(t *testing.T) {
	const raw = `---
# tt-epic
title: Test Epic
status: in-progress
type: epic
priority: normal
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
parent: tt-mlst
tags:
    - urgent
    - backend
---

Epic fixture body.
`
	fm, body, err := parseRawBean(raw)
	if err != nil {
		t.Fatalf("parseRawBean() error = %v", err)
	}
	if fm.Title != "Test Epic" {
		t.Errorf("Title = %q, want %q", fm.Title, "Test Epic")
	}
	if fm.Status != "in-progress" {
		t.Errorf("Status = %q, want %q", fm.Status, "in-progress")
	}
	if fm.Type != "epic" {
		t.Errorf("Type = %q, want %q", fm.Type, "epic")
	}
	if fm.Priority != "normal" {
		t.Errorf("Priority = %q, want %q", fm.Priority, "normal")
	}
	if fm.Parent != "tt-mlst" {
		t.Errorf("Parent = %q, want %q", fm.Parent, "tt-mlst")
	}
	if !equalStrings(fm.Tags, []string{"urgent", "backend"}) {
		t.Errorf("Tags = %v, want [urgent backend]", fm.Tags)
	}
	if len(fm.Blocking) != 0 || len(fm.BlockedBy) != 0 {
		t.Errorf("Blocking/BlockedBy = %v/%v, want both empty (fixture carries neither)", fm.Blocking, fm.BlockedBy)
	}
	const wantBody = "\nEpic fixture body.\n"
	if body != wantBody {
		t.Errorf("body = %q, want %q", body, wantBody)
	}
}

// TestParseRawBeanBodyWithEmbeddedDelimiterLines pins the split-at-SECOND-
// delimiter contract against a body that itself contains "---" lines (F03,
// bt-z4b1 Review-Runde 1): markdown horizontal rules (a completely normal
// bean-body construct -- this very repo's own bean bodies use them) must
// stay IN the body, never be mistaken for the frontmatter's closing
// delimiter. parseRawBean's strings.Index finds the FIRST "\n---\n" after
// the opening delimiter, which is the frontmatter's close -- everything
// after it, embedded "---" lines included, is body verbatim. The logic was
// correct from the start (reviewer sandbox-verified); this test exists to
// keep it correct across refactors.
func TestParseRawBeanBodyWithEmbeddedDelimiterLines(t *testing.T) {
	const raw = `---
# tt-hr
title: Horizontal Rule Fixture
status: todo
type: task
priority: normal
---

Intro paragraph.

---

Section below a markdown horizontal rule.

---
Another one, no blank line before it.
`
	fm, body, err := parseRawBean(raw)
	if err != nil {
		t.Fatalf("parseRawBean() error = %v", err)
	}
	if fm.Title != "Horizontal Rule Fixture" {
		t.Errorf("Title = %q, want %q (frontmatter must end at the FIRST closing delimiter)", fm.Title, "Horizontal Rule Fixture")
	}
	if fm.Status != "todo" || fm.Type != "task" || fm.Priority != "normal" {
		t.Errorf("frontmatter = %+v, want status/type/priority from the REAL frontmatter block only", fm)
	}
	const wantBody = "\nIntro paragraph.\n\n---\n\nSection below a markdown horizontal rule.\n\n---\nAnother one, no blank line before it.\n"
	if body != wantBody {
		t.Errorf("body = %q, want %q (embedded --- lines must stay in the body verbatim)", body, wantBody)
	}
}

// TestParseRawBeanRejectsMalformedFrontmatter guards the error path (D01):
// a missing leading/second "---" delimiter or invalid YAML must all surface
// an error -- applyEditorFinished (update.go) routes any of these into the
// SAME recovery-tempfile convention as a CLI VALIDATION_ERROR, never a
// silent data loss.
func TestParseRawBeanRejectsMalformedFrontmatter(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"missing leading delimiter", "title: Foo\nstatus: todo\n"},
		{"missing second delimiter", "---\ntitle: Foo\nstatus: todo\n"},
		{"invalid yaml", "---\ntitle: [unterminated\n---\n\nbody\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseRawBean(tc.raw)
			if err == nil {
				t.Fatalf("parseRawBean(%q) error = nil, want an error", tc.raw)
			}
		})
	}
}

// --- buildWholeEditDiff ---

// TestBuildWholeEditDiffDetectsPerFieldChanges guards the field-level diff
// (D01, design-spec.md §15 PF-17 Step 2) in isolation, no CLI/tea involved:
// a zero-change round-trip produces a zero-value diff (every pointer nil,
// every slice empty, ParentChanged false) and a full-field change surfaces
// each one independently, including the Tags/Blocking/BlockedBy SET-diffs
// (add/remove, mirrors applyTagPickerDiff's own convention,
// box_picker_tag.go) and Parent's dedicated ParentChanged bool (distinct
// from "" as a valid --remove-parent target).
func TestBuildWholeEditDiffDetectsPerFieldChanges(t *testing.T) {
	snapshot := &item.Item{
		ID: "tk-1", Title: "Old Title", Status: "todo", Type: "task", Priority: "normal",
		Tags: []string{"keep", "drop"}, Blocking: []string{"bk-keep", "bk-drop"},
		BlockedBy: []string{"bb-keep", "bb-drop"}, Parent: "old-parent", Body: "old body",
	}

	t.Run("no changes at all -> zero-value diff", func(t *testing.T) {
		fm := rawBeanFrontmatter{
			Title: snapshot.Title, Status: snapshot.Status, Type: snapshot.Type, Priority: snapshot.Priority,
			Tags: snapshot.Tags, Blocking: snapshot.Blocking, BlockedBy: snapshot.BlockedBy, Parent: snapshot.Parent,
		}
		diff := buildWholeEditDiff(snapshot, fm, snapshot.Body)
		if diff.Title != nil || diff.Status != nil || diff.Type != nil || diff.Priority != nil {
			t.Fatalf("scalar fields = %+v, want all nil (unchanged)", diff)
		}
		if len(diff.TagsAdd) != 0 || len(diff.TagsRemove) != 0 {
			t.Fatalf("Tags diff = add=%v remove=%v, want both empty", diff.TagsAdd, diff.TagsRemove)
		}
		if len(diff.BlockingAdd) != 0 || len(diff.BlockingRemove) != 0 {
			t.Fatalf("Blocking diff not empty: add=%v remove=%v", diff.BlockingAdd, diff.BlockingRemove)
		}
		if len(diff.BlockedByAdd) != 0 || len(diff.BlockedByRemove) != 0 {
			t.Fatalf("BlockedBy diff not empty: add=%v remove=%v", diff.BlockedByAdd, diff.BlockedByRemove)
		}
		if diff.ParentChanged {
			t.Fatal("ParentChanged = true, want false (unchanged)")
		}
		if diff.Body != nil {
			t.Fatal("Body diff set, want nil (unchanged)")
		}
	})

	t.Run("every field changes", func(t *testing.T) {
		fm := rawBeanFrontmatter{
			Title: "New Title", Status: "completed", Type: "bug", Priority: "critical",
			Tags: []string{"keep", "added"}, Blocking: []string{"bk-keep", "bk-added"},
			BlockedBy: []string{"bb-keep", "bb-added"}, Parent: "new-parent",
		}
		diff := buildWholeEditDiff(snapshot, fm, "new body")

		if diff.Title == nil || *diff.Title != "New Title" {
			t.Fatalf("Title = %v, want New Title", diff.Title)
		}
		if diff.Status == nil || *diff.Status != "completed" {
			t.Fatalf("Status = %v, want completed", diff.Status)
		}
		if diff.Type == nil || *diff.Type != "bug" {
			t.Fatalf("Type = %v, want bug", diff.Type)
		}
		if diff.Priority == nil || *diff.Priority != "critical" {
			t.Fatalf("Priority = %v, want critical", diff.Priority)
		}
		if !equalStrings(diff.TagsAdd, []string{"added"}) || !equalStrings(diff.TagsRemove, []string{"drop"}) {
			t.Fatalf("Tags diff = add=%v remove=%v, want add=[added] remove=[drop]", diff.TagsAdd, diff.TagsRemove)
		}
		if !equalStrings(diff.BlockingAdd, []string{"bk-added"}) || !equalStrings(diff.BlockingRemove, []string{"bk-drop"}) {
			t.Fatalf("Blocking diff = add=%v remove=%v", diff.BlockingAdd, diff.BlockingRemove)
		}
		if !equalStrings(diff.BlockedByAdd, []string{"bb-added"}) || !equalStrings(diff.BlockedByRemove, []string{"bb-drop"}) {
			t.Fatalf("BlockedBy diff = add=%v remove=%v", diff.BlockedByAdd, diff.BlockedByRemove)
		}
		if !diff.ParentChanged || diff.Parent != "new-parent" {
			t.Fatalf("Parent diff = changed=%v value=%q, want changed=true value=new-parent", diff.ParentChanged, diff.Parent)
		}
		if diff.Body == nil || *diff.Body != "new body" {
			t.Fatalf("Body = %v, want new body", diff.Body)
		}
	})

	t.Run("parent cleared to empty", func(t *testing.T) {
		fm := rawBeanFrontmatter{
			Title: snapshot.Title, Status: snapshot.Status, Type: snapshot.Type, Priority: snapshot.Priority,
			Tags: snapshot.Tags, Blocking: snapshot.Blocking, BlockedBy: snapshot.BlockedBy, Parent: "",
		}
		diff := buildWholeEditDiff(snapshot, fm, snapshot.Body)
		if !diff.ParentChanged || diff.Parent != "" {
			t.Fatalf("Parent diff = changed=%v value=%q, want changed=true value=\"\" (--remove-parent)", diff.ParentChanged, diff.Parent)
		}
	})
}

// --- showRawCmd / beanRawLoadedMsg (openBeanEditor's FIRST Cmd-hop) ---

// TestShowRawCmdReturnsBeanRawLoadedMsg guards messages.go's showRawCmd
// wiring against Store.Raw: the message carries the item's file as it lies on
// disk, tagged with the id.
func TestShowRawCmdReturnsBeanRawLoadedMsg(t *testing.T) {
	store := fixtureStore(t)
	cmd := showRawCmd(store, "tk-1")
	if cmd == nil {
		t.Fatal("showRawCmd() returned a nil Cmd")
	}
	msg := cmd()
	loaded, ok := msg.(beanRawLoadedMsg)
	if !ok {
		t.Fatalf("showRawCmd()()  = %T, want beanRawLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("beanRawLoadedMsg.err = %v, want nil", loaded.err)
	}
	if loaded.id != "tk-1" {
		t.Fatalf("id = %q, want tk-1", loaded.id)
	}
	if want := "title: " + storeItem(t, store, "tk-1").Title; !strings.Contains(loaded.raw, want) {
		t.Fatalf("raw = %q, want it to contain %q", loaded.raw, want)
	}
	if missing := showRawCmd(store, "no-such-item")().(beanRawLoadedMsg); missing.err == nil {
		t.Fatal("showRawCmd for an unknown id: want an error")
	}
}

// TestApplyBeanRawLoadedFiresEditorSuspendOnSuccess guards Update()'s
// beanRawLoadedMsg case (the SECOND Cmd-hop, design-spec §15 PF-17: "zwei
// Cmd-Hops statt einem"): a successful ShowRaw read must fire the actual
// tea.ExecProcess suspend and leave editorTarget/editorETag/editorSnapshot
// untouched (they stay frozen until applyEditorFinished's own tail).
func TestApplyBeanRawLoadedFiresEditorSuspendOnSuccess(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	snap := *m.idx.ByID["tk-1"]
	m.editorTarget = "tk-1"
	m.editorETag = "tk-1-etag"
	m.editorSnapshot = &snap

	tm, cmd := m.Update(beanRawLoadedMsg{id: "tk-1", raw: "---\ntitle: X\n---\n\nbody\n"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(beanRawLoadedMsg) did not return a model, got %T", tm)
	}
	if nm.editorTarget != "tk-1" || nm.editorETag != "tk-1-etag" || nm.editorSnapshot == nil {
		t.Fatalf("editor-open state must stay frozen across the second Cmd-hop: target=%q etag=%q snapshot=%v", nm.editorTarget, nm.editorETag, nm.editorSnapshot)
	}
	if cmd == nil {
		t.Fatal("a successful ShowRaw load must return a Cmd (the ExecProcess-wrapped editor suspend)")
	}
}

// TestApplyBeanRawLoadedErrorSurfacesToastAndResetsEditorState guards the
// failure path: a ShowRaw error must never suspend into $EDITOR on content
// that never successfully loaded -- surfaces a toast and clears the
// editor-open state instead (mirrors applyEditorFinished's own err path).
func TestApplyBeanRawLoadedErrorSurfacesToastAndResetsEditorState(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	snap := *m.idx.ByID["tk-1"]
	m.editorTarget = "tk-1"
	m.editorETag = "tk-1-etag"
	m.editorSnapshot = &snap

	tm, cmd := m.Update(beanRawLoadedMsg{id: "tk-1", err: errors.New("show --raw boom")})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(beanRawLoadedMsg) did not return a model, got %T", tm)
	}
	if nm.editorTarget != "" || nm.editorETag != "" || nm.editorSnapshot != nil {
		t.Fatalf("editor-open state must be cleared on a ShowRaw error: target=%q etag=%q snapshot=%v", nm.editorTarget, nm.editorETag, nm.editorSnapshot)
	}
	if nm.err == "" {
		t.Fatal("a ShowRaw error must surface a status-line note")
	}
	if cmd == nil {
		t.Fatal("a ShowRaw error must still fire the error-Toast's auto-dismiss Cmd")
	}
}

// TestApplyBeanRawLoadedDiscardsStaleLoad guards the defensive msg.id
// mismatch check: a load resolving for a DIFFERENT id than the CURRENT
// editorTarget must be a pure no-op, never suspending into the wrong
// bean's editor.
func TestApplyBeanRawLoadedDiscardsStaleLoad(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	snap := *m.idx.ByID["tk-2"]
	m.editorTarget = "tk-2"
	m.editorETag = "tk-2-etag"
	m.editorSnapshot = &snap

	tm, cmd := m.Update(beanRawLoadedMsg{id: "tk-1", raw: "---\ntitle: X\n---\n\nbody\n"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(beanRawLoadedMsg) did not return a model, got %T", tm)
	}
	if nm.editorTarget != "tk-2" {
		t.Fatalf("editorTarget = %q, want unchanged tk-2 (stale load for a different id)", nm.editorTarget)
	}
	if cmd != nil {
		t.Fatal("a stale load must fire no Cmd at all")
	}
}

// --- applyEditorFinished ---

// TestEditorFinishedUnchangedFiresNoMutation guards the changed==false
// no-op path: an unedited editor round-trip must not fire any mutation, and
// must clear the full editor-open state (target/etag/snapshot).
func TestEditorFinishedUnchangedFiresNoMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.store = newTestStore(t)
	snap := *m.idx.ByID["tk-1"]
	m.editorTarget = "tk-1"
	m.editorETag = "captured-etag"
	m.editorSnapshot = &snap

	tm, cmd := m.Update(editorFinishedMsg{content: "same raw content", changed: false})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if cmd != nil {
		t.Fatal("unchanged editor content must not fire a mutation Cmd")
	}
	if nm.editorTarget != "" {
		t.Errorf("editorTarget = %q, want cleared after handling", nm.editorTarget)
	}
	if nm.editorETag != "" {
		t.Errorf("editorETag = %q, want cleared after handling (F2)", nm.editorETag)
	}
	if nm.editorSnapshot != nil {
		t.Errorf("editorSnapshot = %v, want cleared after handling", nm.editorSnapshot)
	}
}

// TestApplyEditorFinishedBuildsCombinedUpdateWholeCall guards the core D01
// round-trip: a changed editor return is parsed (parseRawBean), diffed
// against m.editorSnapshot (buildWholeEditDiff), and dispatched as ONE
// Store.UpdateWhole call carrying ONLY the changed fields -- verified on the
// item on disk afterwards: changed fields landed, unchanged ones kept.
func TestApplyEditorFinishedBuildsCombinedUpdateWholeCall(t *testing.T) {
	beans := fixtureBeans()
	for _, b := range beans {
		if b.ID == "tk-1" {
			b.Title, b.Tags, b.Body = "Old Title", []string{"keep", "drop"}, "old body"
		}
	}
	store := newTestStore(t, beans...)
	before := storeItem(t, store, "tk-1")

	m := fixtureModel(t, beans)
	m.store = store
	m.editorTarget = "tk-1"
	m.editorETag = before.ETag()
	m.editorSnapshot = before

	fm := rawBeanFrontmatter{
		Title: "New Title", Status: before.Status, Type: before.Type, Priority: before.Priority, Parent: before.Parent,
		Tags: []string{"keep", "added"},
	}
	raw := rawEditorText(t, fm, "new body")

	tm, cmd := m.Update(editorFinishedMsg{content: raw, changed: true})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if nm.editorTarget != "" || nm.editorETag != "" || nm.editorSnapshot != nil {
		t.Fatalf("editor-open state must be cleared: target=%q etag=%q snapshot=%v", nm.editorTarget, nm.editorETag, nm.editorSnapshot)
	}
	if cmd == nil {
		t.Fatal("changed editor content must fire a mutation Cmd")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err != nil {
		t.Fatalf("UpdateWhole failed: %v", mdm.err)
	}
	got := storeItem(t, store, "tk-1")
	if got.Title != "New Title" || !got.HasTag("added") || !got.HasTag("keep") || got.HasTag("drop") ||
		!strings.Contains(got.Body, "new body") {
		t.Errorf("changed fields did not land: %+v", got)
	}
	if got.Status != before.Status || got.Type != before.Type || got.Priority != before.Priority || got.Parent != before.Parent {
		t.Errorf("unchanged fields were touched: status %q type %q priority %q parent %q", got.Status, got.Type, got.Priority, got.Parent)
	}
}

// TestEditorFinishedUsesEtagCapturedAtOpenNotFreshIndexRead is F2's core
// regression test (Review-Runde 2, Finding 2, ETag-Lost-Update), carried
// forward into D01's two-Cmd-hop design: a watch-reload landing WHILE the
// $EDITOR session is still open (ShowRaw-read or $EDITOR-suspend in
// flight) rotates tk-1's etag in the LIVE index -- design decision d's
// "always read fresh at submit" would silently pick up that NEW etag and
// let UpdateWhole sail through with no conflict ever raised. Freezing the
// etag AND the full snapshot at open (openBeanEditor, editor.go) must mean
// applyEditorFinished still dispatches the OLD (open-time) etag even after
// the reload. Proven against a real store: an external write makes the
// open-time etag stale on disk, so it fails with ErrConflict where a fresh
// re-read at submit would have sailed through.
func TestEditorFinishedUsesEtagCapturedAtOpenNotFreshIndexRead(t *testing.T) {
	store := fixtureStore(t)
	openETag := storeItem(t, store, "tk-1").ETag()
	m := fixtureModel(t, fixtureBeans())
	m.store = store
	m = focusBean(m, "tk-1")

	// ctrl+e captures editorETag/editorSnapshot from the CURRENT (open-time) index.
	tm, cmd := m.Update(keyMsg(tea.KeyCtrlE))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(ctrl+e) did not return a model, got %T", tm)
	}
	if nm.editorETag != openETag {
		t.Fatalf("editorETag = %q, want %q (captured at open)", nm.editorETag, openETag)
	}
	if nm.editorSnapshot == nil || nm.editorSnapshot.ETag() != openETag {
		t.Fatalf("editorSnapshot = %+v, want a snapshot carrying ETag %q", nm.editorSnapshot, openETag)
	}
	if cmd == nil {
		t.Fatal("ctrl+e must return a Cmd (the ShowRaw-read, first Cmd-hop)")
	}

	// A watch-reload lands WHILE the $EDITOR-open sequence is still in
	// flight, rotating tk-1's etag in the live index -- the exact async
	// window F2 guards.
	if err := store.SetPriority("tk-1", "critical", openETag); err != nil {
		t.Fatalf("setup: external write: %v", err)
	}
	nm = step(t, nm, beansLoadedMsg{beans: store.Items()})
	if nm.editorETag != openETag {
		t.Fatalf("editorETag = %q, want unchanged by an unrelated reload", nm.editorETag)
	}
	if nm.editorSnapshot.ETag() != openETag {
		t.Fatalf("editorSnapshot.ETag() = %q, want unchanged by an unrelated reload", nm.editorSnapshot.ETag())
	}

	// The $EDITOR session finishes -- what matters here is the --if-match
	// VALUE the resulting UpdateWhole call dispatches, not the content.
	fm := rawBeanFrontmatter{
		Title: nm.editorSnapshot.Title, Status: nm.editorSnapshot.Status,
		Type: nm.editorSnapshot.Type, Priority: nm.editorSnapshot.Priority,
	}
	raw := rawEditorText(t, fm, "new body")
	tm2, cmd2 := nm.Update(editorFinishedMsg{content: raw, changed: true})
	if _, ok := tm2.(model); !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm2)
	}
	if cmd2 == nil {
		t.Fatal("changed editor content must fire a mutation Cmd")
	}
	msg := cmd2()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if !errors.Is(mdm.err, data.ErrConflict) {
		t.Fatalf("UpdateWhole error = %v, want ErrConflict: the OPEN-time etag is stale on disk (F2: must NOT re-read m.idx fresh at submit)", mdm.err)
	}
	if strings.Contains(storeItem(t, store, "tk-1").Body, "new body") {
		t.Fatal("the stale-etag edit reached the disk (F2 regression: the RELOADED etag was dispatched)")
	}
}

// TestApplyEditorFinishedRecoversTempfileOnParseError guards the
// parse-error recovery path (design-spec.md §15 PF-17 "Fehlerfall"): the
// Ganz-Bean-Editor is bewusst UNconstrained (Freitext-YAML) -- a malformed
// edit (e.g. a broken frontmatter delimiter) must not silently discard the
// PO's work. Mirrors writeConflictTempFile's existing recovery convention,
// but persists the FULL raw text (not just a body), since the whole edited
// file is what needs to survive. RENAMED from ...OnValidationError (F02,
// bt-z4b1 Review-Runde 1): the old name suggested CLI-VALIDATION_ERROR
// coverage while only exercising the parseRawBean failure BEFORE any CLI
// call ever fires -- exactly the false confidence that let F01 slip
// through. The CLI-side case has its own dedicated test now
// (TestApplyEditorFinishedRecoversTempfileOnCLIValidationError below).
func TestApplyEditorFinishedRecoversTempfileOnParseError(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.store = fixtureStore(t)
	m.editorTarget = "tk-1"
	m.editorETag = "etag-open"
	m.editorSnapshot = &item.Item{ID: "tk-1", Title: "Task One", Status: "in-progress", Type: "task", Priority: "high"}

	const malformed = "not a valid raw bean at all, no frontmatter delimiters"
	tm, cmd := m.Update(editorFinishedMsg{content: malformed, changed: true})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("a parse error must still fire the error-Toast's auto-dismiss Cmd")
	}
	if nm.editorTarget != "" || nm.editorETag != "" || nm.editorSnapshot != nil {
		t.Fatalf("editor-open state must be cleared even on a parse error: target=%q etag=%q snapshot=%v", nm.editorTarget, nm.editorETag, nm.editorSnapshot)
	}
	if !strings.Contains(strings.ToLower(nm.err), "invalid") {
		t.Fatalf("status line = %q, want it to mention the invalid/malformed content", nm.err)
	}
	const marker = "your version: "
	idx := strings.Index(nm.err, marker)
	if idx < 0 {
		t.Fatalf("status line = %q, want it to carry a recovery tempfile path", nm.err)
	}
	path := strings.TrimSpace(nm.err[idx+len(marker):])
	content, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("ReadFile(%q) error = %v (recovery tempfile missing)", path, rerr)
	}
	if string(content) != malformed {
		t.Fatalf("recovery tempfile content = %q, want %q", content, malformed)
	}
}

// TestApplyEditorFinishedRecoversTempfileOnCLIValidationError is F01's
// regression test (critical, bt-z4b1 Review-Runde 1, live reproduced by the
// reviewer): a CLI-side VALIDATION_ERROR from UpdateWhole AFTER a successful
// parse (e.g. a hand-typed `status: banana`, or a deleted title: line
// reaching the CLI as --title "") used to lose the ENTIRE $EDITOR raw text
// -- applyEditorFinished's mutateCmd closure only wrapped
// errors.Is(err, data.ErrConflict) failures in writeConflictTempFile/
// conflictWithRecovery, so every non-conflict CLI rejection fell through to
// applyMutationResult's bare else branch with nothing persisted.
// design-spec §15 PF-17's Fehlerfall wording explicitly covers BOTH
// ("Parse-Fehler ODER CLI-VALIDATION_ERROR ... mirrort
// writeConflictTempFile"). EVERY UpdateWhole failure must now be
// recoverable: tempfile exists, carries the FULL raw edited text, and its
// path rides along in the status-line text.
func TestApplyEditorFinishedRecoversTempfileOnCLIValidationError(t *testing.T) {
	store := fixtureStore(t)
	m := fixtureModel(t, fixtureBeans())
	m.store = store
	m.editorTarget = "tk-1"
	m.editorETag = storeItem(t, store, "tk-1").ETag() // current, so the rejection is validation, not conflict
	m.editorSnapshot = &item.Item{ID: "tk-1", Title: "Task One", Status: "in-progress", Type: "task", Priority: "high"}

	// Parses fine (valid YAML/delimiters) -- the CLI is what rejects it.
	fm := rawBeanFrontmatter{Title: "Task One", Status: "banana", Type: "task", Priority: "high"}
	raw := rawEditorText(t, fm, "edited body that must not be lost to a CLI rejection")

	tm, cmd := m.Update(editorFinishedMsg{content: raw, changed: true})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("changed editor content must fire a mutation Cmd")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err == nil {
		t.Fatal("setup: the invalid status must be rejected")
	}
	if errors.Is(mdm.err, data.ErrConflict) {
		t.Fatalf("mutationDoneMsg.err = %v, want a NON-conflict error (F01 is precisely about the non-conflict path)", mdm.err)
	}

	fm2 := step(t, nm, mdm)
	if !strings.Contains(fm2.err, "invalid status") {
		t.Fatalf("status line = %q, want it to surface the CLI's own validation error", fm2.err)
	}
	const marker = "your version: "
	idx := strings.Index(fm2.err, marker)
	if idx < 0 {
		t.Fatalf("status line = %q, want it to carry the recovery tempfile's path (%q) -- F01: a CLI rejection must not lose the PO's edits", fm2.err, marker)
	}
	path := strings.TrimSpace(fm2.err[idx+len(marker):])
	content, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("ReadFile(%q) error = %v (recovery tempfile missing)", path, rerr)
	}
	if string(content) != raw {
		t.Fatalf("recovery tempfile content = %q, want the FULL raw edited text %q", content, raw)
	}
}

// TestApplyEditorFinishedConflictWritesRecoveryTempFileAndSurfacesPath
// guards F2's nice-to-have (Review-Runde 2): on a genuine ErrConflict, the
// PO's just-$EDITOR-edited FULL raw text would otherwise be silently
// discarded by applyMutationResult's unconditional reload -- it must
// instead be persisted to a KEPT tempfile first, with the file's path
// riding along in the status-line text applyMutationResult (update.go)
// produces.
func TestApplyEditorFinishedConflictWritesRecoveryTempFileAndSurfacesPath(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.store = fixtureStore(t)
	m.editorTarget = "tk-1"
	m.editorETag = "etag-open"
	m.editorSnapshot = &item.Item{ID: "tk-1", Title: "Task One", Status: "in-progress", Type: "task", Priority: "high", Body: "old body"}

	fm := rawBeanFrontmatter{Title: "Task One", Status: "in-progress", Type: "task", Priority: "high"}
	raw := rawEditorText(t, fm, "edited body that must not be lost")

	tm, cmd := m.Update(editorFinishedMsg{content: raw, changed: true})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("changed editor content must fire a mutation Cmd")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if !errors.Is(mdm.err, data.ErrConflict) {
		t.Fatalf("mutationDoneMsg.err = %v, want errors.Is(_, data.ErrConflict)", mdm.err)
	}

	fm2 := step(t, nm, mdm)
	if !strings.Contains(fm2.err, "Conflict") {
		t.Fatalf("status line = %q, want it to mention the conflict", fm2.err)
	}
	const marker = "your version: "
	idx := strings.Index(fm2.err, marker)
	if idx < 0 {
		t.Fatalf("status line = %q, want it to carry the recovery tempfile's path (%q)", fm2.err, marker)
	}
	path := strings.TrimSpace(fm2.err[idx+len(marker):])
	content, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("ReadFile(%q) error = %v (recovery tempfile missing)", path, rerr)
	}
	if string(content) != raw {
		t.Fatalf("recovery tempfile content = %q, want the FULL raw edited text %q (not just the body)", content, raw)
	}
}

// TestEditorFinishedTargetVanishedSurfacesError guards the vanished-target
// guard (mirrors beanETag's every other caller): the bean editorTarget
// names is no longer in the index -- no doomed mutation Cmd, a status-line
// note instead. m.beanETag(id) is still consulted here, but ONLY for its ok
// bool (bean presence) -- the etag VALUE it would return stays discarded in
// favor of m.editorETag (unchanged F2 contract).
func TestEditorFinishedTargetVanishedSurfacesError(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.editorTarget = "does-not-exist"
	m.editorETag = "captured-etag"
	m.editorSnapshot = &item.Item{ID: "does-not-exist", Title: "Ghost"}

	tm, cmd := m.Update(editorFinishedMsg{content: "new content", changed: true})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("a vanished target must still fire the warn-Toast's auto-dismiss Cmd (E5 Task 1)")
	}
	if nm.err == "" {
		t.Fatal("a vanished target must surface a status-line error")
	}
	// F04 (bean bt-6bgn) split the former title==m.err mirror: m.err now
	// additionally carries the recovery tempfile's path (see
	// TestApplyEditorFinishedVanishedTargetRecoversTempfile below), while
	// the toast title stays the bare note -- same title/ctx split as the
	// conflict branch and the F01 else branch (applyMutationResult,
	// update.go). The Dual-Write contract (toast mirrors the status-line
	// NOTE) holds as a prefix relation now.
	// bt-6bgn-Review F05 hardening (2026-07-16, PRELUDE of bt-2v38):
	// strings.HasPrefix(nm.err, nm.toast.title) is trivially true for an
	// EMPTY toast.title (every string has "" as a prefix) -- the explicit
	// nm.toast.title == "" check below closes that blind spot so this guard
	// can no longer pass on an accidentally-blank toast title.
	if nm.toast == nil || nm.toast.kind != toastWarn || nm.toast.title == "" || !strings.HasPrefix(nm.err, nm.toast.title) {
		t.Fatalf("toast = %+v, want a non-nil toastWarn with a non-empty title that prefixes m.err %q (E5 Task 1 Dual-Write, F04 title/ctx split, F05 hardening)", nm.toast, nm.err)
	}
	if nm.editorTarget != "" {
		t.Errorf("editorTarget = %q, want cleared even on the vanished-target path", nm.editorTarget)
	}
	if nm.editorETag != "" {
		t.Errorf("editorETag = %q, want cleared even on the vanished-target path (F2)", nm.editorETag)
	}
	if nm.editorSnapshot != nil {
		t.Errorf("editorSnapshot = %v, want cleared even on the vanished-target path", nm.editorSnapshot)
	}
}

// TestApplyEditorFinishedVanishedTargetRecoversTempfile is F04's regression
// test (bean bt-6bgn, T2-Re-Review-Fund -- same data-loss class as F01):
// the vanished-target guard ("Bean no longer exists") used to discard the
// FULL $EDITOR raw text with no recovery tempfile when the bean was deleted
// externally during the editor session. Pikant: WITHOUT the guard the CLI
// call would fail and F01's wrap (Commit 34c62c8) would recover -- the
// guard actively PREVENTED the recovery. Same convention as the F01 fix
// now: writeConflictTempFile(msg.content), path appended to the warn
// message (m.err), toast ctx carries "Version saved:". Warn semantics
// (toastWarn, not an error) stay unchanged -- that part is pinned by
// TestEditorFinishedTargetVanishedSurfacesError above; THIS test pins the
// recovery half: tempfile exists AND carries the full raw edited text.
func TestApplyEditorFinishedVanishedTargetRecoversTempfile(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.editorTarget = "does-not-exist"
	m.editorETag = "captured-etag"
	m.editorSnapshot = &item.Item{ID: "does-not-exist", Title: "Ghost", Status: "todo", Type: "task", Priority: "normal"}

	fm := rawBeanFrontmatter{Title: "Ghost Edited", Status: "todo", Type: "task", Priority: "normal"}
	raw := rawEditorText(t, fm, "edited body that must survive an externally deleted target")

	tm, cmd := m.Update(editorFinishedMsg{content: raw, changed: true})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(editorFinishedMsg) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("a vanished target must still fire the warn-Toast's auto-dismiss Cmd")
	}
	if nm.toast == nil || nm.toast.kind != toastWarn {
		t.Fatalf("toast = %+v, want a toastWarn (vanished target stays a warning, not an error -- F04 changes only the recovery half)", nm.toast)
	}
	const marker = "your version: "
	idx := strings.Index(nm.err, marker)
	if idx < 0 {
		t.Fatalf("status line = %q, want it to carry the recovery tempfile's path (%q) -- F04: a vanished target must not lose the PO's edits", nm.err, marker)
	}
	path := strings.TrimSpace(nm.err[idx+len(marker):])
	content, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("ReadFile(%q) error = %v (recovery tempfile missing)", path, rerr)
	}
	if string(content) != raw {
		t.Fatalf("recovery tempfile content = %q, want the FULL raw edited text %q", content, raw)
	}
}
