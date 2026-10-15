package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileFindingsSchema compiles the embedded FindingsSchema document. Every
// test in this file exercises it, so a broken or empty embed (AC1) turns
// every one of them red, not just a single dedicated case.
func compileFindingsSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	if strings.TrimSpace(FindingsSchema) == "" {
		t.Fatal("FindingsSchema is empty")
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(FindingsSchema))
	if err != nil {
		t.Fatalf("unmarshal embedded schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("findings-schema.json", doc); err != nil {
		t.Fatalf("add embedded schema as resource: %v", err)
	}
	sch, err := c.Compile("findings-schema.json")
	if err != nil {
		t.Fatalf("compile embedded schema: %v", err)
	}
	return sch
}

// validate decodes doc as a JSON instance and runs it through sch.
func validate(t *testing.T, sch *jsonschema.Schema, doc string) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("unmarshal instance: %v", err)
	}
	return sch.Validate(inst)
}

// SC-01: one valid record per code (D, T, B, I, Q) validates with zero
// rejections. Also grounds AC1 (the document publishes both shapes) and the
// per-code exact-fields-key rule (AC2) on the accepting side.
func TestFindingsSchema_ValidArtifactWithAllCodes(t *testing.T) {
	sch := compileFindingsSchema(t)
	doc := `{
		"findings": [
			{
				"id": "D01", "code": "D", "title": "Use file-adjacent embeds",
				"axis": "standards", "evidence": ["S01"],
				"fields": {
					"Hintergrund": "go:embed can only read its own package directory.",
					"Entscheidung": "Ship the schema beside prime.go.",
					"Empfehlung": "Keep the pattern for future embeds."
				}
			},
			{
				"id": "T01", "code": "T", "title": "Wire WP2 to the schema",
				"axis": "spec", "evidence": [],
				"fields": { "Prio": "P1", "Aufgabe": "Load FindingsSchema in promote.go." }
			},
			{
				"id": "B01", "code": "B", "title": "Missing axis on legacy record",
				"axis": "standards", "evidence": ["S02", "S03"],
				"blocking": true, "criterion": "acceptance-criteria-violation",
				"fields": {
					"Schwere": "hoch",
					"Beschreibung": "Record has no axis field.",
					"Empfehlung": "Reject at validation time."
				}
			},
			{
				"id": "I01", "code": "I", "title": "Group renderings by axis",
				"axis": "spec", "evidence": ["S04"],
				"fields": {
					"Beschreibung": "Renderings should group by axis.",
					"Nutzen": "Readers see standards vs spec findings separately.",
					"Empfehlung": "Add a group-by-axis rendering step."
				}
			},
			{
				"id": "Q01", "code": "Q", "title": "Should Status ever return?",
				"axis": "standards", "evidence": [],
				"fields": {
					"Hintergrund": "R-17 already drops Status for B/I.",
					"Relevanz": "Confirms the schema drops it everywhere.",
					"Frage": "Is there any lifecycle left to track post-write?"
				}
			}
		]
	}`
	if err := validate(t, sch, doc); err != nil {
		t.Fatalf("expected zero rejections, got: %v", err)
	}
}

// SC-02 / AC3: blocking is permitted only on a B record; blocking: true on
// any other code is a violation.
func TestFindingsSchema_RejectsBlockingOnNonBCode(t *testing.T) {
	sch := compileFindingsSchema(t)
	doc := `{
		"findings": [{
			"id": "I01", "code": "I", "title": "x", "axis": "spec",
			"evidence": [], "blocking": true,
			"fields": { "Beschreibung": "x", "Nutzen": "x", "Empfehlung": "x" }
		}]
	}`
	if err := validate(t, sch, doc); err == nil {
		t.Fatal("expected rejection of blocking: true on a non-B record, got none")
	}
}

// SC-03 / AC4: blocking: true requires criterion, restricted to the four
// enumerated values.
func TestFindingsSchema_BlockingTrueRequiresValidCriterion(t *testing.T) {
	sch := compileFindingsSchema(t)

	t.Run("missing criterion", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "B01", "code": "B", "title": "x", "axis": "standards",
				"evidence": ["S01"], "blocking": true,
				"fields": { "Schwere": "x", "Beschreibung": "x", "Empfehlung": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of blocking: true without criterion, got none")
		}
	})

	t.Run("criterion outside the enum", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "B01", "code": "B", "title": "x", "axis": "standards",
				"evidence": ["S01"], "blocking": true, "criterion": "not-a-real-value",
				"fields": { "Schwere": "x", "Beschreibung": "x", "Empfehlung": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of an out-of-enum criterion, got none")
		}
	})
}

// AC5: criterion is forbidden whenever blocking is not true, whether
// blocking is explicitly false or simply absent.
func TestFindingsSchema_RejectsCriterionWithoutBlockingTrue(t *testing.T) {
	sch := compileFindingsSchema(t)

	t.Run("blocking absent", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "B01", "code": "B", "title": "x", "axis": "standards",
				"evidence": ["S01"], "criterion": "regression",
				"fields": { "Schwere": "x", "Beschreibung": "x", "Empfehlung": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of criterion without blocking: true, got none")
		}
	})

	t.Run("blocking false", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "B01", "code": "B", "title": "x", "axis": "standards",
				"evidence": ["S01"], "blocking": false, "criterion": "regression",
				"fields": { "Schwere": "x", "Beschreibung": "x", "Empfehlung": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of criterion with blocking: false, got none")
		}
	})
}

// SC-04 / AC6: axis is required on every record, exactly "standards" or
// "spec", no default.
func TestFindingsSchema_RejectsMissingOrInvalidAxis(t *testing.T) {
	sch := compileFindingsSchema(t)

	t.Run("axis absent", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "T01", "code": "T", "title": "x", "evidence": [],
				"fields": { "Prio": "x", "Aufgabe": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of a record with no axis, got none")
		}
	})

	t.Run("axis outside the enum", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "T01", "code": "T", "title": "x", "axis": "other", "evidence": [],
				"fields": { "Prio": "x", "Aufgabe": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of axis: other, got none")
		}
	})
}

// AC7: evidence entries must match the S## reference pattern; an empty
// evidence array remains valid (covered by the T01 record in SC-01).
func TestFindingsSchema_RejectsEvidenceEntryOutsidePattern(t *testing.T) {
	sch := compileFindingsSchema(t)
	doc := `{
		"findings": [{
			"id": "T01", "code": "T", "title": "x", "axis": "spec",
			"evidence": ["not-an-s-ref"],
			"fields": { "Prio": "x", "Aufgabe": "x" }
		}]
	}`
	if err := validate(t, sch, doc); err == nil {
		t.Fatal("expected rejection of an evidence entry not matching ^S[0-9]{2}$, got none")
	}
}

// AC2: fields must carry exactly the canonical keys for its code — missing
// one, or carrying an extra one, is a violation.
func TestFindingsSchema_RejectsFieldsMissingOrExtraKey(t *testing.T) {
	sch := compileFindingsSchema(t)

	t.Run("missing a canonical key", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "B01", "code": "B", "title": "x", "axis": "standards",
				"evidence": [],
				"fields": { "Schwere": "x", "Empfehlung": "x" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of a B record missing Beschreibung, got none")
		}
	})

	t.Run("extra key beyond the canonical set", func(t *testing.T) {
		doc := `{
			"findings": [{
				"id": "T01", "code": "T", "title": "x", "axis": "spec",
				"evidence": [],
				"fields": { "Prio": "x", "Aufgabe": "x", "Bonus": "unexpected" }
			}]
		}`
		if err := validate(t, sch, doc); err == nil {
			t.Fatal("expected rejection of a T record with an extra fields key, got none")
		}
	})
}

// AC8: no code's fields object may carry a Status key.
func TestFindingsSchema_RejectsStatusFieldOnAnyCode(t *testing.T) {
	sch := compileFindingsSchema(t)
	doc := `{
		"findings": [{
			"id": "Q01", "code": "Q", "title": "x", "axis": "standards",
			"evidence": [],
			"fields": {
				"Hintergrund": "x", "Relevanz": "x", "Frage": "x", "Status": "open"
			}
		}]
	}`
	if err := validate(t, sch, doc); err == nil {
		t.Fatal("expected rejection of a Status key in fields, got none")
	}
}

// Integration-points table: id's first character must equal code, not just
// be one of D/T/B/I/Q in isolation.
func TestFindingsSchema_RejectsIDCodeMismatch(t *testing.T) {
	sch := compileFindingsSchema(t)
	doc := `{
		"findings": [{
			"id": "D01", "code": "T", "title": "x", "axis": "spec",
			"evidence": [],
			"fields": { "Prio": "x", "Aufgabe": "x" }
		}]
	}`
	if err := validate(t, sch, doc); err == nil {
		t.Fatal("expected rejection of an id whose first character does not match code, got none")
	}
}

// AC9: every field name, criterion value, and axis value this package
// introduces is English. Pinned against the schema's own decoded structure
// (not retyped literals) so a rename or a re-introduced German token turns
// this red.
func TestFindingsSchema_IntroducedVocabularyIsEnglish(t *testing.T) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(FindingsSchema))
	if err != nil {
		t.Fatalf("unmarshal embedded schema: %v", err)
	}
	root, ok := doc.(map[string]any)
	if !ok {
		t.Fatal("schema root is not a JSON object")
	}
	defs, ok := root["$defs"].(map[string]any)
	if !ok {
		t.Fatal("schema has no $defs object")
	}
	record, ok := defs["record"].(map[string]any)
	if !ok {
		t.Fatal("schema has no $defs.record object")
	}
	properties, ok := record["properties"].(map[string]any)
	if !ok {
		t.Fatal("record has no properties object")
	}

	wantFieldNames := []string{"id", "code", "title", "axis", "fields", "evidence", "blocking", "criterion"}
	for _, name := range wantFieldNames {
		if _, ok := properties[name]; !ok {
			t.Errorf("record.properties is missing introduced English field name %q", name)
		}
	}
	for name := range properties {
		if !isASCIILower(name) {
			t.Errorf("record.properties has non-English-looking field name %q", name)
		}
	}

	axis, ok := properties["axis"].(map[string]any)
	if !ok {
		t.Fatal("record.properties.axis is not an object")
	}
	wantEnum(t, axis["enum"], []string{"standards", "spec"})

	criterion, ok := properties["criterion"].(map[string]any)
	if !ok {
		t.Fatal("record.properties.criterion is not an object")
	}
	wantEnum(t, criterion["enum"], []string{
		"acceptance-criteria-violation",
		"regression",
		"security-or-data-integrity-risk",
		"demonstrated-stability-risk",
	})
}

func isASCIILower(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return len(s) > 0
}

func wantEnum(t *testing.T, got any, want []string) {
	t.Helper()
	list, ok := got.([]any)
	if !ok {
		t.Fatalf("enum is not a list: %v", got)
	}
	if len(list) != len(want) {
		t.Fatalf("enum = %v, want exactly %v", list, want)
	}
	for i, w := range want {
		if list[i] != w {
			t.Errorf("enum[%d] = %v, want %q", i, list[i], w)
		}
	}
}

// SC-04 / AC6: the embedded schema names the product as lif — in its $id and
// in the promote command its description points to — and still compiles and
// validates a record under that $id.
func TestFindingsSchema_NamesLif(t *testing.T) {
	var head struct {
		ID          string `json:"$id"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(FindingsSchema), &head); err != nil {
		t.Fatalf("decoding the embedded schema: %v", err)
	}
	if want := "https://github.com/rierek/lif/schemas/findings-artifact.schema.json"; head.ID != want {
		t.Errorf("$id = %q, want %q", head.ID, want)
	}
	if !strings.Contains(head.Description, "`lif promote`") {
		t.Errorf("description does not name `lif promote`: %q", head.Description)
	}
	if err := validate(t, compileFindingsSchema(t), `{"findings": []}`); err != nil {
		t.Errorf("empty findings array no longer validates: %v", err)
	}
}
