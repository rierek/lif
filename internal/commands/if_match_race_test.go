package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

// ifMatchWriter gives the arguments one side of a race passes to lif update in
// a round. Every writer changes a value that no other round and no other side
// writes, so a write never leaves the file as it was and every successful
// write moves the etag.
type ifMatchWriter func(round int, side string) []string

func setsAMark(round int, side string) []string {
	return []string{"--set", "mark=round" + strconv.Itoa(round) + side}
}

func changesTheTitle(round int, side string) []string {
	return []string{"--title", "Round " + strconv.Itoa(round) + " " + side}
}

// storeItemFiles lists the item files of the project in dir.
func storeItemFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, config.DefaultStoreDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range files {
		files[i] = filepath.Base(files[i])
	}
	return files
}

// beans-9r10: two lif processes that update one item with the same --if-match
// etag at the same moment must not both succeed, and must leave one file.
// Before the item file was locked from the check to the write, about every
// second round let both through and the file kept only one of the two changes.
// A title change renames the file once it is written, which is its own way to
// go wrong: the writer that lost wrote the old file name again beside the new
// one (review B1).
func TestSimultaneousIfMatchWritersExactlyOneWins(t *testing.T) {
	tests := []struct {
		name string
		a, b ifMatchWriter
	}{
		{"extra key against extra key", setsAMark, setsAMark},
		{"title against extra key", changesTheTitle, setsAMark},
		{"title against title", changesTheTitle, changesTheTitle},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const rounds = 15
			dir := integrityProject(t, map[string]string{
				"ex-race--race.md": integrityItem("ex-race", "Race", ""),
			})

			for round := range rounds {
				etag, stderr, exit := runLif(t, dir, "show", "ex-race", "--etag-only")
				if exit != 0 || etag == "" {
					t.Fatalf("round %d: lif show --etag-only: exit code = %d, etag = %q\nstderr: %s", round, exit, etag, stderr)
				}

				var outs, errs [2]bytes.Buffer
				var cmds [2]*exec.Cmd
				for i, writer := range []ifMatchWriter{tc.a, tc.b} {
					args := append([]string{"update", "ex-race"}, writer(round, string(rune('A'+i)))...)
					args = append(args, "--if-match", strings.TrimSpace(etag))
					cmds[i] = startLif(t, dir, &outs[i], &errs[i], args...)
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
				files := storeItemFiles(t, dir)
				if wins != 1 || len(files) != 1 {
					t.Fatalf("round %d: %d of 2 simultaneous --if-match writers succeeded and the store holds %d item files %v, want 1 and 1",
						round, wins, len(files), files)
				}
			}
		})
	}
}
