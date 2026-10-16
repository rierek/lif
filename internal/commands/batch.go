package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
)

// resolveBatchTargets resolves every ID a batch verb was given before any of
// them is written, following the delete command's preflight: an unresolvable
// ID anywhere in the call means nothing is mutated at all.
//
// IDs are normalized first, so that "abc" and "beans-abc" are recognized as
// the same item. Naming one item twice is rejected rather than deduplicated:
// the verbs append body sections, and silently collapsing the repeat would
// hide a typo that changed which items the caller thought they were touching.
//
// The returned code is the output.Err* constant the caller passes to cmdError.
func resolveBatchTargets(ctx context.Context, resolver *itemgraph.CoreResolver, ids []string) ([]*item.Item, string, error) {
	targets := make([]*item.Item, 0, len(ids))
	seen := make(map[string]string, len(ids))

	for _, id := range ids {
		fullID, _ := core.NormalizeID(id)
		if first, dup := seen[fullID]; dup {
			return nil, output.ErrValidation, fmt.Errorf("item %s named more than once (as %q and %q)", fullID, first, id)
		}
		seen[fullID] = id

		b, err := resolver.Item(ctx, id)
		if err != nil {
			return nil, output.ErrNotFound, fmt.Errorf("failed to find item: %v", err)
		}
		if b == nil {
			return nil, output.ErrNotFound, fmt.Errorf("item not found: %s", id)
		}
		targets = append(targets, b)
	}

	return targets, "", nil
}

// preflightStatusPolicy answers, without writing, the question the write path
// asks in itemcore: would moving these items into newStatus leave a required
// front matter field unset? It mirrors two conditions of that check — the
// policy only applies on a real status change, and the fields contributed by
// --commit/--set count towards satisfying it.
func preflightStatusPolicy(targets []*item.Item, newStatus string, setMap map[string]any) error {
	fields := cfg.RequiredFieldsFor(newStatus)
	if len(fields) == 0 {
		return nil
	}

	for _, b := range targets {
		if b.Status == newStatus {
			continue
		}
		probe := *b
		probe.Extra = make(map[string]any, len(b.Extra)+len(setMap))
		for k, v := range b.Extra {
			probe.Extra[k] = v
		}
		for k, v := range setMap {
			probe.Extra[k] = v
		}
		if missing := itemcore.MissingRequiredFields(&probe, fields); len(missing) > 0 {
			return &itemcore.PolicyViolationError{ItemID: b.ID, Status: newStatus, Missing: missing}
		}
	}
	return nil
}

// emitBatchSuccess renders the result of a batch mutation. With a single ID
// the caller's established single-item shape is kept, so an existing
// invocation sees byte-identical output; several IDs give one bare array.
func emitBatchSuccess(jsonMode bool, done []*item.Item, single func(*item.Item) error, line func(*item.Item) string) error {
	if jsonMode {
		if len(done) == 1 {
			return single(done[0])
		}
		return output.SuccessMultiple(done)
	}
	for _, b := range done {
		fmt.Println(line(b))
	}
	return nil
}

// emitBatchFailure reports a failure that struck partway through the write
// loop. The preflight rules out unresolvable IDs and policy violations, so
// what remains here is an I/O error, a gate that blocked a later target
// (hooks), or, once a hook ran, an etag conflict on a later target: the hook's
// task may have written that item, and the resolver then expects the etag
// that it read. By then some items are already written. Without hooks no etag
// conflict can occur: batch verbs deliberately set no IfMatch (an etag
// belongs to one item, so `update --if-match` takes one ID; a batch has no
// per-target etag to pass), and last write wins. Under require_if_match,
// itemcore rejects the batch at the first target, before any write. Unlike
// delete, which discards the written list on its way out, the items that made
// it through are named, because an agent's next move depends on knowing which
// half of the batch happened.
func emitBatchFailure(jsonMode bool, done []*item.Item, err error) error {
	if jsonMode {
		// A bare array cannot carry the failure, so the error case keeps the
		// envelope — one document, with the items already written inside it.
		// PartialFailure returns that document's *emittedError marker (like
		// output.Error), so it is returned as-is: discarding it here and
		// returning a plain error instead would leave reportExecutionError
		// free to print a second, human-readable copy of the failure the
		// document above already carries.
		return output.PartialFailure(done, mutationErrorCode(err), err)
	}

	if len(done) > 0 {
		ids := make([]string, len(done))
		for i, b := range done {
			ids[i] = b.ID
		}
		// %w, not %v: reportExecutionError and mutationErrorCode classify the
		// error by its type. With %v a gate's block would print as a usage
		// error, with the pointer to the manual.
		return fmt.Errorf("%w (already written: %s)", err, strings.Join(ids, ", "))
	}
	return err
}
