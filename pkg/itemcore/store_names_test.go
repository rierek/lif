package itemcore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// newTestCoreWithConfig loads a store holding files under cfg.
func newTestCoreWithConfig(t *testing.T, cfg *config.Config, files map[string]string) *Core {
	t.Helper()
	dir := filepath.Join(cfg.ConfigDir(), config.DefaultStoreDir)
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := New(dir, cfg)
	c.SetWarnWriter(nil)
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	return c
}

// The store directory a message names is the one the store really has: .lif
// in a new layout, .beans in a legacy one.
func orphanBackupLog(t *testing.T, storeName string, backups ...string) string {
	t.Helper()
	repo := t.TempDir()
	for _, b := range backups {
		dir := filepath.Join(repo, b)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "tp-aaaa--test.md"), []byte("---\ntitle: T\nstatus: todo\ntype: task\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.DefaultWithPrefix("tp-")
	cfg.SetConfigDir(repo)
	c := New(filepath.Join(repo, storeName), cfg)
	var buf bytes.Buffer
	c.SetWarnWriter(&buf)
	if err := c.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return buf.String()
}
func TestOrphanBackupWarnings_NameTheStoreDirectoryInUse(t *testing.T) {
	cases := []struct{ store, backupPrefix string }{
		{config.DefaultStoreDir, config.BackupDirPrefix},
		{config.LegacyDefaultStoreDir, config.LegacyBackupDirPrefix},
	}
	for _, tc := range cases {
		t.Run(tc.store, func(t *testing.T) {
			repaired := orphanBackupLog(t, tc.store, tc.backupPrefix+"1000")
			if !strings.Contains(repaired, "repaired "+tc.store+" from backup") {
				t.Errorf("repair warning = %q, want it to name %q", repaired, tc.store)
			}

			multiple := orphanBackupLog(t, tc.store, tc.backupPrefix+"1000", tc.backupPrefix+"2000")
			if !strings.Contains(multiple, "backups of the "+tc.store+" directory") {
				t.Errorf("multiple-backup warning = %q, want it to name %q", multiple, tc.store)
			}
		})
	}
}

// writeConfigWithKey writes a config file whose section key is key and loads it.
func writeConfigWithKey(t *testing.T, file, key, prefix string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, file)
	if err := os.WriteFile(path, []byte(key+":\n    prefix: "+prefix+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}

func TestPrefixConsistency_NamesTheSectionKeyInUse(t *testing.T) {
	for _, tc := range []struct{ file, key string }{
		{config.ConfigFileName, config.SectionKey},
		{config.LegacyConfigFileName, config.LegacySectionKey},
	} {
		t.Run(tc.key, func(t *testing.T) {
			cfg := writeConfigWithKey(t, tc.file, tc.key, "aa-")
			c := newTestCoreWithConfig(t, cfg, map[string]string{
				"bb-aaaa--test.md": "---\ntitle: T\nstatus: todo\ntype: task\n---\n",
			})
			msg := c.ValidatePrefixConsistency()
			want := "config." + tc.key + ".prefix"
			if !strings.Contains(msg, want) {
				t.Errorf("ValidatePrefixConsistency() = %q, want it to contain %q", msg, want)
			}
		})
	}
}

func TestRebrandConfigWriteError_NamesTheConfigFileInUse(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod does not prevent the write")
	}
	for _, tc := range []struct{ file, key string }{
		{config.ConfigFileName, config.SectionKey},
		{config.LegacyConfigFileName, config.LegacySectionKey},
	} {
		t.Run(tc.file, func(t *testing.T) {
			cfg := writeConfigWithKey(t, tc.file, tc.key, "aa-")
			dir := cfg.ConfigDir()
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			c := New(filepath.Join(dir, config.DefaultStoreDir), cfg)
			err := c.writeRebrandConfig("bb-")
			if err == nil {
				t.Fatal("writeRebrandConfig() error = nil in a read-only directory")
			}
			if !strings.Contains(err.Error(), "writing "+tc.file+":") {
				t.Errorf("error = %q, want it to name %q", err, tc.file)
			}
		})
	}
}

// The failure warnings of the orphan-backup repair name the store directory
// in use too. A read-only repository directory makes each of them fire.
func TestOrphanBackupFailureWarnings_NameTheStoreDirectoryInUse(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod does not prevent the failures")
	}
	for _, store := range []string{config.DefaultStoreDir, config.LegacyDefaultStoreDir} {
		run := func(t *testing.T, mkStore, mkBackup bool) string {
			repo := t.TempDir()
			if mkStore {
				if err := os.MkdirAll(filepath.Join(repo, store), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if mkBackup {
				backup := filepath.Join(repo, config.BackupDirPrefix+"1000")
				if err := os.MkdirAll(backup, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(backup, "a.md"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(repo, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(repo, 0o755) })
			cfg := config.DefaultWithPrefix("tp-")
			cfg.SetConfigDir(repo)
			c := New(filepath.Join(repo, store), cfg)
			var buf bytes.Buffer
			c.SetWarnWriter(&buf)
			_ = c.Load()
			return buf.String()
		}
		t.Run(store+"/create", func(t *testing.T) {
			if got := run(t, false, false); !strings.Contains(got, "failed to create "+store+" directory") {
				t.Errorf("warnings = %q, want the failed create to name %q", got, store)
			}
		})
		t.Run(store+"/clear and repair", func(t *testing.T) {
			got := run(t, true, true)
			for _, want := range []string{"failed to clear empty " + store + " directory", "failed to repair " + store + " from backup"} {
				if !strings.Contains(got, want) {
					t.Errorf("warnings = %q, lack %q", got, want)
				}
			}
		})
		t.Run(store+"/multiple backups create", func(t *testing.T) {
			repo := t.TempDir()
			for _, n := range []string{"1", "2"} {
				b := filepath.Join(repo, config.BackupDirPrefix+n)
				if err := os.MkdirAll(b, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(b, "a.md"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(repo, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(repo, 0o755) })
			cfg := config.DefaultWithPrefix("tp-")
			cfg.SetConfigDir(repo)
			c := New(filepath.Join(repo, store), cfg)
			var buf bytes.Buffer
			c.SetWarnWriter(&buf)
			_ = c.Load()
			if !strings.Contains(buf.String(), "failed to create "+store+" directory") {
				t.Errorf("warnings = %q, want the failed create to name %q", buf.String(), store)
			}
		})
	}
}

func TestMixedPrefixConsistency_NamesTheSectionKeyInUse(t *testing.T) {
	for _, tc := range []struct{ file, key string }{
		{config.ConfigFileName, config.SectionKey},
		{config.LegacyConfigFileName, config.LegacySectionKey},
	} {
		t.Run(tc.key, func(t *testing.T) {
			cfg := writeConfigWithKey(t, tc.file, tc.key, "aa-")
			body := "---\ntitle: T\nstatus: todo\ntype: task\n---\n"
			c := newTestCoreWithConfig(t, cfg, map[string]string{"bb-aaaa--a.md": body, "cc-bbbb--b.md": body})
			msg := c.ValidatePrefixConsistency()
			if !strings.Contains(msg, "mixed-prefix") || !strings.Contains(msg, "config."+tc.key+".prefix") {
				t.Errorf("ValidatePrefixConsistency() = %q, want a mixed-prefix message naming config.%s.prefix", msg, tc.key)
			}
		})
	}
}
