package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MainCheckoutRoot returns the root that identifies the repository of dir:
// the toplevel of a main checkout, or the main worktree of a repository for a
// linked worktree. For a repository with a bare main directory, such as
// proj.git with its worktrees beside it, that is the bare directory itself.
//
// A linked worktree counts only when its gitdir points back at the .git file
// of dir. A directory with a copied .git file that names a worktree of another
// repository is no checkout of that repository: the result is ("", false), as
// it is outside git.
func MainCheckoutRoot(dir string) (string, bool) {
	top, ok := Toplevel(dir)
	if !ok {
		return "", false
	}
	commonDir, err := gitRevParse(dir, "--git-common-dir")
	if err != nil {
		return "", false
	}
	gitDir, err := gitRevParse(dir, "--git-dir")
	if err != nil {
		return "", false
	}
	commonDir, gitDir = resolveGitPath(dir, commonDir), resolveGitPath(dir, gitDir)
	if commonDir == gitDir {
		return top, true // a main checkout
	}
	back, err := os.ReadFile(filepath.Join(gitDir, "gitdir"))
	if err != nil || !samePath(strings.TrimSpace(string(back)), filepath.Join(top, ".git")) {
		return "", false
	}
	out, err := exec.Command("git", "-C", dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", false
	}
	first, _, _ := strings.Cut(string(out), "\n")
	root, found := strings.CutPrefix(first, "worktree ")
	if !found || root == "" {
		return "", false
	}
	return root, true
}

// samePath reports whether a and b name the same file once symlinks are
// resolved; a path that cannot be resolved is compared as written.
func samePath(a, b string) bool {
	if r, err := filepath.EvalSymlinks(a); err == nil {
		a = r
	}
	if r, err := filepath.EvalSymlinks(b); err == nil {
		b = r
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
