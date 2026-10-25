package tui

// editor.go — the $EDITOR-Suspend machinery (`e`/`ctrl+e`, D01, design-
// spec.md §15 PF-17, bean bt-z4b1): tea.ExecProcess suspends the running
// Bubble Tea program, hands the terminal to the user's editor on a temp file
// seeded with the bean's FULL raw markdown representation (`beans show <id>
// --raw`, byte-identical to the on-disk .beans/*.md file), and resumes once
// the editor exits. Port devd editor.go:14-96 (editorFinishedMsg/
// prepareEditor/readEditorResult/editInEditor) VERBATIM -- these stay
// suffix/content-agnostic, unaffected by D01's scope change from
// Body-only to whole-bean.
//
// openBeanEditor (below) REPLACES the former openBodyEditor (E3 Task 5/E8
// B10, both SUPERSEDED by D01): "e"/"ctrl+e" now UNCONDITIONALLY open the
// whole bean, not just its Body, and never open the Title-Edit-Form either
// -- ONE editor helper, ONE dispatch path, no more context-sensitive
// branching (update.go's keyNodeAction). Because ShowRaw is itself a
// subprocess call (~20-50ms, design-spec §3.1), openBeanEditor cannot fire
// tea.ExecProcess directly -- it returns showRawCmd's Cmd (messages.go)
// instead, and Update()'s new beanRawLoadedMsg case (update.go) fires the
// ACTUAL suspend once that read resolves (two Cmd-hops, not one).
//
// editorBinary()'s cascade (E5 Task 5, bean bt-0l8c, updates the E3-era
// comment this file used to carry here): configuredEditor (Settings, E5's
// ~/.config/beans-tui/config.yaml -- now wired, see configuredEditor's own
// doc-stamp below) -> $VISUAL -> $EDITOR -> a bare "vi" fallback
// (design-spec §7 says literally "$EDITOR"; "vi" is the POSIX default,
// portable everywhere -- devd's "nvim" assumption is not).
//
// glowRender (devd editor.go:98-126) is NOT ported here -- beans-tui already
// has its own copy in accordion.go (E2, view_detail_bean.go's Body-section
// render path); this file only carries the Editor-Suspend half.

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"os"
	"os/exec"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
	"gopkg.in/yaml.v3"
)

// editorFinishedMsg carries an $EDITOR-Suspend session's result back into the
// Update loop (Port devd editor.go:21-25). The caller (keyNodeAction's
// ctrl+e branch, update.go) remembers WHICH bean is being edited via
// m.editorTarget, captured BEFORE the suspend fires -- content/changed/err
// alone carry no bean identity.
type editorFinishedMsg struct {
	content string
	changed bool
	err     error
}

// configuredEditor is E5's Settings override (design decision c, PFLICHT:
// Settings.Editor > $VISUAL > $EDITOR > vi -- bean bt-0l8c, epic bt-5h4d).
// Package-level like devd's own configuredEditor: set once at TUI-start
// (app.go Run(), config.LoadSettings) and LIVE-updated by the Settings-Form's
// submit (box_form_settings.go, Port devd DD2-221). Default "" -- a NEW
// layer STRICTLY above the pre-existing $VISUAL/$EDITOR/vi cascade below, so
// an empty value (out-of-the-box, config.yaml never touched) falls straight
// through unchanged: TestEditorBinaryResolvesVisualThenEditorThenVi remains
// green without modification.
var configuredEditor string

// editorBinary resolves the editor command to launch (design decision c):
// configuredEditor (Settings, E5) -> $VISUAL -> $EDITOR -> a bare "vi"
// fallback (POSIX default -- portable everywhere, unlike devd's "nvim"
// assumption; design-spec §7 says literally "$EDITOR"). A value may carry
// arguments (e.g. "code -w"), split on whitespace like devd's own
// editorBinary.
func editorBinary() []string {
	if ed := strings.TrimSpace(configuredEditor); ed != "" {
		return strings.Fields(ed)
	}
	for _, envVar := range []string{"VISUAL", "EDITOR"} {
		if ed := strings.TrimSpace(os.Getenv(envVar)); ed != "" {
			return strings.Fields(ed)
		}
	}
	return []string{"vi"}
}

// prepareEditor writes initial into a temp file and builds the exec.Cmd that
// opens the resolved editor on it. Factored out for testability (no tea
// runtime needed, Port devd editor.go:44-68 comment). The caller owns
// os.Remove(path).
func prepareEditor(initial, suffix string) (path string, cmd *exec.Cmd, err error) {
	if suffix == "" {
		suffix = ".md"
	}
	f, err := os.CreateTemp("", "lif-tui-*"+suffix)
	if err != nil {
		return "", nil, err
	}
	path = f.Name()
	if _, err = f.WriteString(initial); err != nil {
		f.Close()
		os.Remove(path)
		return "", nil, err
	}
	if err = f.Close(); err != nil {
		os.Remove(path)
		return "", nil, err
	}
	bin := editorBinary()
	cmd = exec.Command(bin[0], append(bin[1:], path)...)
	return path, cmd, nil
}

// readEditorResult reads the (possibly edited) file back, compares against
// initial, and cleans up the temp file. runErr is the editor process's own
// exit error (Port devd editor.go:70-83).
func readEditorResult(path, initial string, runErr error) editorFinishedMsg {
	defer os.Remove(path)
	if runErr != nil {
		return editorFinishedMsg{err: runErr}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return editorFinishedMsg{err: err}
	}
	content := string(data)
	return editorFinishedMsg{content: content, changed: content != initial}
}

// editInEditor opens initial in the resolved editor via a tea.ExecProcess
// suspend, returning an editorFinishedMsg once the editor exits (Port devd
// editor.go:85-96). suffix controls the temp file's extension (".md" for a
// bean's body).
func editInEditor(initial, suffix string) tea.Cmd {
	path, cmd, err := prepareEditor(initial, suffix)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg{err: err} }
	}
	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		return readEditorResult(path, initial, runErr)
	})
}

// openBeanEditor suspends into $EDITOR on b's FULL raw markdown
// representation (D01, design-spec.md §15 PF-17, bean bt-z4b1) -- the ONE
// shared helper for the ONE remaining "e"/"ctrl+e" call site (keyNodeAction,
// update.go), REPLACING the former openBodyEditor (Body-only, E3 Task 5/E8
// B10 -- both superseded). Unlike openBodyEditor, the raw text is NOT
// already in memory: ShowRaw is a subprocess read (~20-50ms, design-spec
// §3.1) that must never run synchronously inside Update, so this returns
// showRawCmd's Cmd (messages.go) -- the FIRST of two Cmd-hops. The actual
// tea.ExecProcess suspend fires once that read resolves (Update()'s new
// beanRawLoadedMsg case, update.go).
//
// editorTarget/editorETag/editorSnapshot are all frozen HERE, at
// $EDITOR-open time (F2, mirrors openBodyEditor's own rationale -- never a
// fresh m.beanETag(id)/m.idx re-read later). editorSnapshot (types.go, NEW
// field) is the FULL *item.Item value, not just ID+ETag: applyEditorFinished
// diffs every field individually against it (buildWholeEditDiff below), so
// title/status/type/priority/tags/blocking/blocked_by/parent/body all need
// their open-time value on hand, not just enough to detect an ETag
// conflict. A local copy (not a pointer into m.idx) so a later
// beansLoadedMsg reload can never mutate it out from under an in-flight
// $EDITOR session (same "frozen, not live" contract editorETag already
// established).
func (m model) openBeanEditor(b *item.Item) (model, tea.Cmd) {
	m.editorTarget = b.ID
	m.editorETag = b.ETag()
	snap := *b
	m.editorSnapshot = &snap
	return m, showRawCmd(m.store, b.ID)
}

// rawBeanFrontmatter is the parse target for a whole-bean $EDITOR round-trip
// (D01, design-spec.md §15 PF-17) -- gopkg.in/yaml.v3 unmarshals the
// frontmatter block parseRawBean (below) splits off. created_at/
// updated_at/the ID itself are deliberately NOT fields here: `beans update`
// has no flag for any of them -- the "Bekannte Grenze" applyEditorFinished's
// own doc-stamp documents (update.go) as an ERRATUM, not a bug. The
// "# <id>" comment line ShowRaw's output carries is a YAML comment, yaml.v3
// skips it automatically.
type rawBeanFrontmatter struct {
	Title     string   `yaml:"title"`
	Status    string   `yaml:"status"`
	Type      string   `yaml:"type"`
	Priority  string   `yaml:"priority"`
	Tags      []string `yaml:"tags"`
	Parent    string   `yaml:"parent"`
	Blocking  []string `yaml:"blocking"`
	BlockedBy []string `yaml:"blocked_by"`
}

// parseRawBean splits raw (the $EDITOR's returned content, the SAME shape
// as ShowRaw's own seed text) at the SECOND "---" delimiter into
// frontmatter + body, then yaml-unmarshals the frontmatter block. Returns
// an error for malformed input (missing leading/second delimiter, invalid
// YAML) -- the caller (applyEditorFinished, update.go) surfaces it via the
// SAME recovery-tempfile convention as a CLI VALIDATION_ERROR (design-spec
// §15 PF-17's "Fehlerfall": the whole-bean editor is bewusst UNconstrained
// Freitext-YAML, so a malformed edit must be recoverable, never silently
// discarded).
//
// body's leading-newline convention deliberately mirrors item.Item.Body's
// own CLI-JSON shape (verified empirically, internal/data's
// TestUpdateWholeSendsOnlyChangedFields "body only" subtest): the on-disk
// format always has a blank line between the frontmatter's closing "---"
// and the body text, so everything AFTER the second delimiter's own
// trailing newline -- including that blank line -- becomes the body,
// unmodified.
func parseRawBean(raw string) (rawBeanFrontmatter, string, error) {
	const openDelim = "---\n"
	if !strings.HasPrefix(raw, openDelim) {
		return rawBeanFrontmatter{}, "", fmt.Errorf("parseRawBean: content does not start with a %q frontmatter delimiter", "---")
	}
	rest := raw[len(openDelim):]

	const closeDelim = "\n---\n"
	idx := strings.Index(rest, closeDelim)
	if idx < 0 {
		return rawBeanFrontmatter{}, "", fmt.Errorf("parseRawBean: missing second %q frontmatter delimiter", "---")
	}
	fmBlock := rest[:idx]
	body := rest[idx+len(closeDelim):]

	var fm rawBeanFrontmatter
	if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
		return rawBeanFrontmatter{}, "", fmt.Errorf("parseRawBean: invalid YAML frontmatter: %w", err)
	}
	return fm, body, nil
}

// diffStringSlices computes the add/remove SET-diff between old and new
// (both treated as unordered sets -- mirrors applyTagPickerDiff's/
// applyBlockingPickerDiff's own add/remove convention, box_picker_tag.go/
// box_picker_blocking.go), sorted for deterministic CLI flag order.
func diffStringSlices(old, new []string) (add, remove []string) {
	oldSet := make(map[string]bool, len(old))
	for _, v := range old {
		oldSet[v] = true
	}
	newSet := make(map[string]bool, len(new))
	for _, v := range new {
		newSet[v] = true
	}
	for v := range newSet {
		if !oldSet[v] {
			add = append(add, v)
		}
	}
	for v := range oldSet {
		if !newSet[v] {
			remove = append(remove, v)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}

// buildWholeEditDiff computes the field-level data.WholeEditDiff between
// snapshot (the bean's state frozen at $EDITOR-open time, m.editorSnapshot)
// and the $EDITOR round-trip's parsed return (fm, body) -- D01, design-spec
// §15 PF-17 Step 2. Only fields that actually differ carry a non-nil/
// non-empty value (data.Store.UpdateWhole's own "only changed fields"
// contract, mutations.go) -- a fully unchanged round-trip (e.g. the PO only
// reformatted whitespace) produces a zero-value diff, which UpdateWhole
// turns into NO CLI call at all. created_at/updated_at/the ID are not
// compared here -- they are not even parsed into rawBeanFrontmatter
// (parseRawBean's own doc-stamp), the "Bekannte Grenze" this task documents
// as an ERRATUM.
func buildWholeEditDiff(snapshot *item.Item, fm rawBeanFrontmatter, body string) data.WholeEditDiff {
	var diff data.WholeEditDiff
	if fm.Title != snapshot.Title {
		diff.Title = &fm.Title
	}
	if fm.Status != snapshot.Status {
		diff.Status = &fm.Status
	}
	if fm.Type != snapshot.Type {
		diff.Type = &fm.Type
	}
	if fm.Priority != snapshot.Priority {
		diff.Priority = &fm.Priority
	}
	diff.TagsAdd, diff.TagsRemove = diffStringSlices(snapshot.Tags, fm.Tags)
	diff.BlockingAdd, diff.BlockingRemove = diffStringSlices(snapshot.Blocking, fm.Blocking)
	diff.BlockedByAdd, diff.BlockedByRemove = diffStringSlices(snapshot.BlockedBy, fm.BlockedBy)
	if fm.Parent != snapshot.Parent {
		diff.ParentChanged = true
		diff.Parent = fm.Parent
	}
	if body != snapshot.Body {
		diff.Body = &body
	}
	return diff
}
