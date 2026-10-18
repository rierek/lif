package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func real(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestMainCheckoutRoot_LinkedWorktreeMapsToMainCheckout(t *testing.T) {
	main := initTestRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, main, "worktree", "add", wt, "-b", "side")

	got, ok := MainCheckoutRoot(wt)
	if !ok || real(t, got) != real(t, main) {
		t.Errorf("MainCheckoutRoot(worktree) = %q, %v; want %q", got, ok, main)
	}
	if got, ok := MainCheckoutRoot(main); !ok || real(t, got) != real(t, main) {
		t.Errorf("MainCheckoutRoot(main) = %q, %v; want %q", got, ok, main)
	}
}

// TRUST-04: worktrees of bare repositories in one directory have one root each.
func TestMainCheckoutRoot_BareRepositoriesInOneDirectoryAreDistinct(t *testing.T) {
	base := t.TempDir()
	roots := map[string]bool{}
	for _, n := range []string{"a", "b"} {
		bare := filepath.Join(base, n+".git")
		runGit(t, base, "init", "-q", "--bare", "-b", "main", bare)
		wt := filepath.Join(base, "w"+n)
		seed := t.TempDir()
		runGit(t, seed, "init", "-q", "-b", "main")
		runGit(t, seed, "commit", "--allow-empty", "-m", "i")
		runGit(t, seed, "push", "-q", bare, "main")
		runGit(t, bare, "worktree", "add", wt, "main")
		root, ok := MainCheckoutRoot(wt)
		if !ok {
			t.Fatalf("MainCheckoutRoot(%s) not ok", wt)
		}
		if real(t, root) != real(t, bare) {
			t.Errorf("root of %s = %q, want the bare dir %q", wt, root, bare)
		}
		roots[root] = true
	}
	if len(roots) != 2 {
		t.Errorf("roots = %v, want two distinct roots", roots)
	}
}

// TRUST-03: a .git file that names a linked worktree of another repository
// does not make a directory a checkout of that repository.
func TestMainCheckoutRoot_ForeignGitFileIsNotAWorktree(t *testing.T) {
	main := initTestRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, main, "worktree", "add", wt, "-b", "side")

	evil := t.TempDir()
	gitdir := filepath.Join(real(t, main), ".git", "worktrees", "wt")
	if err := os.WriteFile(filepath.Join(evil, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if root, ok := MainCheckoutRoot(evil); ok {
		t.Errorf("MainCheckoutRoot(spoofed dir) = %q, true; want false", root)
	}
}

func TestMainCheckoutRoot_NotGit(t *testing.T) {
	if root, ok := MainCheckoutRoot(t.TempDir()); ok {
		t.Errorf("MainCheckoutRoot = %q, true; want false", root)
	}
}
