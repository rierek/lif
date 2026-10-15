package commands

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// startLif starts the built lif binary in dir with a private HOME and no
// store variables, and returns without waiting for it.
func startLif(t *testing.T, dir string, out, errOut *bytes.Buffer, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(completionTestBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), config.StoreEnvVar+"=", config.LegacyStoreEnvVar+"=")
	cmd.Stdout, cmd.Stderr = out, errOut
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting lif %v: %v", args, err)
	}
	return cmd
}

// beans-9r10: two lif processes that update one item with the same --if-match
// etag at the same moment must not both succeed. Before the item file was
// locked from the check to the write, about every second round let both
// through and the file kept only one of the two changes.
func TestSimultaneousIfMatchWritersExactlyOneWins(t *testing.T) {
	const rounds = 15
	dir := integrityProject(t, map[string]string{
		"ex-race--race.md": integrityItem("ex-race", "Race", ""),
	})

	for round := range rounds {
		etag, stderr, exit := runLif(t, dir, "show", "ex-race", "--etag-only")
		if exit != 0 || etag == "" {
			t.Fatalf("round %d: lif show --etag-only: exit code = %d, etag = %q\nstderr: %s", round, exit, etag, stderr)
		}

		// Each writer changes a different value, so no write is a no-op and
		// every successful write moves the etag.
		var outs, errs [2]bytes.Buffer
		var cmds [2]*exec.Cmd
		for i, writer := range []string{"A", "B"} {
			cmds[i] = startLif(t, dir, &outs[i], &errs[i], "update", "ex-race",
				"--set", "mark=round"+strconv.Itoa(round)+writer, "--if-match", strings.TrimSpace(etag))
		}

		wins := 0
		for i, cmd := range cmds {
			err := cmd.Wait()
			if err == nil {
				wins++
				continue
			}
			if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 || !strings.Contains(errs[i].String(), "etag mismatch") {
				t.Fatalf("round %d: the losing writer failed in another way: %v\nstderr: %s", round, err, errs[i].String())
			}
		}
		if wins != 1 {
			t.Fatalf("round %d: %d of 2 simultaneous --if-match writers succeeded, want exactly 1\nfile:\n%s",
				round, wins, readStoreFile(t, dir, "ex-race--race.md"))
		}
	}
}
