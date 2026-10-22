package data

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/pkg/item"
)

func openStore(t *testing.T, repo string) *Store {
	t.Helper()
	s, err := Open(repo, "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func get(t *testing.T, s *Store, id string) *item.Item {
	t.Helper()
	it, err := s.Core.Get(id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	return it
}

func TestOpenLoadsAllItems(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	if n := len(s.Items()); n != 3 {
		t.Fatalf("len(Items()) = %d, want 3", n)
	}
	task := get(t, s, "tt-task")
	if task.Parent != "tt-epic" || len(task.BlockedBy) != 1 || task.Body == "" {
		t.Errorf("task fields not loaded: %+v", task)
	}
}

func TestOpenHonorsExplicitStorePath(t *testing.T) {
	repo := newTestRepo(t)
	other := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Open(repo, other, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if n := len(s.Items()); n != 0 {
		t.Fatalf("len(Items()) = %d, want 0 from the empty explicit store", n)
	}
}

func TestOpenHonorsExplicitConfigPath(t *testing.T) {
	repo := newTestRepo(t)
	cfgPath := filepath.Join(t.TempDir(), "other.yml")
	if err := os.WriteFile(cfgPath, []byte("project:\n    name: other\nlif:\n    path: "+filepath.Join(repo, ".lif")+"\n    prefix: tt-\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(t.TempDir(), "", cfgPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if n := len(s.Items()); n != 3 {
		t.Fatalf("len(Items()) = %d, want 3 via --config", n)
	}
}

func TestOpenMissingStoreFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte("project:\n    name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir, "", ""); err == nil {
		s.Close()
		t.Fatal("Open without a store dir: want error")
	}
}

func TestRawReturnsFileOnDisk(t *testing.T) {
	repo := newTestRepo(t)
	s := openStore(t, repo)
	raw, err := s.Raw("tt-task")
	if err != nil {
		t.Fatalf("Raw: %v", err)
	}
	disk, err := os.ReadFile(filepath.Join(repo, ".lif", "tt-task--test-task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(disk) {
		t.Errorf("Raw differs from the file on disk:\n%q\n%q", raw, disk)
	}
	if _, err := s.Raw("nope"); err == nil {
		t.Error("Raw(unknown id): want error")
	}
}

func TestSearchFindsBodyText(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	got, err := s.Search(`"Milestone fixture body"`)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].ID != "tt-mlst" {
		t.Fatalf("Search = %v, want only tt-mlst", got)
	}
}

func TestIsArchivedDetectsPathPrefix(t *testing.T) {
	for path, want := range map[string]bool{
		"tt-mlst--test-milestone.md":         false,
		"archive/tt-mlst--test-milestone.md": true,
		"":                                   false,
		"sub/archive/x.md":                   false,
	} {
		if got := IsArchived(&item.Item{Path: path}); got != want {
			t.Errorf("IsArchived(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestArchivedItemStaysLoadedWithArchivePath(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	task := get(t, s, "tt-task")
	if err := s.SetStatus(task.ID, "completed", task.ETag()); err != nil {
		t.Fatal(err)
	}
	if err := s.Core.Archive(task.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if len(s.Items()) != 3 {
		t.Fatalf("len(Items()) = %d after archive, want 3", len(s.Items()))
	}
	if a := get(t, s, "tt-task"); !IsArchived(a) || a.Status != "completed" {
		t.Errorf("archived item: path %q status %q", a.Path, a.Status)
	}
}

func TestSettersRoundtrip(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	step := func(f func(etag string) error) {
		t.Helper()
		if err := f(get(t, s, "tt-task").ETag()); err != nil {
			t.Fatal(err)
		}
	}
	step(func(e string) error { return s.SetStatus("tt-task", "in-progress", e) })
	step(func(e string) error { return s.SetPriority("tt-task", "high", e) })
	step(func(e string) error { return s.SetTitle("tt-task", "Renamed", e) })
	step(func(e string) error { return s.SetType("tt-task", "bug", e) })
	epic2 := mk(t, s, "Epic 2", "epic")
	step(func(e string) error { return s.SetParent("tt-task", epic2, e) })
	got := get(t, s, "tt-task")
	if got.Status != "in-progress" || got.Priority != "high" || got.Title != "Renamed" || got.Type != "bug" || got.Parent != epic2 {
		t.Fatalf("setters did not land: %+v", got)
	}
	step(func(e string) error { return s.RemoveParent("tt-task", e) })
	if p := get(t, s, "tt-task").Parent; p != "" {
		t.Fatalf("Parent = %q after RemoveParent", p)
	}
}

func TestCreateReturnsNewItem(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	created, err := s.Create(CreateOpts{Title: "--fix login", Type: "task", Parent: "tt-epic", Tags: []string{"x"}, Body: "hello"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.ETag() == "" || created.Title != "--fix login" || created.Parent != "tt-epic" {
		t.Fatalf("created = %+v", created)
	}
	if got := get(t, s, created.ID); got.Body != "hello" || !got.HasTag("x") {
		t.Errorf("on disk: %+v", got)
	}
}

func TestConflictOnStaleETag(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	stale := get(t, s, "tt-task").ETag()
	if err := s.SetTitle("tt-task", "first", stale); err != nil {
		t.Fatal(err)
	}
	err := s.SetStatus("tt-task", "in-progress", stale)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale etag error = %v, want ErrConflict", err)
	}
	if get(t, s, "tt-task").Status != "todo" {
		t.Error("a conflicting write changed the item")
	}
}

func TestValidationErrorIsNotConflict(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	task := get(t, s, "tt-task")
	err := s.SetType(task.ID, "etag mismatch", task.ETag())
	if err == nil || errors.Is(err, ErrConflict) {
		t.Fatalf("invalid type error = %v, want a non-conflict error", err)
	}
}

func TestSetTagsAddsAndRemovesInOneCall(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	epic := get(t, s, "tt-epic")
	if err := s.SetTags(epic.ID, []string{"alpha", "beta"}, []string{"urgent"}, epic.ETag()); err != nil {
		t.Fatal(err)
	}
	got := get(t, s, "tt-epic")
	if !got.HasTag("alpha") || !got.HasTag("beta") || !got.HasTag("backend") || got.HasTag("urgent") {
		t.Errorf("tags = %v", got.Tags)
	}
}

func TestSetBlockingAndCycle(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	other := mk(t, s, "Other", "task")
	epic := get(t, s, "tt-epic")
	if err := s.SetBlocking(epic.ID, []string{other}, nil, epic.ETag()); err != nil {
		t.Fatalf("SetBlocking: %v", err)
	}
	if !get(t, s, "tt-epic").IsBlocking(other) {
		t.Fatal("blocking link missing")
	}
	// tt-task is blocked by tt-mlst, which blocks tt-task: making tt-task block
	// tt-mlst closes a cycle.
	task := get(t, s, "tt-task")
	err := s.SetBlocking(task.ID, []string{"tt-mlst"}, nil, task.ETag())
	if err == nil || !strings.Contains(err.Error(), "cycle") || errors.Is(err, ErrConflict) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestUpdateWholeAppliesOnlyChangedFields(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	task := get(t, s, "tt-task")

	// Empty diff: no write, so the etag does not move.
	if err := s.UpdateWhole(task.ID, WholeEditDiff{}, task.ETag()); err != nil {
		t.Fatalf("empty diff: %v", err)
	}
	if get(t, s, "tt-task").ETag() != task.ETag() {
		t.Fatal("empty diff rewrote the item")
	}

	title, body := "Whole", "new body\n"
	epic2, other := mk(t, s, "Epic 2", "epic"), mk(t, s, "Other", "task")
	diff := WholeEditDiff{
		Title: &title, Body: &body,
		TagsAdd:         []string{"t1"},
		BlockedByAdd:    []string{other},
		BlockedByRemove: []string{"tt-mlst"},
		ParentChanged:   true, Parent: epic2,
	}
	if err := s.UpdateWhole(task.ID, diff, task.ETag()); err != nil {
		t.Fatalf("UpdateWhole: %v", err)
	}
	got := get(t, s, "tt-task")
	if got.Title != title || strings.TrimSpace(got.Body) != "new body" || !got.HasTag("t1") || got.Parent != epic2 ||
		!got.IsBlockedBy(other) || got.IsBlockedBy("tt-mlst") {
		t.Fatalf("whole update did not land: %+v", got)
	}
	if got.Status != task.Status || got.Priority != task.Priority || got.Type != task.Type {
		t.Fatalf("untouched fields changed: %+v", got)
	}
}

func TestDeleteClearsReferences(t *testing.T) {
	s := openStore(t, newTestRepo(t))
	if err := s.Delete("tt-epic"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Core.Get("tt-epic"); err == nil {
		t.Fatal("deleted item still loaded")
	}
	if p := get(t, s, "tt-task").Parent; p != "" {
		t.Errorf("former child Parent = %q, want cleared", p)
	}
	if err := s.Delete("tt-mlst"); err != nil {
		t.Fatal(err)
	}
	if bb := get(t, s, "tt-task").BlockedBy; len(bb) != 0 {
		t.Errorf("BlockedBy = %v, want cleared", bb)
	}
}

func TestWatchReportsExternalChange(t *testing.T) {
	repo := newTestRepo(t)
	s := openStore(t, repo)
	ch, err := s.Watch()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	path := filepath.Join(repo, ".lif", "tt-task--test-task.md")
	b, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), "Test Task", "Changed Outside", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no event within 5s of an external write")
	}
	if got := get(t, s, "tt-task").Title; got != "Changed Outside" {
		t.Errorf("Title = %q", got)
	}
}

func TestCloseIsIdempotentAndClosesWatchChannel(t *testing.T) {
	s, err := Open(newTestRepo(t), "", "")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Watch()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("event after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("watch channel not closed by Close")
	}
}

func TestOpenPerformanceAt100Items(t *testing.T) {
	dir := newTestRepoN(t, 100)
	start := time.Now()
	s, err := Open(dir, "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	n, elapsed := len(s.Items()), time.Since(start)
	t.Logf("Open + Items @ 100 items: %v", elapsed)
	if n != 100 || elapsed > 2*time.Second {
		t.Fatalf("Items() = %d in %v, want 100 in <2s", n, elapsed)
	}
}

// hookRepo is a repo whose config carries hooks; trust is granted for the
// config as written.
func hookRepo(t *testing.T, hooksYAML string) *Store {
	t.Helper()
	t.Setenv("LIF_NO_HOOKS", "")
	t.Setenv("HOME", t.TempDir())
	repo := newTestRepo(t)
	cfgFile := filepath.Join(repo, ".lif.yml")
	if err := os.WriteFile(cfgFile, []byte(testLifYML+hooksYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	s := openStore(t, repo)
	if err := hooks.Allow(hooks.Key(s.Config().ConfigFile()), hooks.Fingerprint(s.Config())); err != nil {
		t.Fatalf("hooks.Allow: %v", err)
	}
	return s
}

func TestGateBlockSurfacesHookName(t *testing.T) {
	s := hookRepo(t, `
tasks:
  deny:
    steps:
      - 'echo "no completing here"; exit 1'
hooks:
  block-done:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: deny
    message: "Completing is gated."
`)
	task := get(t, s, "tt-task")
	err := s.SetStatus(task.ID, "completed", task.ETag())
	if err == nil {
		t.Fatal("gate did not block")
	}
	for _, want := range []string{`hook "block-done" blocked`, "Completing is gated.", "no completing here"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if get(t, s, "tt-task").Status != "todo" {
		t.Error("blocked change reached the disk")
	}
}

func TestOpenWarningsDoNotReachStderr(t *testing.T) {
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() { os.Stderr = orig })

	s := hookRepo(t, `
tasks:
  boom:
    steps:
      - 'echo after-hook-output; exit 1'
hooks:
  on-update:
    event: [updated]
    run: boom
`)
	task := get(t, s, "tt-task")
	if err := s.SetStatus(task.ID, "in-progress", task.ETag()); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	os.Stderr = orig

	if b, _ := os.ReadFile(stderr.Name()); len(b) != 0 {
		t.Fatalf("store wrote to os.Stderr: %q", b)
	}
	if w := s.DrainWarnings(); !strings.Contains(w, "after-hook-output") && !strings.Contains(w, "on-update") {
		t.Fatalf("DrainWarnings = %q, want the failed after hook", w)
	}
	if w := s.DrainWarnings(); w != "" {
		t.Fatalf("second DrainWarnings = %q, want empty", w)
	}
}

// mk creates an unparented item and returns its ID.
func mk(t *testing.T, s *Store, title, typ string) string {
	t.Helper()
	it, err := s.Create(CreateOpts{Title: title, Type: typ})
	if err != nil {
		t.Fatalf("Create(%s): %v", title, err)
	}
	return it.ID
}
