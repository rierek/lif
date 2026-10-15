package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// setupE2ETriageTest installs a throwaway core and default config into the
// package globals promoteCmd/listCmd.RunE read, mirroring setupPromoteTest
// and setupListTest. The store lives entirely under tmpDir/.beans and
// core/cfg point straight at it -- RunE never resolves config via cwd or
// --lif-path, so the store is driven from inside its own directory by
// construction (Constraints/AC-01), never through the cwd-resolution path
// that corrupted this hull's real .beans.yml earlier in this wave.
func setupE2ETriageTest(t *testing.T) (tmpDir, itemsDir string) {
	t.Helper()
	tmpDir = t.TempDir()
	itemsDir = filepath.Join(tmpDir, ".beans")
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
	return tmpDir, itemsDir
}

// TestEndToEndTriageControl is beans-w5hu's proof that WP1's schema, WP2's
// promote verb and WP3's artifact shape compose into the triage control the
// container promises. One throwaway store carries the whole sequence in
// order: a reviewer-shaped artifact goes in (AC-02), an explicit promote
// call selects one of three findings (AC-03/AC-04), the store answers what
// became of the review (AC-05), the rejected pair leaves no trace anywhere
// in the store (AC-06), a repeat of the same call is a no-op (AC-07), a
// malformed record is rejected by name (AC-08), and the recorded artifact
// path still resolves to the untouched original file (AC-09).
func TestEndToEndTriageControl(t *testing.T) {
	_, itemsDir := setupE2ETriageTest(t)
	resetPromoteFlags(t)
	resetListWhereFlag(t)
	oldListJSON, oldListFull := listJSON, listFull
	listJSON, listFull = true, true
	t.Cleanup(func() { listJSON, listFull = oldListJSON, oldListFull })
	oldListView := listView
	listView = "table"
	t.Cleanup(func() { listView = oldListView })

	// The item under review. Its existence is not required by the
	// attachments mechanics, but it grounds <reviewed-item-id> in something
	// real rather than a bare literal.
	reviewed := &item.Item{
		ID:     "beans-revd",
		Slug:   item.Slugify("Reviewed leaf"),
		Title:  "Reviewed leaf",
		Status: "todo",
		Type:   "task",
	}
	if err := core.Create(reviewed); err != nil {
		t.Fatalf("creating reviewed bean: %v", err)
	}

	// AC-02: write the findings artifact -- one B (blocking, closed
	// criterion, evidence) and two I (non-blocking), each with a populated
	// evidence field -- at .beans/attachments/<reviewed-item-id>/.
	attachDir := filepath.Join(itemsDir, itemcore.AttachmentsDir, reviewed.ID)
	if err := os.MkdirAll(attachDir, 0755); err != nil {
		t.Fatalf("creating attachments dir: %v", err)
	}
	artifactPath := filepath.Join(attachDir, "findings.json")
	artifactDoc := `{"findings":[
		{
			"id": "B01", "code": "B", "title": "Missing nil guard on batch path",
			"axis": "standards", "evidence": ["S01"],
			"blocking": true, "criterion": "acceptance-criteria-violation",
			"fields": {
				"Schwere": "hoch",
				"Beschreibung": "Batch path dereferences without a nil check.",
				"Empfehlung": "Add the guard before dereferencing."
			}
		},
		{
			"id": "I01", "code": "I", "title": "Extract shared preflight helper",
			"axis": "standards", "evidence": ["S02"],
			"fields": {
				"Beschreibung": "Preflight logic is duplicated across two verbs.",
				"Nutzen": "One helper removes the duplication.",
				"Empfehlung": "Extract a resolveBatchTargets-style helper."
			}
		},
		{
			"id": "I02", "code": "I", "title": "Group renderings by axis",
			"axis": "spec", "evidence": ["S03"],
			"fields": {
				"Beschreibung": "Renderings should group by axis.",
				"Nutzen": "Readers see standards vs spec findings separately.",
				"Empfehlung": "Add a group-by-axis rendering step."
			}
		}
	]}`
	if err := os.WriteFile(artifactPath, []byte(artifactDoc), 0644); err != nil {
		t.Fatalf("writing findings artifact: %v", err)
	}
	originalArtifactBytes, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("reading artifact back: %v", err)
	}

	// Measured precondition (Risks): the set-difference assertion below is
	// only meaningful if the store starts with no review=-tagged item.
	if pre := filterByWhere(core.All(), []string{"review=" + artifactPath}); len(pre) != 0 {
		t.Fatalf("precondition failed: store already has %d review=%s bean(s)", len(pre), artifactPath)
	}

	// AC-03: promote is invoked as a separate, subsequent act, selecting
	// exactly one of the three finding IDs present in the artifact.
	if err := promoteCmd.RunE(promoteCmd, []string{artifactPath, "B01"}); err != nil {
		t.Fatalf("promote step: %v", err)
	}

	// AC-05: `lif list --where review=<artifact-path>` returns exactly the
	// one promoted item.
	listWhere = []string{"review=" + artifactPath}
	out := captureRunEStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatalf("list step: %v", err)
		}
	})
	var promotedByQuery []*item.Item
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&promotedByQuery); err != nil {
		t.Fatalf("decoding list --where JSON: %v; output = %s", err, out)
	}
	// Errorf, not Fatalf: this is the one substantive assertion (not a
	// setup/IO guard) a downstream mutation must be able to fail in
	// isolation without halting AC-06/AC-07/AC-08's independent checks. A
	// nil `promoted` guards every block below that needs a single
	// unambiguous match to inspect.
	if len(promotedByQuery) != 1 {
		t.Errorf("list --where review=... returned %d beans, want 1", len(promotedByQuery))
	}
	var promoted *item.Item
	if len(promotedByQuery) == 1 {
		promoted = promotedByQuery[0]
	}
	if promoted != nil {
		if promoted.Extra["finding"] != "B01" {
			t.Errorf("promoted bean finding = %v, want %q", promoted.Extra["finding"], "B01")
		}

		// AC-04: the promoted item's body carries the selected record's
		// evidence and source locations unchanged from the artifact.
		if !contains(promoted.Body, "Batch path dereferences without a nil check.") {
			t.Errorf("promoted body missing the record's Beschreibung: %q", promoted.Body)
		}
		if !contains(promoted.Body, "S01") {
			t.Errorf("promoted body missing evidence reference S01: %q", promoted.Body)
		}
	}

	// AC-06: the rejected pair -- the set difference between the artifact's
	// three finding IDs and the promoted set -- has no item, file or record
	// anywhere in the store.
	// Both sides are derived, never literal: allIDs comes from the
	// artifact bytes the review step actually wrote, promotedIDs from the
	// finding= front matter of the items genuinely present in the store --
	// so a defect that promotes the wrong record, or the wrong count of
	// records, changes this computation instead of being invisible to it.
	var artifactDecoded struct {
		Findings []struct {
			ID string `json:"id"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(originalArtifactBytes, &artifactDecoded); err != nil {
		t.Fatalf("decoding artifact to derive its finding IDs: %v", err)
	}
	allIDs := make(map[string]bool, len(artifactDecoded.Findings))
	for _, f := range artifactDecoded.Findings {
		allIDs[f.ID] = true
	}
	promotedIDs := make(map[string]bool, len(promotedByQuery))
	for _, b := range promotedByQuery {
		if id, ok := b.Extra["finding"].(string); ok {
			promotedIDs[id] = true
		}
	}
	var rejected []string
	for id := range allIDs {
		if !promotedIDs[id] {
			rejected = append(rejected, id)
		}
	}
	if len(rejected) != 2 {
		t.Fatalf("rejected set = %v, want 2 members", rejected)
	}
	for _, id := range rejected {
		if matches := filterByWhere(core.All(), []string{"finding=" + id}); len(matches) != 0 {
			t.Errorf("rejected finding %s has a bean in the store: %v", id, matches)
		}
	}
	// SC-02: confirmed by direct inspection of the store directory, not
	// only the --where query -- no .md file in the store carries a
	// `finding:` front matter entry for either rejected finding-id.
	storeEntries, err := os.ReadDir(itemsDir)
	if err != nil {
		t.Fatalf("reading store directory: %v", err)
	}
	for _, entry := range storeEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(itemsDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		for _, id := range rejected {
			if contains(string(raw), "finding: "+id) {
				t.Errorf("%s carries a finding: %s front matter entry for a rejected finding", entry.Name(), id)
			}
		}
	}
	// Total item count is exactly the reviewed item plus the one promoted
	// item -- nothing else was written for the rejected pair.
	if got := countItems(t); got != 2 {
		t.Fatalf("beans in store = %d, want 2 (reviewed + promoted)", got)
	}

	// AC-07: repeating the identical promote call creates nothing new and
	// reports the already-promoted finding as a skipped no-op.
	rerunOut := captureRunEStdout(t, func() {
		if err := promoteCmd.RunE(promoteCmd, []string{artifactPath, "B01"}); err != nil {
			t.Fatalf("repeated promote step: %v", err)
		}
	})
	if got := countItems(t); got != 2 {
		t.Errorf("beans after repeated promote = %d, want 2 (no duplicate)", got)
	}
	if !contains(string(rerunOut), "B01") || !contains(string(rerunOut), "skipped") {
		t.Errorf("repeated promote did not report B01 as a skipped no-op: %q", rerunOut)
	}

	// AC-08: a malformed record (missing the required Beschreibung field) is
	// rejected by record ID and offending field, and nothing is written.
	malformedAttachDir := filepath.Join(itemsDir, itemcore.AttachmentsDir, "beans-malf")
	if err := os.MkdirAll(malformedAttachDir, 0755); err != nil {
		t.Fatalf("creating malformed attachments dir: %v", err)
	}
	malformedArtifactPath := filepath.Join(malformedAttachDir, "findings.json")
	malformedDoc := `{"findings":[
		{
			"id": "B01", "code": "B", "title": "Malformed record",
			"axis": "standards", "evidence": [],
			"blocking": true, "criterion": "acceptance-criteria-violation",
			"fields": {
				"Schwere": "niedrig",
				"Empfehlung": "n/a"
			}
		}
	]}`
	if err := os.WriteFile(malformedArtifactPath, []byte(malformedDoc), 0644); err != nil {
		t.Fatalf("writing malformed artifact: %v", err)
	}
	beforeMalformed := countItems(t)
	rejectErr := promoteCmd.RunE(promoteCmd, []string{malformedArtifactPath, "B01"})
	if rejectErr == nil {
		t.Fatal("expected the malformed record to be rejected, got nil")
	}
	if !contains(rejectErr.Error(), "B01") {
		t.Errorf("rejection %q does not name the malformed record's ID", rejectErr.Error())
	}
	if !contains(rejectErr.Error(), "Beschreibung") {
		t.Errorf("rejection %q does not name the offending field", rejectErr.Error())
	}
	if got := countItems(t); got != beforeMalformed {
		t.Errorf("beans after malformed submission = %d, want %d (nothing written)", got, beforeMalformed)
	}

	// AC-09: the artifact path recorded on the promoted item resolves to the
	// exact file the artifact-writing step produced, byte-for-byte
	// unmodified.
	if promoted != nil && promoted.Extra["review"] != artifactPath {
		t.Errorf("promoted bean review = %v, want %q", promoted.Extra["review"], artifactPath)
	}
	finalArtifactBytes, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("re-reading artifact at recorded path: %v", err)
	}
	if !bytes.Equal(originalArtifactBytes, finalArtifactBytes) {
		t.Errorf("artifact bytes changed since it was written: recorded path %s no longer matches", artifactPath)
	}
}
