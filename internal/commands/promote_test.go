package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/pflag"
)

// setupPromoteTest installs a throwaway core and default config into the
// package globals promoteCmd.RunE reads, mirroring setupCompleteTest.
func setupPromoteTest(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	testCfg := config.Default()
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })
}

// resetPromoteFlags clears promoteJSON and promoteParent before/after each test.
func resetPromoteFlags(t *testing.T) {
	t.Helper()
	oldJSON, oldParent := promoteJSON, promoteParent
	promoteJSON, promoteParent = false, ""
	t.Cleanup(func() { promoteJSON, promoteParent = oldJSON, oldParent })
}

// captureRunEStdout redirects os.Stdout for the duration of fn and returns
// everything written to it, mirroring captureCompleteStdout.
func captureRunEStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
		w.Close()
	}()
	fn()
	w.Close()

	out := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}
	return out
}

// writeArtifact writes a findings artifact document to a temp file and
// returns its path.
func writeArtifact(t *testing.T, doc string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "findings.json")
	if err := os.WriteFile(path, []byte(doc), 0644); err != nil {
		t.Fatalf("writing artifact: %v", err)
	}
	return path
}

// validBugRecord and validImprovementRecord are canonical, schema-valid
// records used across tests; each test embeds them in its own artifact
// document so mutations to one test's fixture never affect another.
const validBugRecord = `{
	"id": "B01", "code": "B", "title": "Missing axis on legacy record",
	"axis": "standards", "evidence": ["S02", "S03"],
	"blocking": true, "criterion": "acceptance-criteria-violation",
	"fields": {
		"Schwere": "hoch",
		"Beschreibung": "Record has no axis field.",
		"Empfehlung": "Reject at validation time."
	}
}`

const validImprovementRecord = `{
	"id": "I02", "code": "I", "title": "Group renderings by axis",
	"axis": "spec", "evidence": ["S04"],
	"fields": {
		"Beschreibung": "Renderings should group by axis.",
		"Nutzen": "Readers see standards vs spec findings separately.",
		"Empfehlung": "Add a group-by-axis rendering step."
	}
}`

// malformedBugRecord (B03) is missing the required "Beschreibung" key inside
// fields -- a schema violation used by SC-01/AC4/AC5.
const malformedBugRecord = `{
	"id": "B03", "code": "B", "title": "Malformed record",
	"axis": "standards", "evidence": [],
	"fields": {
		"Schwere": "niedrig",
		"Empfehlung": "n/a"
	}
}`

func countItems(t *testing.T) int {
	t.Helper()
	return len(core.All())
}

// AC1/AC2: promote requires the artifact path plus at least one finding-id.
func TestPromoteRequiresArtifactAndFindingID(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)

	if err := promoteCmd.Args(promoteCmd, []string{"artifact.json"}); err == nil {
		t.Fatal("expected an error with only the artifact argument, got nil")
	}
	if err := promoteCmd.Args(promoteCmd, []string{}); err == nil {
		t.Fatal("expected an error with zero arguments, got nil")
	}
	if err := promoteCmd.Args(promoteCmd, []string{"artifact.json", "B01"}); err != nil {
		t.Fatalf("expected artifact+one finding-id to pass Args, got: %v", err)
	}
}

// SC-05/AC15: repeating the same finding-id within one call is rejected
// before any write.
func TestPromoteRejectsDuplicateFindingIDInSameCall(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	before := countItems(t)
	err := promoteCmd.RunE(promoteCmd, []string{path, "B01", "B01"})
	if err == nil {
		t.Fatal("expected an error for a repeated finding-id, got nil")
	}
	if got := countItems(t); got != before {
		t.Errorf("beans written = %d, want %d (zero writes on rejection)", got, before)
	}
}

// SC-01/AC4/AC5: a malformed selected record is rejected by ID and field,
// zero items are written, and an unselected malformed record is never
// evaluated.
func TestPromoteSchemaValidationRejectsMalformedSelectedRecord(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	// A second malformed record (B04) is present in the artifact but never
	// selected -- it must not be validated, so it must not appear in any
	// error and must not block promotion of B01/B03.
	unselectedMalformed := `{
		"id": "B04", "code": "B", "title": "Unselected malformed record",
		"axis": "standards", "evidence": [],
		"fields": { "Empfehlung": "n/a" }
	}`
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`,`+malformedBugRecord+`,`+unselectedMalformed+`]}`)

	before := countItems(t)
	err := promoteCmd.RunE(promoteCmd, []string{path, "B01", "B03"})
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	if got := countItems(t); got != before {
		t.Errorf("beans written = %d, want %d (zero writes on preflight failure)", got, before)
	}
	if !contains(err.Error(), "B03") {
		t.Errorf("error %q does not name the offending record B03", err.Error())
	}
	if !contains(err.Error(), "Beschreibung") {
		t.Errorf("error %q does not name the offending field Beschreibung", err.Error())
	}
	if contains(err.Error(), "B04") {
		t.Errorf("error %q mentions unselected record B04, which must never be validated", err.Error())
	}
}

// SC-02: a valid B record is promoted into a bug item carrying review,
// finding and severity front matter, no priority, evidence/body transferred
// unchanged, no Acceptance section, and a Provenance section.
func TestPromoteBugRecordSucceeds(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
		t.Fatalf("promoteCmd.RunE() error = %v", err)
	}

	items := core.All()
	if len(items) != 1 {
		t.Fatalf("beans created = %d, want 1", len(items))
	}
	b := items[0]

	if b.Type != "bug" {
		t.Errorf("Type = %q, want %q", b.Type, "bug")
	}
	if b.Priority != "" {
		t.Errorf("Priority = %q, want empty (no priority set)", b.Priority)
	}
	if got := b.Extra["review"]; got != path {
		t.Errorf("Extra[review] = %v, want %q", got, path)
	}
	if got := b.Extra["finding"]; got != "B01" {
		t.Errorf("Extra[finding] = %v, want %q", got, "B01")
	}
	if got := b.Extra["axis"]; got != "standards" {
		t.Errorf("Extra[axis] = %v, want %q", got, "standards")
	}
	if got := b.Extra["severity"]; got != "hoch" {
		t.Errorf("Extra[severity] = %v, want %q", got, "hoch")
	}
	if !contains(b.Body, "S02") || !contains(b.Body, "S03") {
		t.Errorf("body %q does not carry evidence S02/S03 unchanged", b.Body)
	}
	if !contains(b.Body, "Record has no axis field.") {
		t.Errorf("body %q does not carry Beschreibung unchanged", b.Body)
	}
	if !contains(b.Body, "Reject at validation time.") {
		t.Errorf("body %q does not carry Empfehlung unchanged", b.Body)
	}
	if contains(b.Body, "## Acceptance") {
		t.Errorf("body %q must not contain an Acceptance section", b.Body)
	}
	if !contains(b.Body, "## Provenance") || !contains(b.Body, path) || !contains(b.Body, "B01") {
		t.Errorf("body %q must contain a Provenance section naming the artifact and finding id", b.Body)
	}
}

// SC-03: a valid I record is promoted into a task item carrying the
// "improvement" tag and no severity key.
func TestPromoteImprovementRecordSucceeds(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validImprovementRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "I02"}); err != nil {
		t.Fatalf("promoteCmd.RunE() error = %v", err)
	}

	items := core.All()
	if len(items) != 1 {
		t.Fatalf("beans created = %d, want 1", len(items))
	}
	b := items[0]

	if b.Type != "task" {
		t.Errorf("Type = %q, want %q", b.Type, "task")
	}
	if !b.HasTag("improvement") {
		t.Errorf("Tags = %v, want to contain %q", b.Tags, "improvement")
	}
	if _, ok := b.Extra["severity"]; ok {
		t.Errorf("Extra[severity] = %v, want absent for an I record", b.Extra["severity"])
	}
	if !contains(b.Body, "Readers see standards vs spec findings separately.") {
		t.Errorf("body %q does not carry Nutzen unchanged", b.Body)
	}
}

// SC-04/AC11: re-promoting an already-promoted finding is a no-op that
// creates no second item.
func TestPromoteSkipsAlreadyPromotedFinding(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
		t.Fatalf("first promoteCmd.RunE() error = %v", err)
	}
	if got := countItems(t); got != 1 {
		t.Fatalf("beans after first call = %d, want 1", got)
	}

	out := captureRunEStdout(t, func() {
		if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
			t.Fatalf("second promoteCmd.RunE() error = %v", err)
		}
	})
	if got := countItems(t); got != 1 {
		t.Errorf("beans after repeated call = %d, want 1 (no duplicate)", got)
	}
	if !contains(string(out), "B01") {
		t.Errorf("text output %q does not report B01 as skipped", out)
	}
}

// beans-z69m: a record that fails the create-time validation (here an
// overlong title) is caught in the preflight, so a good record queued before
// it is not written either.
func TestPromoteInvalidCandidateWritesNothing(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	overlongTitle := strings.Repeat("x", 1001)
	secondBug := `{
		"id": "B02", "code": "B", "title": "` + overlongTitle + `",
		"axis": "standards", "evidence": ["S05"],
		"fields": {
			"Schwere": "mittel",
			"Beschreibung": "Second description.",
			"Empfehlung": "Second recommendation."
		}
	}`
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`,`+secondBug+`]}`)

	err := promoteCmd.RunE(promoteCmd, []string{path, "B01", "B02"})
	if err == nil {
		t.Fatal("expected a preflight failure, got nil")
	}
	if got := countItems(t); got != 0 {
		t.Fatalf("items created = %d, want 0 (nothing written when one candidate is invalid)", got)
	}
	if !contains(err.Error(), "B02") {
		t.Errorf("error %q does not name the invalid record B02", err.Error())
	}
}

// beans-z69m: a schema-valid record whose code has no promotion mapping (D)
// used to fail only after earlier records were written.
func TestPromoteUnmappedCodeWritesNothing(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	decision := `{
		"id": "D01", "code": "D", "title": "A decision",
		"axis": "spec", "evidence": [],
		"fields": {"Hintergrund": "h", "Entscheidung": "e", "Empfehlung": "r"}
	}`
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`,`+decision+`]}`)

	err := promoteCmd.RunE(promoteCmd, []string{path, "B01", "D01"})
	if err == nil {
		t.Fatal("expected a preflight failure, got nil")
	}
	if got := countItems(t); got != 0 {
		t.Fatalf("items created = %d, want 0", got)
	}
}

// beans-z69m: promoted items get the configured default status.
func TestPromoteAppliesDefaultStatus(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
		t.Fatalf("promoteCmd.RunE() error = %v", err)
	}
	all := core.All()
	if len(all) != 1 {
		t.Fatalf("items = %d, want 1", len(all))
	}
	if want := cfg.GetDefaultStatus(); want == "" || all[0].Status != want {
		t.Errorf("status = %q, want configured default %q", all[0].Status, want)
	}
}

// beans-tgph: --parent attaches every promoted item to the given item.
func TestPromoteParentFlagSetsParent(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	if err := core.Create(&item.Item{ID: "feat1", Title: "Feature", Type: "feature", Status: "todo"}); err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	promoteParent = "feat1"
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`,`+validImprovementRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01", "I02"}); err != nil {
		t.Fatalf("promoteCmd.RunE() error = %v", err)
	}
	promoted := 0
	for _, b := range core.All() {
		if b.ID == "feat1" {
			continue
		}
		promoted++
		if b.Parent != "feat1" {
			t.Errorf("item %s parent = %q, want feat1", b.ID, b.Parent)
		}
	}
	if promoted != 2 {
		t.Errorf("promoted items = %d, want 2", promoted)
	}
}

// beans-tgph: an unknown parent is rejected before anything is written.
func TestPromoteUnknownParentWritesNothing(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	promoteParent = "nope"
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err == nil {
		t.Fatal("expected an error for an unknown parent, got nil")
	}
	if got := countItems(t); got != 0 {
		t.Fatalf("items created = %d, want 0", got)
	}
}

// beans-tgph: a parent of a type that cannot parent the promoted type is
// rejected before anything is written.
func TestPromoteInvalidParentTypeWritesNothing(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	if err := core.Create(&item.Item{ID: "leaf1", Title: "Leaf", Type: "bug", Status: "todo"}); err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	promoteParent = "leaf1"
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err == nil {
		t.Fatal("expected an error for a bug as parent of a bug, got nil")
	}
	if got := countItems(t); got != 1 {
		t.Fatalf("items = %d, want 1 (only the parent)", got)
	}
}

// R-27/Risks: the idempotency skip is keyed on the review+finding PAIR, never
// on finding alone -- finding-ids restart at 01 per report, so the same
// value recurs across unrelated artifacts. Promoting the same finding-id
// from two different artifacts must create two independent items, not
// silently skip the second as "already promoted".
func TestPromoteCrossArtifactIdempotencyKeyedOnReviewAndFinding(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	firstArtifact := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)
	secondArtifact := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{firstArtifact, "B01"}); err != nil {
		t.Fatalf("promoting B01 from artifact 1: %v", err)
	}
	if err := promoteCmd.RunE(promoteCmd, []string{secondArtifact, "B01"}); err != nil {
		t.Fatalf("promoting B01 from artifact 2: %v", err)
	}

	if got := countItems(t); got != 2 {
		t.Fatalf("beans created = %d, want 2 (B01 from two different artifacts must not be deduplicated)", got)
	}
	fromFirst := filterByWhere(core.All(), []string{"review=" + firstArtifact, "finding=B01"})
	if len(fromFirst) != 1 {
		t.Errorf("bean for B01 from artifact 1 (%s) missing", firstArtifact)
	}
	fromSecond := filterByWhere(core.All(), []string{"review=" + secondArtifact, "finding=B01"})
	if len(fromSecond) != 1 {
		t.Errorf("bean for B01 from artifact 2 (%s) missing", secondArtifact)
	}
}

// AC2/R-20: an unselected finding-id argument is rejected via ErrNotFound,
// naming the id, without touching the store.
func TestPromoteRejectsUnknownFindingID(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	before := countItems(t)
	err := promoteCmd.RunE(promoteCmd, []string{path, "B99"})
	if err == nil {
		t.Fatal("expected an error for an unknown finding id, got nil")
	}
	if !contains(err.Error(), "B99") {
		t.Errorf("error for unknown finding id %q does not name it", err.Error())
	}
	if got := countItems(t); got != before {
		t.Errorf("beans written = %d, want %d (zero writes for an unknown finding id)", got, before)
	}
}

// AC3: a successful partial promote (only some finding-ids named) never
// edits the artifact file, and the unselected record is not itself
// promoted -- "without requiring the artifact file to be edited" is a
// guarantee about the artifact's own bytes, which the AC5 error-path tests
// never exercise since they never reach a successful write.
func TestPromoteSuccessLeavesArtifactFileUntouched(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	doc := `{"findings":[` + validBugRecord + `,` + validImprovementRecord + `]}`
	path := writeArtifact(t, doc)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading artifact before promote: %v", err)
	}

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
		t.Fatalf("promoteCmd.RunE() error = %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading artifact after promote: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("artifact file %s was modified by promote; before=%q after=%q", path, before, after)
	}

	matches := filterByWhere(core.All(), []string{"finding=I02"})
	if len(matches) != 0 {
		t.Errorf("unselected record I02 was promoted (%d beans), want 0", len(matches))
	}
	if got := countItems(t); got != 1 {
		t.Errorf("beans created = %d, want 1 (only the selected B01)", got)
	}
}

// SC-06: JSON output shape matches the established batch convention -- a
// single finding-id gives a bare item, several give a bare array, and a
// preflight rejection returns the standard error envelope.
func TestPromoteJSONShape(t *testing.T) {
	t.Run("single gives a bare bean", func(t *testing.T) {
		setupPromoteTest(t)
		resetPromoteFlags(t)
		promoteJSON = true
		path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

		out := captureRunEStdout(t, func() {
			if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
				t.Fatalf("promoteCmd.RunE() error = %v", err)
			}
		})

		var got struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON: %v; output = %s", err, out)
		}
		if got.ID == "" {
			t.Errorf("bare bean id empty; output = %s", out)
		}
	})

	t.Run("multiple gives a bare array", func(t *testing.T) {
		setupPromoteTest(t)
		resetPromoteFlags(t)
		promoteJSON = true
		path := writeArtifact(t, `{"findings":[`+validBugRecord+`,`+validImprovementRecord+`]}`)

		out := captureRunEStdout(t, func() {
			if err := promoteCmd.RunE(promoteCmd, []string{path, "B01", "I02"}); err != nil {
				t.Fatalf("promoteCmd.RunE() error = %v", err)
			}
		})

		var got []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON as a bare array: %v; output = %s", err, out)
		}
		if len(got) != 2 {
			t.Errorf("array len = %d, want 2; output = %s", len(got), out)
		}
	})

	t.Run("preflight rejection returns the standard envelope", func(t *testing.T) {
		setupPromoteTest(t)
		resetPromoteFlags(t)
		promoteJSON = true
		path := writeArtifact(t, `{"findings":[`+malformedBugRecord+`]}`)

		out := captureRunEStdout(t, func() {
			err := promoteCmd.RunE(promoteCmd, []string{path, "B03"})
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
		})

		var got struct {
			Success bool   `json:"success"`
			Code    string `json:"code"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON envelope: %v; output = %s", err, out)
		}
		if got.Success {
			t.Errorf("Success = true, want false")
		}
		if got.Code == "" {
			t.Errorf("Code empty, want an output.Err* code")
		}
	})

	t.Run("unknown finding id returns the NOT_FOUND envelope", func(t *testing.T) {
		setupPromoteTest(t)
		resetPromoteFlags(t)
		promoteJSON = true
		path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

		out := captureRunEStdout(t, func() {
			err := promoteCmd.RunE(promoteCmd, []string{path, "B99"})
			if err == nil {
				t.Fatal("expected a not-found error, got nil")
			}
		})

		var got struct {
			Success bool   `json:"success"`
			Code    string `json:"code"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON envelope: %v; output = %s", err, out)
		}
		if got.Success {
			t.Errorf("Success = true, want false")
		}
		if got.Code != output.ErrNotFound {
			t.Errorf("Code = %q, want %q", got.Code, output.ErrNotFound)
		}
	})
}

// SC-07/AC16: the record's review axis is written as extra front matter,
// grouping promoted items by axis.
func TestPromoteSetsAxisFrontMatter(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)
	path := writeArtifact(t, `{"findings":[`+validBugRecord+`]}`)

	if err := promoteCmd.RunE(promoteCmd, []string{path, "B01"}); err != nil {
		t.Fatalf("promoteCmd.RunE() error = %v", err)
	}

	items := core.All()
	if len(items) != 1 {
		t.Fatalf("beans created = %d, want 1", len(items))
	}
	if got := items[0].Extra["axis"]; got != "standards" {
		t.Errorf("Extra[axis] = %v, want %q", got, "standards")
	}
	matching := filterByWhere(core.All(), []string{"axis=standards"})
	if len(matching) != 1 {
		t.Errorf("filterByWhere(axis=standards) = %d beans, want 1", len(matching))
	}
}

// AC14: every flag name, help string and template string promote
// introduces is ASCII English -- no German field labels (Beschreibung,
// Empfehlung, Nutzen, Schwere) leak into command-facing text. Fixed record
// field values legitimately contain German (they are WP1's data, not
// promote's own vocabulary) so this checks the command metadata only, not
// item bodies.
func TestPromoteVocabularyIsEnglish(t *testing.T) {
	sharedTestRoot(t)
	forbidden := []string{"Beschreibung", "Empfehlung", "Nutzen", "Schwere", "Befund"}
	haystacks := []string{promoteCmd.Use, promoteCmd.Short, promoteCmd.Long}
	promoteCmd.Flags().VisitAll(func(f *pflag.Flag) {
		haystacks = append(haystacks, f.Name, f.Usage)
	})
	for _, h := range haystacks {
		for _, word := range forbidden {
			if contains(h, word) {
				t.Errorf("command text %q contains non-English vocabulary %q", h, word)
			}
		}
	}
	if got := promoteCmd.Flags().Lookup("json"); got == nil {
		t.Fatal("expected a --json flag")
	}
}

// AC1: promote is the exclusive CLI surface, reachable through the real root
// command -- every other test in this file calls promoteCmd.RunE directly,
// which never touches register.go's RegisterPromoteCmd(root) call and so
// cannot detect that line going missing. This test drives sharedTestRoot's
// real ExecuteC path instead (the same helper error_shape_test.go's wiring
// tests use), so removing RegisterPromoteCmd(root) from register.go turns
// this test red without touching command behavior, which stays covered by
// the RunE-based tests above.
func TestPromoteReachableThroughRootCommand(t *testing.T) {
	setupPromoteTest(t)
	resetPromoteFlags(t)

	_, stderr, err := runRootWithArgs(t, "promote", "/nonexistent/artifact.json", "B01")
	if err == nil {
		t.Fatal("expected an error: the artifact path does not exist")
	}
	if strings.Contains(stderr, "unknown command") {
		t.Fatalf("command \"promote\" is not reachable through the root command: %s", stderr)
	}
	if !strings.Contains(stderr, "findings artifact") {
		t.Errorf("root command did not reach promote's own RunE (expected the artifact-read error, got): %s", stderr)
	}
}
