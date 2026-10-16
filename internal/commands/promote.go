package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
)

var (
	promoteJSON   bool
	promoteParent string
)

// findingRecord is the subset of a WP1 finding record promote needs. Fields
// is kept as a generic map because its required key set differs per code
// (findings-schema.json's per-code allOf branches) and promote only reads
// the canonical labels it renders (Beschreibung, Nutzen, Empfehlung,
// Schwere), never the object's full shape.
type findingRecord struct {
	ID       string            `json:"id"`
	Code     string            `json:"code"`
	Title    string            `json:"title"`
	Axis     string            `json:"axis"`
	Fields   map[string]string `json:"fields"`
	Evidence []string          `json:"evidence"`
}

// findingsArtifactRaw preserves each record as raw JSON so the schema
// preflight validates exactly what the artifact contains -- decoding into
// findingRecord and re-encoding would silently drop an unknown key that
// additionalProperties:false must catch (AC4/R-19).
type findingsArtifactRaw struct {
	Findings []json.RawMessage `json:"findings"`
}

var promoteCmd = &cobra.Command{
	Use:   "promote <artifact> <finding-id> [<finding-id>...]",
	Short: "Promote selected findings from a review artifact into items",
	Long: `Validates the named finding records from a review artifact against the
findings schema and, once every selected record passes, creates one item per
record by a fixed code-to-type mapping (B becomes a bug, I becomes a task;
D, T and Q have no mapping and are rejected). New items get the configured
default status. With --parent every new item is created under that item.

The whole call is checked before the first write: if any selected record,
field or the parent is invalid, nothing is written. A finding already
promoted by an earlier call is skipped rather than duplicated. If a write
still fails, the error names the items already written.`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		artifactPath := args[0]
		findingIDs := args[1:]

		if err := rejectDuplicateFindingIDs(findingIDs); err != nil {
			return cmdError(promoteJSON, output.ErrValidation, "%s", err)
		}

		artifactBytes, err := os.ReadFile(artifactPath)
		if err != nil {
			return cmdError(promoteJSON, output.ErrFileError, "failed to read findings artifact: %v", err)
		}

		var artifact findingsArtifactRaw
		if err := json.Unmarshal(artifactBytes, &artifact); err != nil {
			return cmdError(promoteJSON, output.ErrFileError, "failed to parse findings artifact: %v", err)
		}

		rawByID := make(map[string]json.RawMessage, len(artifact.Findings))
		for _, raw := range artifact.Findings {
			var idOnly struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &idOnly); err != nil || idOnly.ID == "" {
				continue
			}
			rawByID[idOnly.ID] = raw
		}

		selectedRaw := make([]json.RawMessage, 0, len(findingIDs))
		var missing []string
		for _, id := range findingIDs {
			raw, ok := rawByID[id]
			if !ok {
				missing = append(missing, id)
				continue
			}
			selectedRaw = append(selectedRaw, raw)
		}
		if len(missing) > 0 {
			return cmdError(promoteJSON, output.ErrNotFound, "finding id(s) not found in artifact: %s", strings.Join(missing, ", "))
		}

		if err := validateSelectedRecords(findingIDs, selectedRaw); err != nil {
			return cmdError(promoteJSON, output.ErrValidation, "%s", err)
		}

		records := make([]findingRecord, len(selectedRaw))
		for i, raw := range selectedRaw {
			var rec findingRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return cmdError(promoteJSON, output.ErrValidation, "failed to decode validated record %s: %v", findingIDs[i], err)
			}
			records[i] = rec
		}

		resolver := newResolver()
		ctx := context.Background()

		parentID := ""
		if promoteParent != "" {
			id, ok := core.NormalizeID(promoteParent)
			if !ok {
				return cmdError(promoteJSON, output.ErrNotFound, "parent item not found: %s", promoteParent)
			}
			parentID = id
		}

		// Preflight: prepare and validate every candidate before the first
		// write, so a bad record, parent, field or required-fields policy
		// leaves the store untouched (beans-z69m). Skipped lines are held back
		// so a failed preflight prints none.
		type candidate struct {
			findingID string
			input     model.CreateItemInput
			setMap    map[string]any
		}
		var queue []candidate
		var skipped []string
		for _, rec := range records {
			if alreadyPromoted(artifactPath, rec.ID) {
				skipped = append(skipped, fmt.Sprintf("skipped %s: already promoted from %s", rec.ID, artifactPath))
				continue
			}
			input, setMap, err := preparePromotedItem(artifactPath, parentID, rec)
			if err != nil {
				return cmdError(promoteJSON, output.ErrValidation, "record %s: %s", rec.ID, err)
			}
			if err := resolver.ValidateCreateInput(input); err != nil {
				return cmdError(promoteJSON, output.ErrValidation, "record %s: %s", rec.ID, err)
			}
			if err := core.ValidateParent(&item.Item{Type: *input.Type}, parentID); err != nil {
				return cmdError(promoteJSON, output.ErrValidation, "record %s: %s", rec.ID, err)
			}
			// require_fields_on for the status the item is created in: the
			// front matter promote sets must satisfy it, or the write refuses.
			if err := preflightStatusPolicy([]*item.Item{{ID: rec.ID}}, *input.Status, setMap); err != nil {
				return cmdError(promoteJSON, mutationErrorCode(err), "record %s: %s", rec.ID, err)
			}
			queue = append(queue, candidate{rec.ID, input, setMap})
		}
		if !promoteJSON {
			for _, line := range skipped {
				fmt.Println(line)
			}
		}

		done := make([]*item.Item, 0, len(queue))
		for _, c := range queue {
			b, err := resolver.CreateItem(ctx, c.input, itemcore.WithExtraOps(c.setMap, nil))
			if err != nil {
				return emitBatchFailure(promoteJSON, done, fmt.Errorf("failed to create item for %s: %v", c.findingID, err))
			}
			done = append(done, b)
		}

		return emitBatchSuccess(promoteJSON, done,
			func(b *item.Item) error { return output.SuccessSingle(b) },
			func(b *item.Item) string { return fmt.Sprintf("Created %s %s", b.ID, b.Title) },
		)
	},
}

// rejectDuplicateFindingIDs rejects a finding-id named more than once in the
// same call (AC15/SC-05), before the artifact is even read.
func rejectDuplicateFindingIDs(ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return fmt.Errorf("finding id %s named more than once in the same call", id)
		}
		seen[id] = true
	}
	return nil
}

// compilePromoteSchema compiles the embedded FindingsSchema document.
func compilePromoteSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(FindingsSchema))
	if err != nil {
		return nil, fmt.Errorf("unmarshal embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("findings-schema.json", doc); err != nil {
		return nil, fmt.Errorf("add embedded schema as resource: %w", err)
	}
	return c.Compile("findings-schema.json")
}

// validateSelectedRecords validates exactly the selected records against
// WP1's schema (R-19/AC4/AC5): it builds a filtered artifact document
// containing only the records named by ids, in the same order, so an
// unselected record is never evaluated and a rejection can be mapped back
// to the finding-id that produced it by array index.
func validateSelectedRecords(ids []string, selectedRaw []json.RawMessage) error {
	sch, err := compilePromoteSchema()
	if err != nil {
		return fmt.Errorf("compiling findings schema: %w", err)
	}

	doc := struct {
		Findings []json.RawMessage `json:"findings"`
	}{Findings: selectedRaw}
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("building preflight document: %w", err)
	}

	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(docBytes)))
	if err != nil {
		return fmt.Errorf("decoding preflight document: %w", err)
	}

	err = sch.Validate(inst)
	if err == nil {
		return nil
	}

	verr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return err
	}

	// BasicOutput's flatten collapses an if/then branch into its parent
	// allOf failure without surfacing the "then" schema's own required-
	// property error as a separate flat entry, so the offending field is
	// lost. DetailedOutput preserves the full tree instead; walking it to
	// its true leaves (no further Errors children) is what recovers the
	// specific keyword failure (e.g. "missing property 'Beschreibung'" at
	// ".../fields") rather than the generic "validation failed" at the
	// record's allOf node.
	var leaves []*jsonschema.OutputUnit
	collectValidationLeaves(verr.DetailedOutput(), &leaves)

	// Per malformed record, report only its deepest (most specific) leaf --
	// several leaves can share the same record when one failure cascades
	// (e.g. a missing key inside an if/then branch also fails the
	// surrounding allOf), and the deepest one names the actual field.
	deepestPerRecord := make(map[string]*jsonschema.OutputUnit)
	order := make([]string, 0, len(ids))
	for _, leaf := range leaves {
		recordID, _ := describeInstanceLocation(ids, leaf.InstanceLocation)
		existing, seen := deepestPerRecord[recordID]
		if !seen {
			order = append(order, recordID)
		}
		if !seen || len(leaf.InstanceLocation) > len(existing.InstanceLocation) {
			deepestPerRecord[recordID] = leaf
		}
	}
	if len(order) == 0 {
		return fmt.Errorf("selected record(s) failed schema validation: %v", err)
	}

	messages := make([]string, 0, len(order))
	for _, recordID := range order {
		leaf := deepestPerRecord[recordID]
		_, field := describeInstanceLocation(ids, leaf.InstanceLocation)
		messages = append(messages, fmt.Sprintf("record %s: field %s: %s", recordID, field, leaf.Error.String()))
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

// collectValidationLeaves appends every true leaf (a node with no further
// Errors children) reachable from unit into leaves.
func collectValidationLeaves(unit *jsonschema.OutputUnit, leaves *[]*jsonschema.OutputUnit) {
	if len(unit.Errors) == 0 {
		*leaves = append(*leaves, unit)
		return
	}
	for i := range unit.Errors {
		collectValidationLeaves(&unit.Errors[i], leaves)
	}
}

// describeInstanceLocation maps a JSON-Schema instance location such as
// "/findings/1/fields/Schwere" back to the finding-id at that position in
// ids (the preflight document's records are in the same order as ids) and
// the remaining path as a dotted field reference.
func describeInstanceLocation(ids []string, instanceLocation string) (recordID, field string) {
	segments := strings.Split(strings.TrimPrefix(instanceLocation, "/"), "/")
	if len(segments) < 2 || segments[0] != "findings" {
		return "unknown", instanceLocation
	}
	var idx int
	if _, err := fmt.Sscanf(segments[1], "%d", &idx); err != nil || idx < 0 || idx >= len(ids) {
		return "unknown", instanceLocation
	}
	recordID = ids[idx]
	if len(segments) > 2 {
		field = strings.Join(segments[2:], ".")
	} else {
		field = "(record)"
	}
	return recordID, field
}

// alreadyPromoted reports whether artifactPath+findingID was already
// promoted by an earlier call (R-27/AC11/SC-04), keyed on the review+finding
// pair together -- never on finding alone, since finding-ids restart at 01
// per artifact and can recur across unrelated reviews.
func alreadyPromoted(artifactPath, findingID string) bool {
	matches := filterByWhere(core.All(), []string{"review=" + artifactPath, "finding=" + findingID})
	return len(matches) > 0
}

// preparePromotedItem renders one validated finding record into the create
// input and extra front matter of its item, applying the fixed code-to-type
// mapping (R-16/AC9) and the extra front matter every promoted item carries
// (R-27/R-28/R-30/AC10/AC12/AC16). It writes nothing.
func preparePromotedItem(artifactPath, parentID string, rec findingRecord) (model.CreateItemInput, map[string]any, error) {
	var itemType string
	var tags []string
	switch rec.Code {
	case "B":
		itemType = "bug"
	case "I":
		itemType = "task"
		tags = []string{"improvement"}
	default:
		return model.CreateItemInput{}, nil, fmt.Errorf("code %s has no promotion mapping", rec.Code)
	}

	setMap := map[string]any{
		"review":  artifactPath,
		"finding": rec.ID,
		"axis":    rec.Axis,
	}
	if rec.Code == "B" {
		if severity, ok := rec.Fields["Schwere"]; ok && severity != "" {
			setMap["severity"] = severity
		}
	}

	body := renderPromotedBody(rec, artifactPath)
	// The default status is passed explicitly, as `lif create` does, so
	// ValidateCreateInput checks it against the status table.
	status := cfg.GetDefaultStatus()
	input := model.CreateItemInput{
		Title:  rec.Title,
		Type:   &itemType,
		Status: &status,
		Tags:   tags,
		Body:   &body,
	}
	if parentID != "" {
		input.Parent = &parentID
	}
	return input, setMap, nil
}

// renderPromotedBody renders a promoted item's body by the fixed mapping
// (R-18/AC7/AC8): the record's canonical fields (excluding Schwere, which
// promote diverts to the severity front matter key instead of duplicating
// it in the body) become English-headed sections, evidence/source
// locations (S## references) transfer unchanged, no Acceptance section is
// ever produced, and Provenance is synthesized here rather than supplied by
// the reviewer.
func renderPromotedBody(rec findingRecord, artifactPath string) string {
	var sb strings.Builder

	if desc, ok := rec.Fields["Beschreibung"]; ok && desc != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}
	if benefit, ok := rec.Fields["Nutzen"]; ok && benefit != "" {
		sb.WriteString("## Benefit\n\n")
		sb.WriteString(benefit)
		sb.WriteString("\n\n")
	}
	if rec_, ok := rec.Fields["Empfehlung"]; ok && rec_ != "" {
		sb.WriteString("## Recommendation\n\n")
		sb.WriteString(rec_)
		sb.WriteString("\n\n")
	}
	if len(rec.Evidence) > 0 {
		sb.WriteString("## Evidence\n\n")
		for _, e := range rec.Evidence {
			sb.WriteString("- " + e + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Provenance\n\n")
	sb.WriteString(fmt.Sprintf("Promoted from `%s`, finding `%s`.\n", artifactPath, rec.ID))

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// RegisterPromoteCmd adds the promote command to root. Flag registration is
// idempotent (guarded by a Lookup check), the same pattern RegisterListCmd
// and RegisterOrderCmd use, so a test registering promoteCmd into a
// throwaway root does not panic on a second flag definition.
func RegisterPromoteCmd(root *cobra.Command) {
	if promoteCmd.Flags().Lookup("json") == nil {
		promoteCmd.Flags().BoolVar(&promoteJSON, "json", false, "Output as JSON")
		promoteCmd.Flags().StringVar(&promoteParent, "parent", "", "Parent item ID for every promoted item")
	}
	root.AddCommand(promoteCmd)
}
