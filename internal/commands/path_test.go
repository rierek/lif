package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
)

// The store path is resolved in exactly one place (resolveStorePath). Scripts
// that need it must be able to ask for it rather than re-deriving it, so the
// command prints the bare path and nothing else.
func TestPathCmdPrintsResolvedStore(t *testing.T) {
	dir := t.TempDir()
	core = itemcore.New(dir, config.Default())

	var out bytes.Buffer
	pathCmd.SetOut(&out)
	pathCmd.Run(pathCmd, nil)

	got := out.String()
	if strings.TrimSpace(got) != dir {
		t.Errorf("expected bare store path %q, got %q", dir, got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Errorf("expected a single line, got %q", got)
	}
}

// Scripts call `lif path`, so the command has to be wired into the CLI, not
// merely defined.
func TestPathCmdIsRegistered(t *testing.T) {
	// The command objects and their flag variables are package-level
	// singletons, so RegisterCoreCommands may run only once per process.
	// sharedTestRoot owns that single registration.
	root := sharedTestRoot(t)

	for _, c := range root.Commands() {
		if c.Name() == "path" {
			return
		}
	}
	t.Fatal("expected a registered \"path\" command, found none")
}
