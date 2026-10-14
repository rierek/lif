package commands

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xRiErOS/lif/internal/ui"
	"github.com/xRiErOS/lif/pkg/candidates"
	"github.com/xRiErOS/lif/pkg/item"
)

const keyUp = "\x1b[A"

// cmRun runs the menu on a draft with scripted programs and returns the
// action, error and chrome.
func cmRun(t *testing.T, d *createDraft, programs ...[]string) (createMenuAction, error, *bytes.Buffer) {
	t.Helper()
	var chrome bytes.Buffer
	act, err := runCreateMenuWith(d, feedCreatePrograms(t, programs...), &chrome)
	return act, err, &chrome
}

// Menu keys from the initial cursor on create.
var (
	cmBody      = []string{keyUp, keyEnter}
	cmTags      = []string{keyDown, keyEnter}
	cmBlockedBy = []string{keyDown, keyDown, keyEnter}
	cmBlocking  = []string{keyDown, keyDown, keyDown, keyEnter}
	cmStatus    = []string{keyDown, keyDown, keyDown, keyDown, keyEnter}
)

func TestRunCreateMenuEntriesOrderAndCursor(t *testing.T) {
	ciFixture(t)
	d := newCreateDraft([]string{"T"})
	act, err, chrome := cmRun(t, &d, []string{keyEnter})
	if err != nil || act != createMenuCreate {
		t.Fatalf("act = %v, err = %v, want createMenuCreate", act, err)
	}
	view := ciText(chrome.String())
	last := -1
	for _, label := range []string{"go to body", "create", "set tags", "set blocked_by", "set blocking", "set status"} {
		i := strings.Index(view, label)
		if i < 0 || i < last {
			t.Fatalf("entry %q missing or out of order in %q", label, view)
		}
		last = i
	}
	if strings.Contains(view, "create next") {
		t.Error("menu shows create next")
	}
	if !strings.Contains(view, "▸ create") {
		t.Errorf("cursor not on create in %q", view)
	}
}

func TestRunCreateMenuReturnsAfterTags(t *testing.T) {
	ciFixture(t)
	d := newCreateDraft([]string{"T"})
	act, err, _ := cmRun(t, &d, cmTags, []string{"alpha", keyEnter}, []string{keyEnter})
	if err != nil || act != createMenuCreate {
		t.Fatalf("act = %v, err = %v", act, err)
	}
	if !slices.Equal(d.Tags, []string{"alpha"}) {
		t.Errorf("tags = %v", d.Tags)
	}
}

// cmWrite runs the whole interactive core with the form entered by four
// Enters, then the given menu and step programs.
func cmWrite(t *testing.T, programs ...[]string) (*item.Item, []byte, *bytes.Buffer, error) {
	t.Helper()
	form := []string{keyEnter, keyEnter, keyEnter, keyEnter}
	return ciRunRaw(t, []string{"T"}, append([][]string{form}, programs...)...)
}

func TestCreateInteractiveMenuBody(t *testing.T) {
	ciFixture(t)
	got, _, _, err := cmWrite(t, cmBody, []string{"hello body", keyEnter}, []string{keyEnter})
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "hello body" {
		t.Errorf("body = %q", got.Body)
	}
}

func TestCreateInteractiveMenuTagsRelationsStatus(t *testing.T) {
	ciFixture(t)
	if err := core.Create(&item.Item{ID: "pp-x1", Slug: "x", Title: "X", Type: "task", Status: "todo", Tags: []string{"work"}}); err != nil {
		t.Fatal(err)
	}
	def := cfg.GetDefaultStatus()
	statuses := candidates.StatusCandidates(cfg)
	want := ""
	for i, s := range statuses {
		if s.Name == def {
			want = statuses[(i+1)%len(statuses)].Name
		}
	}
	// Candidates sort by type then title: Alpha release, Beta release first.
	got, _, _, err := cmWrite(t,
		cmTags, []string{"work, newtag", keyEnter},
		cmBlockedBy, []string{"x", keyDown, "x", keyEnter},
		cmBlocking, []string{keyDown, keyDown, "x", keyEnter},
		cmStatus, []string{keyDown, keyEnter},
		[]string{keyEnter},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Tags, []string{"work", "newtag"}) {
		t.Errorf("tags = %v", got.Tags)
	}
	if len(got.BlockedBy) != 2 || len(got.Blocking) != 1 {
		t.Errorf("blocked_by = %v, blocking = %v", got.BlockedBy, got.Blocking)
	}
	if got.Status != want {
		t.Errorf("status = %q, want %q", got.Status, want)
	}
	plain, _, _, err := cmWrite(t, []string{keyEnter})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Status != def {
		t.Errorf("status without set status = %q, want %q", plain.Status, def)
	}
}

func TestRunCreateMenuTagValidation(t *testing.T) {
	ciFixture(t)
	d := newCreateDraft([]string{"T"})
	d.Tags = []string{"keep"}
	_, err, chrome := cmRun(t, &d, cmTags, []string{"Bad Tag!", keyEnter, keyCtrlC})
	if err != errCreateAborted {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(ciText(chrome.String()), "invalid tag") {
		t.Error("validation message not shown")
	}
	if !slices.Equal(d.Tags, []string{"keep"}) {
		t.Errorf("tags changed to %v", d.Tags)
	}
}

func TestRunCreateMenuStepsPrefilled(t *testing.T) {
	ciFixture(t)
	d := newCreateDraft([]string{"T"})
	d.Body, d.Tags, d.BlockedBy, d.Blocking = "pre", []string{"a", "b"}, []string{"pp-m2"}, []string{"pp-m1"}
	d.Status = statusOtherThanDefault()
	want := d
	want.Tags, want.BlockedBy, want.Blocking = slices.Clone(d.Tags), slices.Clone(d.BlockedBy), slices.Clone(d.Blocking)
	// Confirm every step untouched: the draft must come out unchanged.
	_, err, _ := cmRun(t, &d,
		cmBody, []string{keyEnter}, cmTags, []string{keyEnter},
		cmBlockedBy, []string{keyEnter}, cmBlocking, []string{keyEnter},
		cmStatus, []string{keyEnter}, []string{keyEnter})
	if err != nil {
		t.Fatal(err)
	}
	if d.Body != want.Body || d.Status != want.Status ||
		!slices.Equal(d.Tags, want.Tags) || !slices.Equal(d.BlockedBy, want.BlockedBy) || !slices.Equal(d.Blocking, want.Blocking) {
		t.Errorf("draft = %+v, want %+v", d, want)
	}
}

func statusOtherThanDefault() string {
	for _, s := range candidates.StatusCandidates(cfg) {
		if s.Name != cfg.GetDefaultStatus() {
			return s.Name
		}
	}
	return ""
}

func TestRunCreateMenuEscInFilterKeepsStepOpen(t *testing.T) {
	ciFixture(t)
	before := len(core.All())
	d := newCreateDraft([]string{"T"})
	// After / alp Esc the step must still take x and Enter; then cancel at the menu.
	_, err, _ := cmRun(t, &d, cmBlockedBy, []string{"/", "alp", keyEsc, "x", keyEnter}, []string{keyCtrlC})
	if err != errCreateAborted {
		t.Fatalf("err = %v", err)
	}
	if !slices.Equal(d.BlockedBy, []string{"pp-m1"}) {
		t.Errorf("blocked_by = %v: step closed on Esc", d.BlockedBy)
	}
	if len(core.All()) != before {
		t.Error("item count changed")
	}
}

func TestRunCreateMenuColoursComeFromFlavour(t *testing.T) {
	withTrueColor(t)
	hexRe := regexp.MustCompile(`\x1b\[[0-9;]*?(?:38|48);2;(\d+);(\d+);(\d+)m`)
	out := map[string]string{}
	for _, flavour := range []string{"mocha", "latte"} {
		t.Run(flavour, func(t *testing.T) {
			ui.SetTheme(flavour)
			t.Cleanup(func() { ui.SetTheme("mocha") })
			ciFixture(t)
			var all strings.Builder
			for _, entry := range [][]string{cmBody, cmTags, cmBlockedBy, cmBlocking, cmStatus} {
				d := newCreateDraft([]string{"T"})
				_, _, chrome := cmRun(t, &d, entry, []string{keyCtrlC})
				all.WriteString(chrome.String())
			}
			out[flavour] = all.String()
			allowed := map[string]bool{}
			for _, hex := range ui.ActiveTheme().Colors {
				allowed[strings.ToLower(hex)] = true
			}
			ms := hexRe.FindAllStringSubmatch(all.String(), -1)
			if len(ms) == 0 {
				t.Fatal("no colour escapes under forced TrueColor")
			}
			for _, m := range ms {
				r, _ := strconv.Atoi(m[1])
				g, _ := strconv.Atoi(m[2])
				b, _ := strconv.Atoi(m[3])
				if !ppNearFlavour(allowed, r, g, b) {
					t.Errorf("colour #%02x%02x%02x is not in flavour %s", r, g, b, flavour)
				}
			}
		})
	}
	if out["mocha"] == out["latte"] {
		t.Error("mocha and latte render identically")
	}
}

func TestRunCreateMenuCancel(t *testing.T) {
	ciFixture(t)
	steps := map[string][]string{"menu": nil, "body": cmBody, "tags": cmTags, "blocked_by": cmBlockedBy, "blocking": cmBlocking, "status": cmStatus}
	for name, entry := range steps {
		t.Run(name, func(t *testing.T) {
			before := len(core.All())
			d := newCreateDraft([]string{"T"})
			programs := [][]string{{keyCtrlC}}
			if entry != nil {
				programs = [][]string{entry, {keyCtrlC}}
			}
			_, err, _ := cmRun(t, &d, programs...)
			if err != errCreateAborted {
				t.Errorf("err = %v, want errCreateAborted", err)
			}
			if len(core.All()) != before {
				t.Error("item count changed")
			}
		})
	}
}
