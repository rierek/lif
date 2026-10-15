package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rierek/lif/internal/agent"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// recvWithin returns the next value from ch, or fails the test.
func recvWithin[T any](t *testing.T, ch <-chan T, d time.Duration, what string) (T, bool) {
	t.Helper()
	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(d):
		var zero T
		t.Fatalf("timed out waiting for %s", what)
		return zero, false
	}
}

// TestSubscriptionsWithoutManagers pins the behaviour of a server started
// without the optional managers — the CLI's GraphQL surface runs that way.
// Each subscription has to hand back a channel that is simply closed, not a nil
// channel (blocks forever) and not a panic.
func TestSubscriptionsWithoutManagers(t *testing.T) {
	resolver, _ := setupTestResolver(t)
	sr := resolver.Subscription()
	ctx := context.Background()

	t.Run("agentSessionChanged", func(t *testing.T) {
		ch, err := sr.AgentSessionChanged(ctx, "bean-1")
		if err != nil {
			t.Fatalf("AgentSessionChanged() error = %v", err)
		}
		if _, ok := recvWithin(t, ch, time.Second, "the closed channel"); ok {
			t.Error("expected a closed channel, got a value")
		}
	})

	t.Run("activeAgentStatuses", func(t *testing.T) {
		ch, err := sr.ActiveAgentStatuses(ctx)
		if err != nil {
			t.Fatalf("ActiveAgentStatuses() error = %v", err)
		}
		if _, ok := recvWithin(t, ch, time.Second, "the closed channel"); ok {
			t.Error("expected a closed channel, got a value")
		}
	})

	t.Run("worktreesChanged", func(t *testing.T) {
		ch, err := sr.WorktreesChanged(ctx)
		if err != nil {
			t.Fatalf("WorktreesChanged() error = %v", err)
		}
		if _, ok := recvWithin(t, ch, time.Second, "the closed channel"); ok {
			t.Error("expected a closed channel, got a value")
		}
	})
}

// TestAgentSessionChangedResetsAfterClear covers the explicit empty payload:
// once a session is cleared the subscriber must be told, with an idle session
// carrying no messages, so the UI resets instead of showing the old transcript.
func TestAgentSessionChangedResetsAfterClear(t *testing.T) {
	mgr := agent.NewManager("", nil)
	defer mgr.Shutdown()
	resolver := &Resolver{AgentMgr: mgr}
	sr := resolver.Subscription()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const itemID = "bean-cleared"
	mgr.AddInfoMessage(itemID, "something happened")

	ch, err := sr.AgentSessionChanged(ctx, itemID)
	if err != nil {
		t.Fatalf("AgentSessionChanged() error = %v", err)
	}

	first, _ := recvWithin(t, ch, 2*time.Second, "the initial session state")
	if len(first.Messages) != 1 {
		t.Fatalf("initial state has %d messages, want 1", len(first.Messages))
	}

	if err := mgr.ClearSession(itemID); err != nil {
		t.Fatalf("ClearSession() error = %v", err)
	}

	after, _ := recvWithin(t, ch, 2*time.Second, "the reset session state")
	if after == nil {
		t.Fatal("received a nil session after the clear")
	}
	if after.ItemID != itemID {
		t.Errorf("ItemID = %q, want %q", after.ItemID, itemID)
	}
	if after.Status != model.AgentSessionStatusIdle {
		t.Errorf("Status = %v, want %v", after.Status, model.AgentSessionStatusIdle)
	}
	if len(after.Messages) != 0 {
		t.Errorf("reset state has %d messages, want 0", len(after.Messages))
	}
}

// TestSubscriptionsStopOnContextCancel makes sure a disconnecting client takes
// the forwarding goroutine and its upstream subscription with it.
func TestSubscriptionsStopOnContextCancel(t *testing.T) {
	t.Run("beanChanged", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		sr := resolver.Subscription()
		ctx, cancel := context.WithCancel(context.Background())

		ch, err := sr.ItemChanged(ctx, nil)
		if err != nil {
			t.Fatalf("ItemChanged() error = %v", err)
		}

		cancel()
		for {
			v, ok := recvWithin(t, ch, 2*time.Second, "the channel to close")
			if !ok {
				return
			}
			_ = v
		}
	})

	t.Run("agentSessionChanged", func(t *testing.T) {
		mgr := agent.NewManager("", nil)
		defer mgr.Shutdown()
		resolver := &Resolver{AgentMgr: mgr}
		sr := resolver.Subscription()
		ctx, cancel := context.WithCancel(context.Background())

		const itemID = "bean-cancel"
		ch, err := sr.AgentSessionChanged(ctx, itemID)
		if err != nil {
			t.Fatalf("AgentSessionChanged() error = %v", err)
		}

		cancel()
		// Nudge the goroutine so it observes the cancelled context.
		mgr.AddInfoMessage(itemID, "after cancel")

		for {
			v, ok := recvWithin(t, ch, 2*time.Second, "the channel to close")
			if !ok {
				return
			}
			_ = v
		}
	})
}

// TestItemChangedResyncsAfterOverflow covers the recovery path for a client
// that fell behind: itemcore drops its events, marks it for resync, and the
// resolver answers with a fresh full snapshot instead of leaving the client on
// an incomplete history.
func TestItemChangedResyncsAfterOverflow(t *testing.T) {
	resolver, core := setupTestResolver(t)
	sr := resolver.Subscription()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Enough items that archiving them all outruns both buffers (16 in
	// itemcore, 64 in the resolver).
	const itemCount = 200
	for i := range itemCount {
		createTestItem(t, core, fmt.Sprintf("overflow-%03d", i), fmt.Sprintf("Bean %d", i), "todo")
	}

	ch, err := sr.ItemChanged(ctx, nil)
	if err != nil {
		t.Fatalf("ItemChanged() error = %v", err)
	}

	// Produce events without reading any of them.
	for i := range itemCount {
		if err := core.Archive(fmt.Sprintf("overflow-%03d", i)); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
	}

	sawSnapshot := false
	for !sawSnapshot {
		event, _ := recvWithin(t, ch, 5*time.Second, "a resync snapshot")
		if event.Type == model.ChangeTypeInitialSnapshot {
			sawSnapshot = true
			if len(event.Items) == 0 {
				t.Error("resync snapshot carried no beans")
			}
		}
	}
}
