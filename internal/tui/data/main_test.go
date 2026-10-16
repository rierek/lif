package data

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain builds the lif binary from this checkout and puts it first on PATH,
// so the data layer is tested against the real CLI it drives, never against an
// installed (possibly older) lif.
func TestMain(m *testing.M) {
	binDir, err := os.MkdirTemp("", "lif-tui-data-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdtemp:", err)
		os.Exit(1)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve repo root:", err)
		os.RemoveAll(binDir)
		os.Exit(1)
	}

	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "lif"), "./cmd/lif")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building lif binary: %v\n%s", err, out)
		os.RemoveAll(binDir)
		os.Exit(1)
	}

	os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The data layer runs this lif: a developer's global hooks must not fire in tests.
	os.Setenv("LIF_NO_HOOKS", "1")

	// os.Exit skips defers, so remove the build dir explicitly.
	code := m.Run()
	os.RemoveAll(binDir)
	os.Exit(code)
}
