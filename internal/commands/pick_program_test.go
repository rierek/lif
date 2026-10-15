package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/item"
)

// TestRunPickWithEscIsSilentAndLeavesChromeOffStdout pins beans-gofn: esc
// (and ctrl+c) inside the core picker program returns errPickAborted,
// which is now silent (beans-lk8t), and every byte of the picker's own
// chrome goes to the injected output writer -- never to the command's own
// stdout, which stays completely empty (R-11 AC2/AC3).
func TestRunPickWithEscIsSilentAndLeavesChromeOffStdout(t *testing.T) {
	for _, key := range []struct {
		name  string
		bytes []byte
	}{
		{"esc", []byte{0x1b}},
		{"ctrl+c", []byte{0x03}},
	} {
		t.Run(key.name, func(t *testing.T) {
			items := []*item.Item{pickTestItem("beans-esc1", "Escapable")}

			inR, inW, err := os.Pipe()
			if err != nil {
				t.Fatalf("os.Pipe() error = %v", err)
			}
			var teaOut bytes.Buffer
			var cmdOut bytes.Buffer
			cmd := &cobra.Command{Use: "pick"}
			cmd.SetOut(&cmdOut)

			done := make(chan error, 1)
			go func() {
				done <- runPickWith(cmd, items, inR, &teaOut)
			}()

			if _, err := inW.Write(key.bytes); err != nil {
				t.Fatalf("writing key bytes: %v", err)
			}

			runErr := <-done
			inW.Close()
			inR.Close()

			if !output.IsSilent(runErr) {
				t.Errorf("runPickWith(%s) error = %v, want a silent error", key.name, runErr)
			}
			if cmdOut.Len() != 0 {
				t.Errorf("runPickWith(%s) wrote to cmd stdout: %q", key.name, cmdOut.String())
			}
			if teaOut.Len() == 0 {
				t.Errorf("runPickWith(%s) wrote no chrome to the injected output writer", key.name)
			}
		})
	}
}

// TestRunPickWithEnterPrintsIDToCommandStdoutOnly pins beans-gofn: selecting
// the only candidate with enter returns nil, prints exactly that item's ID
// to the command's own stdout, and the picker's tea output writer -- which
// only ever carries rendered chrome -- never receives that print call.
func TestRunPickWithEnterPrintsIDToCommandStdoutOnly(t *testing.T) {
	items := []*item.Item{pickTestItem("beans-pick9", "Pick Me")}

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	var teaOut bytes.Buffer
	var cmdOut bytes.Buffer
	cmd := &cobra.Command{Use: "pick"}
	cmd.SetOut(&cmdOut)

	done := make(chan error, 1)
	go func() {
		done <- runPickWith(cmd, items, inR, &teaOut)
	}()

	if _, err := inW.Write([]byte{'\r'}); err != nil {
		t.Fatalf("writing enter: %v", err)
	}

	runErr := <-done
	inW.Close()
	inR.Close()

	if runErr != nil {
		t.Fatalf("runPickWith(enter) error = %v, want nil", runErr)
	}
	if strings.TrimSpace(cmdOut.String()) != "beans-pick9" {
		t.Errorf("cmd stdout = %q, want the selected id", cmdOut.String())
	}
	// The final "print the id" call goes to cmd's stdout exclusively --
	// runPickWith never routes it through the tea output writer, so that
	// exact printed line must not appear there.
	if strings.Contains(teaOut.String(), "beans-pick9\n") {
		t.Errorf("the id print call leaked onto the tea output writer:\n%s", teaOut.String())
	}
}
