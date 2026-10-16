package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

// TestIsUserFacingDefaultsClosed pins the direction IsUserFacing itself
// takes if the wiring invariant below were ever violated. In the running
// tree an unmarked command cannot occur: RegisterCoreCommands's final pass
// explicitly marks every not-yet-plumbing command user-facing, so
// "unmarked" is not a live state, not a silent fallthrough. This test
// exercises IsUserFacing directly on a bare command that never went
// through that wiring, and requires it to read as plumbing rather than
// defaulting open.
func TestIsUserFacingDefaultsClosed(t *testing.T) {
	bare := &cobra.Command{Use: "unclassified"}
	if IsUserFacing(bare) {
		t.Error("a command with no audience annotation reported user-facing, want plumbing-by-default")
	}
}

// TestCobraBuiltinsArePresentAndPlumbing guards against the marker becoming
// vacuous. TestAudienceClassifiesThePlumbingSet only classifies whatever is
// already in root.Commands(): if the InitDefaultHelpCmd/
// InitDefaultCompletionCmd calls in RegisterCoreCommands were ever deleted,
// "completion" and "help" would simply be absent from the tree, and that
// test would stay green over a smaller set. This test finds them by name
// and requires both that they exist and that they are plumbing.
func TestCobraBuiltinsArePresentAndPlumbing(t *testing.T) {
	root := sharedTestRoot(t)
	for _, name := range []string{"completion", "help"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("%q is missing from the command tree: %v", name, err)
		}
		if got := cmd.Annotations[audienceAnnotationKey]; got != audiencePlumbing {
			t.Errorf("%q carries audience marker %q, want explicit %q (not merely unmarked)",
				name, got, audiencePlumbing)
		}
	}
}

// TestAudienceMarkerCoversEveryRegisteredVerb pins AC1: every command in the
// tree carries a queryable audience classification, readable without
// parsing --help output.
func TestAudienceMarkerCoversEveryRegisteredVerb(t *testing.T) {
	root := sharedTestRoot(t)
	for _, cmd := range root.Commands() {
		if cmd.Annotations[audienceAnnotationKey] == "" {
			t.Errorf("%q carries no audience marker", cmd.Name())
		}
	}
}

// TestAudienceClassifiesThePlumbingSet pins AC2: completion, help, init,
// path, and version are plumbing; every other registered verb is
// user-facing.
func TestAudienceClassifiesThePlumbingSet(t *testing.T) {
	root := sharedTestRoot(t)
	plumbing := map[string]bool{
		"completion": true,
		"help":       true,
		"init":       true,
		"path":       true,
		"version":    true,
	}
	for _, cmd := range root.Commands() {
		want := plumbing[cmd.Name()]
		got := !IsUserFacing(cmd)
		if got != want {
			t.Errorf("%q: plumbing=%v, want %v", cmd.Name(), got, want)
		}
	}
}

// TestAudienceCoversDeprecatedStubs pins AC3: the hidden serve stub
// answer the audience query through the same marker mechanism as real
// commands (a queryable Annotations entry) rather than being left out of
// it. AC2 does not name them plumbing, so — like every verb it does not
// name — they classify as user-facing; AC3 only requires that the
// classification exists and is asked the same way, not that it differs.
func TestAudienceCoversDeprecatedStubs(t *testing.T) {
	root := sharedTestRoot(t)
	for _, name := range []string{"serve"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("finding %q: %v", name, err)
		}
		if cmd.Annotations[audienceAnnotationKey] == "" {
			t.Errorf("%q carries no audience marker", name)
		}
		if !IsUserFacing(cmd) {
			t.Errorf("%q classified plumbing, want user-facing per AC2", name)
		}
	}
}

// TestBootstrapSkipMarksExactlyStorelessVerbs pins beans-bv90 AC-2/AC-3
// (SC-01): the bootstrap-skip marker is set if and only if the command is
// init, prime, version, tui, run, help, or completion -- the exact set
// PersistentPreRunE used to decide by comparing cmd.Name() against a literal
// string set.
func TestBootstrapSkipMarksExactlyStorelessVerbs(t *testing.T) {
	root := sharedTestRoot(t)
	skip := map[string]bool{
		"init":    true,
		"prime":   true,
		"version": true,
		"tui":     true, // the Lobby outside a repo has no store to load
		"run":     true, // global tasks run outside any project
		// cobra's text-only built-ins need no store either (beans-mgx8)
		"help":       true,
		"completion": true,
	}
	for _, cmd := range root.Commands() {
		want := skip[cmd.Name()]
		got := IsBootstrapSkip(cmd)
		if got != want {
			t.Errorf("%q: bootstrap-skip=%v, want %v", cmd.Name(), got, want)
		}
	}
}

// TestBootstrapSkipMarkerIsDistinctFromAudienceMarker pins AC-4/AC-5: the
// bootstrap-skip set {init, prime, version} and the audience-plumbing set
// {completion, help, init, path, version} are independent axes -- prime is
// bootstrap-skip but audience-user-facing, while path is
// audience-plumbing but not bootstrap-skip. Collapsing the two markers
// would flip one of these four checks.
func TestBootstrapSkipMarkerIsDistinctFromAudienceMarker(t *testing.T) {
	root := sharedTestRoot(t)

	prime, _, err := root.Find([]string{"prime"})
	if err != nil {
		t.Fatalf("finding %q: %v", "prime", err)
	}
	if !IsBootstrapSkip(prime) {
		t.Error("prime: bootstrap-skip=false, want true")
	}
	if !IsUserFacing(prime) {
		t.Error("prime: user-facing=false, want true (audience marker must be unaffected)")
	}

	for _, name := range []string{"path"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("finding %q: %v", name, err)
		}
		if IsBootstrapSkip(cmd) {
			t.Errorf("%q: bootstrap-skip=true, want false", name)
		}
		if IsUserFacing(cmd) {
			t.Errorf("%q: user-facing=true, want false (still audience-plumbing)", name)
		}
	}

	if bootstrapSkipAnnotationKey == audienceAnnotationKey {
		t.Error("bootstrapSkipAnnotationKey must be distinct from audienceAnnotationKey")
	}
}

// TestBootstrapSkipDoesNotMarkPick pins the beans-bv90 investigation: pick
// reads from the already-loaded store (pick.go's own RunE comment) and adds
// itself to no skip-list, so it must not carry the bootstrap-skip marker.
func TestBootstrapSkipDoesNotMarkPick(t *testing.T) {
	root := sharedTestRoot(t)
	pick, _, err := root.Find([]string{"pick"})
	if err != nil {
		t.Fatalf("finding %q: %v", "pick", err)
	}
	if IsBootstrapSkip(pick) {
		t.Error("pick: bootstrap-skip=true, want false (pick requires a loaded store)")
	}
}

// TestPersistentPreRunEOnlySkipsStoreForBootstrapSkipVerbs exercises
// PersistentPreRunE itself (internal/commands/root.go:39-45), not just the
// IsBootstrapSkip annotation read in isolation, over a directory with no
// reachable store. A bootstrap-skip verb (version) must return with no
// store-load error; a non-skip verb (list) must fail with the store-load
// error resolveStorePath produces. This is the observable difference a
// mutation of root.go:43 back to `if false` or the old cmd.Name() literal
// would erase without any of this file's other tests noticing, since they
// only assert the annotation, never PersistentPreRunE's own behaviour.
func TestPersistentPreRunEOnlySkipsStoreForBootstrapSkipVerbs(t *testing.T) {
	t.Chdir(t.TempDir())
	neutralizeStoreEnv(t)

	root := sharedTestRoot(t)

	resetFlags(root)
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"version"})
	if _, err := root.ExecuteC(); err != nil {
		t.Errorf("version: got error %v over a storeless directory, want nil (bootstrap-skip)", err)
	}

	resetFlags(root)
	outBuf.Reset()
	errBuf.Reset()
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"list"})
	_, err := root.ExecuteC()
	if err == nil {
		t.Fatal("list: got nil error over a storeless directory, want the store-load error")
	}
	if !strings.Contains(err.Error(), "no "+config.DefaultStoreDir+" directory found") {
		t.Errorf("list: error %q does not report the missing store", err.Error())
	}
}
