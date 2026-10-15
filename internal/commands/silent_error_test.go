package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rierek/lif/internal/output"
)

// TestSilentErrorWritesNothing pins beans-lk8t: reportExecutionError must
// skip both stderr lines for an error built by output.Silent, the same way
// it already skips them for an Emitted one -- while the process still
// signals failure to its own caller (a non-nil err out of ExecuteC/RunE).
func TestSilentErrorWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "beans"}
	cmd.SetErr(&buf)

	reportExecutionError(cmd, output.Silent("beans pick: no bean selected"))

	if buf.String() != "" {
		t.Errorf("a silent error printed output:\n%s", buf.String())
	}
}

// TestPlainErrorStillWritesTwoLines guards the other half: a plain error
// (neither Emitted nor Silent) keeps getting both the error line and the
// usage pointer.
func TestPlainErrorStillWritesTwoLines(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "beans"}
	cmd.SetErr(&buf)

	reportExecutionError(cmd, errors.New("something broke"))

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "something broke") {
		t.Errorf("first line missing error text: %q", lines[0])
	}
}
