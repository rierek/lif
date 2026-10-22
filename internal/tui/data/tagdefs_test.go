package data

// tagdefs_test.go — TDD coverage for the Tag-Registry persistence layer
// (E10 Task 1, bean bt-49hh, epic bt-362n D01-D04). Mirrors
// internal/config/settings_test.go's naming/structure convention
// (TestLoadSettingsMissingFileReturnsDefaults etc.) -- LoadTagDefs/
// SaveTagDefs are the repo-local, tolerant-missing analogue of
// config.LoadSettings/SaveUserSettings.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadTagDefsMissingFileReturnsEmpty(t *testing.T) {
	c := &Store{RepoDir: t.TempDir()}
	got, err := c.LoadTagDefs()
	if err != nil || len(got) != 0 {
		t.Fatalf("want (nil, nil), got (%v, %v)", got, err)
	}
}

// TestLoadTagDefsSkipsInvalidNamesDefensively (T1-Review F03, bean bt-49hh,
// 2026-07-16, PRELUDE at bt-r92i): the ORIGINAL fixture's third entry was a
// bare blank scalar (`  - ` with nothing after the dash) -- empirically
// verified (throwaway yaml.v3 probe against this exact struct shape) that
// yaml.v3 drops a bare blank list scalar BEFORE Unmarshal ever populates
// f.Tags: len(f.Tags) == 2 for that fixture, never 3. The test therefore
// only ever proved the Bad_Tag (invalid-charset) filter branch -- it never
// actually exercised ValidTagName("") / the empty-string filter path at all,
// despite the doc-comment-adjacent intent. Fixed here: an EXPLICIT `- ""`
// scalar (quoted) DOES survive Unmarshal as a genuine "" element (verified
// by the same probe: len(f.Tags) == 3, tags[2] == ""), so it now reaches
// LoadTagDefs' ValidTagName filter loop for real -- ValidTagName("") is
// false (tagNameRe requires a leading `[a-z]`), so "" is still dropped and
// the assertion below (want exactly [good-tag]) is unchanged.
func TestLoadTagDefsSkipsInvalidNamesDefensively(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".lif-tags.yml"),
		[]byte("tags:\n  - good-tag\n  - Bad_Tag\n  - \"\"\n"), 0o644)
	c := &Store{RepoDir: dir}
	got, _ := c.LoadTagDefs()
	if len(got) != 1 || got[0] != "good-tag" {
		t.Fatalf("want [good-tag], got %v", got)
	}
}

func TestSaveTagDefsRoundTripSortedDeduped(t *testing.T) {
	dir := t.TempDir()
	c := &Store{RepoDir: dir}
	if err := c.SaveTagDefs([]string{"zeta", "alpha", "alpha"}); err != nil {
		t.Fatal(err)
	}
	got, _ := c.LoadTagDefs()
	want := []string{"alpha", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestRenameTagDefNamePromotesUnregisteredOldName(t *testing.T) {
	got := RenameTagDefName([]string{"a", "b"}, "c-not-registered", "d")
	want := []string{"a", "b", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// TestLoadTagDefsCorruptYAMLReturnsEmptyNoPanic -- corrupt YAML must never
// crash or error out, same "never crash" philosophy as config.LoadSettings.
func TestLoadTagDefsCorruptYAMLReturnsEmptyNoPanic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif-tags.yml"),
		[]byte("tags: [this is not: valid: yaml: at: all"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Store{RepoDir: dir}
	got, err := c.LoadTagDefs()
	if err != nil {
		t.Fatalf("want nil error on corrupt YAML, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty registry on corrupt YAML, got %v", got)
	}
}

// TestSaveTagDefsWritesReadablePermissions verifies the 0o644 mode and that
// the written file round-trips through LoadTagDefs as valid YAML.
func TestSaveTagDefsWritesReadablePermissions(t *testing.T) {
	dir := t.TempDir()
	c := &Store{RepoDir: dir}
	if err := c.SaveTagDefs([]string{"solo"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, ".lif-tags.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("want mode 0o644, got %v", fi.Mode().Perm())
	}
}

func TestAddTagDefNameNoOpOnDuplicate(t *testing.T) {
	got := AddTagDefName([]string{"alpha", "beta"}, "alpha")
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestAddTagDefNameInsertsSortedDeduped(t *testing.T) {
	got := AddTagDefName([]string{"alpha", "zeta"}, "mu")
	want := []string{"alpha", "mu", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestRemoveTagDefNameNoOpOnAbsence(t *testing.T) {
	got := RemoveTagDefName([]string{"alpha", "beta"}, "gamma")
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestRemoveTagDefNameRemovesExisting(t *testing.T) {
	got := RemoveTagDefName([]string{"alpha", "beta", "gamma"}, "beta")
	want := []string{"alpha", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestRenameTagDefNameRenamesExisting(t *testing.T) {
	got := RenameTagDefName([]string{"alpha", "beta"}, "alpha", "delta")
	want := []string{"beta", "delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// TestAddRemoveRenameDoNotMutateInputSlice -- acceptance requires "keine
// Mutation des Eingabe-Slices — neue Slice zurückgeben".
func TestAddRemoveRenameDoNotMutateInputSlice(t *testing.T) {
	orig := []string{"beta", "alpha"}
	snapshot := append([]string(nil), orig...)

	AddTagDefName(orig, "gamma")
	if !reflect.DeepEqual(orig, snapshot) {
		t.Fatalf("AddTagDefName mutated input: got %v, want %v", orig, snapshot)
	}

	RemoveTagDefName(orig, "alpha")
	if !reflect.DeepEqual(orig, snapshot) {
		t.Fatalf("RemoveTagDefName mutated input: got %v, want %v", orig, snapshot)
	}

	RenameTagDefName(orig, "alpha", "delta")
	if !reflect.DeepEqual(orig, snapshot) {
		t.Fatalf("RenameTagDefName mutated input: got %v, want %v", orig, snapshot)
	}
}

// A repo that still carries .beans-tags.yml (and no .lif-tags.yml) is read and
// saved in place; no second file appears.
func TestTagDefsLegacyFileIsUsedInPlace(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, ".beans-tags.yml")
	if err := os.WriteFile(legacy, []byte("tags:\n  - alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Store{RepoDir: dir}

	got, err := c.LoadTagDefs()
	if err != nil || !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("LoadTagDefs() = (%v, %v), want [alpha]", got, err)
	}
	if err := c.SaveTagDefs([]string{"alpha", "beta"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lif-tags.yml")); err == nil {
		t.Fatal("SaveTagDefs created .lif-tags.yml next to the legacy file")
	}
	got, err = c.LoadTagDefs()
	if err != nil || !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("after save LoadTagDefs() = (%v, %v), want [alpha beta]", got, err)
	}
}
