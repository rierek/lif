package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/pkg/itemcore"
)

func TestPresentGraphQLErrorCarriesTheCode(t *testing.T) {
	ctx := context.Background()

	blocked := &hooks.BlockedError{Hook: "release-gate", ItemID: "t-1", Message: "missing", Output: "grep: x"}
	got := presentGraphQLError(ctx, fmt.Errorf("wrapped: %w", blocked))
	if got.Extensions["code"] != "POLICY_VIOLATION" || got.Extensions["hook"] != "release-gate" ||
		got.Extensions["itemId"] != "t-1" || got.Extensions["output"] != "grep: x" {
		t.Errorf("gate block: extensions = %v", got.Extensions)
	}

	got = presentGraphQLError(ctx, &itemcore.ETagMismatchError{Provided: "a", Current: "b"})
	if got.Extensions["code"] != "CONFLICT" {
		t.Errorf("etag mismatch: extensions = %v", got.Extensions)
	}

	got = presentGraphQLError(ctx, itemcore.ErrNotFound)
	if got.Extensions != nil {
		t.Errorf("not found: extensions = %v, want none", got.Extensions)
	}
}
