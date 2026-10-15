package itemcore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// Crash-backup orphan scan (R-11): detection and repair treat `.lif.bak-*`
// and `.beans.bak-*` backups alike; the atomic swap names its own staging and
// backup directories with the lif names. Every test goes through the
// production functions.

const orphanItem = "---\n# tp-xxbb\ntitle: Recovered\nstatus: todo\ntype: task\n---\n"

// makeBackup creates a non-empty backup directory <name> next to the store.
func makeBackup(t *testing.T, repo, name string) string {
	t.Helper()
	dir := filepath.Join(repo, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tp-xxbb--test.md"), []byte(orphanItem), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newOrphanCore(repo string) *Core {
	cfg := config.DefaultWithPrefix("tp-")
	cfg.SetConfigDir(repo)
	return New(filepath.Join(repo, config.DefaultStoreDir), cfg)
}

func TestDetectOrphanBackup_repairsEitherPrefix(t *testing.T) {
	for _, name := range []string{".lif.bak-1234567890", ".beans.bak-1234567890"} {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			backup := makeBackup(t, repo, name)
			c := newOrphanCore(repo)

			if err := c.Load(); err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if _, err := os.Stat(backup); !os.IsNotExist(err) {
				t.Errorf("backup %s was not consumed by the repair (stat err = %v)", name, err)
			}
			b, err := c.Get("tp-xxbb")
			if err != nil {
				t.Fatalf("item not found after repair: %v", err)
			}
			if b.Title != "Recovered" {
				t.Errorf("Title = %q, want %q", b.Title, "Recovered")
			}
		})
	}
}

// One backup of each prefix is two backups: as at baseline for two backups,
// nothing is repaired and both stay for a manual recovery.
func TestDetectOrphanBackup_oneOfEachPrefixIsAmbiguous(t *testing.T) {
	repo := t.TempDir()
	lifBackup := makeBackup(t, repo, ".lif.bak-1000")
	legacyBackup := makeBackup(t, repo, ".beans.bak-2000")
	c := newOrphanCore(repo)

	if err := c.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	for _, dir := range []string{lifBackup, legacyBackup} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("backup %s was touched by an ambiguous scan: %v", filepath.Base(dir), err)
		}
	}
	if _, err := c.Get("tp-xxbb"); err == nil {
		t.Error("an item was loaded although neither backup may be repaired")
	}
}

func TestHasOrphanBackup_eitherPrefix(t *testing.T) {
	tests := []struct {
		name    string
		backups []string
		want    bool
	}{
		{"lif backup", []string{".lif.bak-1"}, true},
		{"legacy backup", []string{".beans.bak-1"}, true},
		{"one of each", []string{".lif.bak-1", ".beans.bak-2"}, true},
		{"unrelated sibling", []string{".other.bak-1"}, false},
		{"none", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			for _, b := range tt.backups {
				makeBackup(t, repo, b)
			}
			if got := HasOrphanBackup(filepath.Join(repo, config.DefaultStoreDir)); got != tt.want {
				t.Errorf("HasOrphanBackup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasOrphanBackup_emptyBackupDoesNotCount(t *testing.T) {
	repo := t.TempDir()
	for _, name := range []string{".lif.bak-1", ".beans.bak-2"} {
		if err := os.MkdirAll(filepath.Join(repo, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if HasOrphanBackup(filepath.Join(repo, config.DefaultStoreDir)) {
		t.Error("HasOrphanBackup() = true for empty backup directories")
	}
}

// The staging and backup directories the atomic swap creates carry the lif
// names, even for a project whose store directory still has the legacy name.
func TestStageAndSwap_usesLifNames(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})

	var seen []string
	orig := swapRename
	swapRename = func(oldpath, newpath string) error {
		entries, err := os.ReadDir(c.repoRoot())
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			seen = append(seen, e.Name())
		}
		return orig(oldpath, newpath)
	}
	defer func() { swapRename = orig }()

	if err := c.stageAndSwap(map[string][]byte{"tp-bbbb--b.md": []byte("x")}, nil); err != nil {
		t.Fatalf("stageAndSwap: %v", err)
	}

	staging := regexp.MustCompile(`^\.lif-staging-.+`)
	backup := regexp.MustCompile(`^\.lif\.bak-\d+$`)
	var gotStaging, gotBackup bool
	for _, name := range seen {
		switch {
		case staging.MatchString(name):
			gotStaging = true
		case backup.MatchString(name):
			gotBackup = true
		case strings.HasPrefix(name, ".beans-staging") || strings.HasPrefix(name, ".beans.bak-"):
			t.Errorf("swap created legacy-named directory %s", name)
		}
	}
	if !gotStaging || !gotBackup {
		t.Errorf("during the swap the repo held %v; want a .lif-staging-* staging and a .lif.bak-<n> backup directory",
			seen)
	}
}
