package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Items.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", cfg.Items.IDLength)
	}
	if cfg.Items.Prefix != "" {
		t.Errorf("Prefix = %q, want empty", cfg.Items.Prefix)
	}
	if cfg.Items.DefaultStatus != "todo" {
		t.Errorf("DefaultStatus = %q, want \"todo\"", cfg.Items.DefaultStatus)
	}
	if cfg.Items.DefaultType != "task" {
		t.Errorf("DefaultType = %q, want \"task\"", cfg.Items.DefaultType)
	}
	// Both types and statuses are hardcoded
	if len(DefaultTypes) != 5 {
		t.Errorf("len(DefaultTypes) = %d, want 5", len(DefaultTypes))
	}
	if len(DefaultStatuses) != 5 {
		t.Errorf("len(DefaultStatuses) = %d, want 5", len(DefaultStatuses))
	}
}

func TestDefaultWithPrefix(t *testing.T) {
	cfg := DefaultWithPrefix("myapp-")

	if cfg.Items.Prefix != "myapp-" {
		t.Errorf("Prefix = %q, want \"myapp-\"", cfg.Items.Prefix)
	}
	// Other defaults should still apply
	if cfg.Items.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", cfg.Items.IDLength)
	}
}

func TestIsValidStatus(t *testing.T) {
	cfg := Default()

	tests := []struct {
		status string
		want   bool
	}{
		{"draft", true},
		{"todo", true},
		{"in-progress", true},
		{"completed", true},
		{"scrapped", true},
		{"invalid", false},
		{"", false},
		{"TODO", false}, // case sensitive
		// Old status names should no longer be valid
		{"open", false},
		{"done", false},
		{"ready", false},
		{"not-ready", false},
		{"backlog", false}, // renamed to draft
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := cfg.IsValidStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusList(t *testing.T) {
	cfg := Default()
	got := cfg.StatusList()
	want := []string{"in-progress", "todo", "draft", "completed", "scrapped"}

	if len(got) != len(want) {
		t.Fatalf("StatusList() has %d entries, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("StatusList()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestStatusNames(t *testing.T) {
	cfg := Default()
	got := cfg.StatusNames()

	if len(got) != 5 {
		t.Fatalf("len(StatusNames()) = %d, want 5", len(got))
	}
	expected := []string{"in-progress", "todo", "draft", "completed", "scrapped"}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("StatusNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestGetStatus(t *testing.T) {
	cfg := Default()

	t.Run("existing status", func(t *testing.T) {
		s := cfg.GetStatus("todo")
		if s == nil {
			t.Fatal("GetStatus(\"todo\") = nil, want non-nil")
		}
		if s.Name != "todo" {
			t.Errorf("Name = %q, want \"todo\"", s.Name)
		}
		if s.Color != "green" {
			t.Errorf("Color = %q, want \"green\"", s.Color)
		}
	})

	t.Run("non-existing status", func(t *testing.T) {
		s := cfg.GetStatus("invalid")
		if s != nil {
			t.Errorf("GetStatus(\"invalid\") = %v, want nil", s)
		}
	})

	t.Run("old status names not valid", func(t *testing.T) {
		s := cfg.GetStatus("open")
		if s != nil {
			t.Errorf("GetStatus(\"open\") = %v, want nil (old status name)", s)
		}
		s = cfg.GetStatus("done")
		if s != nil {
			t.Errorf("GetStatus(\"done\") = %v, want nil (old status name)", s)
		}
		s = cfg.GetStatus("ready")
		if s != nil {
			t.Errorf("GetStatus(\"ready\") = %v, want nil (old status name)", s)
		}
	})
}

func TestGetDefaultStatus(t *testing.T) {
	cfg := Default()
	got := cfg.GetDefaultStatus()

	if got != "todo" {
		t.Errorf("GetDefaultStatus() = %q, want \"todo\"", got)
	}
}

func TestGetDefaultType(t *testing.T) {
	cfg := Default()
	got := cfg.GetDefaultType()

	if got != "task" {
		t.Errorf("GetDefaultType() = %q, want \"task\"", got)
	}
}

func TestIsArchiveStatus(t *testing.T) {
	cfg := Default()

	tests := []struct {
		status string
		want   bool
	}{
		{"completed", true},
		{"scrapped", true},
		{"draft", false},
		{"todo", false},
		{"in-progress", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := cfg.IsArchiveStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsArchiveStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestLoadNonExistent(t *testing.T) {
	// Load from non-existent directory should return defaults
	cfg, err := Load("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Should have default values
	if cfg.Items.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", cfg.Items.IDLength)
	}
}

func TestLoadAndSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a config (statuses are no longer stored in config)
	cfg := &Config{
		Items: ItemsConfig{
			Path:        ".beans",
			Prefix:      "test-",
			IDLength:    6,
			DefaultType: "bug",
		},
	}
	cfg.SetConfigDir(tmpDir)

	// Save it
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify values
	if loaded.Items.Prefix != "test-" {
		t.Errorf("Prefix = %q, want \"test-\"", loaded.Items.Prefix)
	}
	if loaded.Items.IDLength != 6 {
		t.Errorf("IDLength = %d, want 6", loaded.Items.IDLength)
	}
	if loaded.Items.DefaultType != "bug" {
		t.Errorf("DefaultType = %q, want \"bug\"", loaded.Items.DefaultType)
	}
	// Statuses are hardcoded, not stored in config
	if len(loaded.StatusNames()) != 5 {
		t.Errorf("len(StatusNames()) = %d, want 5", len(loaded.StatusNames()))
	}
}

func TestLoadDefaultPriorityAndTags(t *testing.T) {
	for _, section := range []string{SectionKey, LegacySectionKey} {
		t.Run(section, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ConfigFileName)
			body := section + ":\n  prefix: t-\n  default_priority: high\n  default_tags: [backend, api]\n"
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := cfg.GetDefaultPriority(); got != "high" {
				t.Errorf("GetDefaultPriority() = %q, want high", got)
			}
			if got := cfg.GetDefaultTags(); len(got) != 2 || got[0] != "backend" || got[1] != "api" {
				t.Errorf("GetDefaultTags() = %v, want [backend api]", got)
			}

			// Round trip keeps both keys under the file's own section key.
			if err := cfg.Save(dir); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			again, err := Load(path)
			if err != nil {
				t.Fatalf("Load() after Save error = %v", err)
			}
			if again.GetDefaultPriority() != "high" {
				t.Errorf("priority after round trip = %q, want high", again.GetDefaultPriority())
			}
			if got := again.GetDefaultTags(); len(got) != 2 || got[0] != "backend" || got[1] != "api" {
				t.Errorf("tags after round trip = %v, want [backend api]", got)
			}
		})
	}
}

func TestDefaultPriorityAndTagsUnset(t *testing.T) {
	var nilCfg *Config
	if nilCfg.GetDefaultPriority() != "" || nilCfg.GetDefaultTags() != nil {
		t.Error("nil config must return empty priority and no tags")
	}

	dir := t.TempDir()
	cfg := DefaultWithPrefix("t-")
	cfg.SetConfigDir(dir)
	if cfg.GetDefaultPriority() != "" || len(cfg.GetDefaultTags()) != 0 {
		t.Error("default config must leave both keys unset")
	}
	if err := cfg.Save(dir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "default_priority") || strings.Contains(string(data), "default_tags") {
		t.Errorf("saved file contains unset keys:\n%s", data)
	}
}

// toYAMLNode() built the agent, worktree and server mappings without three
// of their fields - agent.default_effort, worktree.fetch_timeout and
// server.cors_origins were all readable via their Get*/GetWorktree* getters
// but never written back out, so a Save() (e.g. via `lif rename`) on an
// already-loaded Config silently dropped them.
func TestSaveRoundTripsAgentServerAndWorktreeExtras(t *testing.T) {
	tmpDir := t.TempDir()

	fetchTimeout := 30
	cfg := &Config{
		Items: ItemsConfig{
			Path:     ".beans",
			Prefix:   "test-",
			IDLength: 4,
		},
		Agent: AgentConfig{
			DefaultEffort: "high",
		},
		Server: ServerConfig{
			CORSOrigins: []string{"https://example.com", "https://example.org"},
		},
		Worktree: WorktreeConfig{
			FetchTimeout: &fetchTimeout,
		},
	}
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := loaded.GetDefaultEffort(); got != "high" {
		t.Errorf("GetDefaultEffort() = %q, want \"high\"", got)
	}
	if got := loaded.GetCORSOrigins(); len(got) != 2 || got[0] != "https://example.com" || got[1] != "https://example.org" {
		t.Errorf("GetCORSOrigins() = %v, want [https://example.com https://example.org]", got)
	}
	if got := loaded.GetWorktreeFetchTimeout(); got != 30*time.Second {
		t.Errorf("GetWorktreeFetchTimeout() = %v, want 30s", got)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	// Create temp directory with minimal config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	// Write minimal config (missing id_length and default_type)
	minimalConfig := `beans:
  prefix: "my-"
`
	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Load it
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify defaults were applied
	if cfg.Items.IDLength != 4 {
		t.Errorf("IDLength default not applied: got %d, want 4", cfg.Items.IDLength)
	}
	// Statuses are hardcoded, always 5
	if len(cfg.StatusNames()) != 5 {
		t.Errorf("Hardcoded statuses: got %d, want 5", len(cfg.StatusNames()))
	}
	// DefaultStatus is always "todo"
	if cfg.GetDefaultStatus() != "todo" {
		t.Errorf("DefaultStatus: got %q, want \"todo\"", cfg.GetDefaultStatus())
	}
	// DefaultType should agree with Default(): "task" when the merged type
	// list carries it, which the built-in table always does.
	if cfg.Items.DefaultType != "task" {
		t.Errorf("DefaultType default not applied: got %q, want \"task\"", cfg.Items.DefaultType)
	}
}

func TestLoadAnchor(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), ConfigFileName)
		if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		return configPath
	}

	t.Run("repo-root is accepted", func(t *testing.T) {
		cfg, err := Load(write(t, "beans:\n  prefix: \"my-\"\n  anchor: repo-root\n"))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Items.Anchor != AnchorRepoRoot {
			t.Errorf("Anchor: got %q, want %q", cfg.Items.Anchor, AnchorRepoRoot)
		}
	})

	t.Run("absent anchor stays empty", func(t *testing.T) {
		cfg, err := Load(write(t, "beans:\n  prefix: \"my-\"\n"))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Items.Anchor != "" {
			t.Errorf("Anchor: got %q, want empty", cfg.Items.Anchor)
		}
	})

	t.Run("a misspelt anchor is an error, not a silent default", func(t *testing.T) {
		_, err := Load(write(t, "beans:\n  prefix: \"my-\"\n  anchor: repo-rooot\n"))
		if err == nil {
			t.Fatal("expected error for unknown anchor value, got nil")
		}
		if !strings.Contains(err.Error(), "beans.anchor") {
			t.Errorf("expected error to name beans.anchor, got %q", err.Error())
		}
	})
}

func TestLoadRequireFieldsOn(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), ConfigFileName)
		if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		return configPath
	}

	t.Run("unknown status is an error", func(t *testing.T) {
		_, err := Load(write(t, "beans:\n  require_fields_on:\n    bogus:\n      - commit\n"))
		if err == nil {
			t.Fatal("expected error for unknown status, got nil")
		}
		if !strings.Contains(err.Error(), "unknown status") {
			t.Errorf("expected error to contain \"unknown status\", got %q", err.Error())
		}
	})

	t.Run("reserved schema field is an error", func(t *testing.T) {
		_, err := Load(write(t, "beans:\n  require_fields_on:\n    completed:\n      - title\n"))
		if err == nil {
			t.Fatal("expected error for reserved field, got nil")
		}
		if !strings.Contains(err.Error(), "reserved front matter field") {
			t.Errorf("expected error to contain \"reserved front matter field\", got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "managed by lif, not an extra key") {
			t.Errorf("expected error to say the field is managed by lif, got %q", err.Error())
		}
	})

	t.Run("save-load roundtrip preserves policy and commit field", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{
			Items: ItemsConfig{
				Path:            ".beans",
				Prefix:          "test-",
				IDLength:        4,
				RequireFieldsOn: map[string][]string{"completed": {"commit"}},
				CommitField:     "commit",
			},
		}
		cfg.SetConfigDir(tmpDir)

		if err := cfg.Save(tmpDir); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		saved, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
		if err != nil {
			t.Fatal(err)
		}
		if want := "# Front matter keys that must be set when an item enters a status"; !strings.Contains(string(saved), want) {
			t.Errorf("saved config lacks the comment %q:\n%s", want, saved)
		}

		loaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got := loaded.RequiredFieldsFor("completed"); len(got) != 1 || got[0] != "commit" {
			t.Errorf("RequiredFieldsFor(\"completed\") = %v, want [commit]", got)
		}
		if got := loaded.GetCommitField(); got != "commit" {
			t.Errorf("GetCommitField() = %q, want \"commit\"", got)
		}
	})

	t.Run("unknown status error is deterministic across many keys", func(t *testing.T) {
		// validateRequireFieldsOn used to range over the map directly, so
		// which of several bad keys got reported first varied run to run
		// with Go's randomised map iteration. Sorting the keys first pins
		// it to the first bad status in lexical order.
		body := "beans:\n  require_fields_on:\n" +
			"    zzz-bogus:\n      - commit\n" +
			"    aaa-bogus:\n      - commit\n" +
			"    mmm-bogus:\n      - commit\n"
		for range 20 {
			_, err := Load(write(t, body))
			if err == nil {
				t.Fatal("expected error for unknown status, got nil")
			}
			if !strings.Contains(err.Error(), `"aaa-bogus"`) {
				t.Fatalf("expected the lexically first bad status \"aaa-bogus\" to be reported, got %q", err.Error())
			}
		}
	})

	t.Run("save preserves a key StatusList does not carry", func(t *testing.T) {
		// A Config built directly (not through Load, which validates every
		// key) can carry a require_fields_on entry for a status name
		// StatusList() doesn't know about. Save() used to iterate
		// StatusList() to get deterministic key order, which silently
		// dropped any such key instead of just writing it out.
		tmpDir := t.TempDir()
		cfg := &Config{
			Items: ItemsConfig{
				Path:            ".beans",
				Prefix:          "test-",
				IDLength:        4,
				RequireFieldsOn: map[string][]string{"not-a-real-status": {"commit"}},
			},
		}
		cfg.SetConfigDir(tmpDir)

		if err := cfg.Save(tmpDir); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}
		if !strings.Contains(string(data), "not-a-real-status") {
			t.Errorf("saved config does not contain %q:\n%s", "not-a-real-status", data)
		}
	})
}

func TestStatusesAreHardcoded(t *testing.T) {
	// Statuses are hardcoded and not configurable (like types)
	// Verify that any config only uses hardcoded statuses
	cfg := Default()

	// All hardcoded statuses should be valid
	hardcodedStatuses := []string{"draft", "todo", "in-progress", "completed", "scrapped"}
	for _, status := range hardcodedStatuses {
		if !cfg.IsValidStatus(status) {
			t.Errorf("IsValidStatus(%q) = false, want true", status)
		}
	}

	// Archive statuses should be completed and scrapped
	if !cfg.IsArchiveStatus("completed") {
		t.Error("IsArchiveStatus(\"completed\") = false, want true")
	}
	if !cfg.IsArchiveStatus("scrapped") {
		t.Error("IsArchiveStatus(\"scrapped\") = false, want true")
	}
	if cfg.IsArchiveStatus("todo") {
		t.Error("IsArchiveStatus(\"todo\") = true, want false")
	}
}

func TestIsValidType(t *testing.T) {
	cfg := Default()

	tests := []struct {
		typeName string
		want     bool
	}{
		{"epic", true},
		{"milestone", true},
		{"feature", true},
		{"bug", true},
		{"task", true},
		{"invalid", false},
		{"", false},
		{"TASK", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			got := cfg.IsValidType(tt.typeName)
			if got != tt.want {
				t.Errorf("IsValidType(%q) = %v, want %v", tt.typeName, got, tt.want)
			}
		})
	}
}

func TestTypeList(t *testing.T) {
	cfg := Default()
	got := cfg.TypeList()
	want := []string{"milestone", "epic", "feature", "bug", "task"}

	if len(got) != len(want) {
		t.Fatalf("TypeList() has %d entries, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("TypeList()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestGetType(t *testing.T) {
	cfg := Default()

	t.Run("existing type", func(t *testing.T) {
		typ := cfg.GetType("bug")
		if typ == nil {
			t.Fatal("GetType(\"bug\") = nil, want non-nil")
		}
		if typ.Name != "bug" {
			t.Errorf("Name = %q, want \"bug\"", typ.Name)
		}
		if typ.Color != "maroon" {
			t.Errorf("Color = %q, want \"maroon\"", typ.Color)
		}
	})

	t.Run("non-existing type", func(t *testing.T) {
		// GetType returns nil for unknown types
		typ := cfg.GetType("invalid-type")
		if typ != nil {
			t.Errorf("GetType(\"invalid-type\") = %v, want nil", typ)
		}
	})

	t.Run("all hardcoded types exist", func(t *testing.T) {
		expectedTypes := []string{"milestone", "epic", "bug", "feature", "task"}
		for _, typeName := range expectedTypes {
			typ := cfg.GetType(typeName)
			if typ == nil {
				t.Errorf("GetType(%q) = nil, want non-nil", typeName)
			}
		}
	})
}

func TestTypesAreHardcoded(t *testing.T) {
	// Types are hardcoded and not stored in config
	// Verify that saving and loading a config doesn't affect types

	tmpDir := t.TempDir()

	cfg := &Config{
		Items: ItemsConfig{
			Path:        ".beans",
			Prefix:      "test-",
			IDLength:    4,
			DefaultType: "task",
		},
	}
	cfg.SetConfigDir(tmpDir)

	// Save it
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load it back
	configPath := filepath.Join(tmpDir, ConfigFileName)
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Types should always come from DefaultTypes, not config
	if len(loaded.TypeNames()) != 5 {
		t.Errorf("len(TypeNames()) = %d, want 5", len(loaded.TypeNames()))
	}

	// All default types should be accessible
	for _, typeName := range []string{"milestone", "epic", "bug", "feature", "task"} {
		if !loaded.IsValidType(typeName) {
			t.Errorf("IsValidType(%q) = false, want true", typeName)
		}
	}

	// Statuses should also be hardcoded
	if len(loaded.StatusNames()) != 5 {
		t.Errorf("len(StatusNames()) = %d, want 5", len(loaded.StatusNames()))
	}
}

func TestTypeDescriptions(t *testing.T) {
	t.Run("hardcoded types have descriptions", func(t *testing.T) {
		cfg := Default()

		expectedDescriptions := map[string]string{
			"epic":      "A thematic container for related work; should have child items, not be worked on directly",
			"milestone": "A target release or checkpoint; group work that should ship together",
			"feature":   "A user-facing capability or enhancement",
			"bug":       "Something that is broken and needs fixing",
			"task":      "A concrete piece of work to complete (eg. a chore, or a sub-task for a feature)",
		}

		for typeName, expectedDesc := range expectedDescriptions {
			typ := cfg.GetType(typeName)
			if typ == nil {
				t.Errorf("GetType(%q) = nil, want non-nil", typeName)
				continue
			}
			if typ.Description != expectedDesc {
				t.Errorf("Type %q description = %q, want %q", typeName, typ.Description, expectedDesc)
			}
		}
	})

	t.Run("types in config file extend the defaults", func(t *testing.T) {
		// A type the config file names but the defaults do not know is
		// appended, not ignored (Task 2b: types became overridable).
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configYAML := `beans:
  prefix: "test-"
  id_length: 4
  default_status: open
statuses:
  - name: open
    color: green
types:
  - name: custom-type
    color: pink
    description: "A configured type"
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// The configured type is now valid.
		if !loaded.IsValidType("custom-type") {
			t.Error("IsValidType(\"custom-type\") = false, want true (configured types are appended)")
		}

		// Hardcoded types should still work
		if !loaded.IsValidType("bug") {
			t.Error("IsValidType(\"bug\") = false, want true")
		}
	})
}

func TestStatusDescriptions(t *testing.T) {
	t.Run("hardcoded statuses have descriptions", func(t *testing.T) {
		cfg := Default()

		expectedDescriptions := map[string]string{
			"draft":       "Needs refinement before it can be worked on",
			"todo":        "Ready to be worked on",
			"in-progress": "Currently being worked on",
			"completed":   "Finished successfully",
			"scrapped":    "Will not be done",
		}

		for statusName, expectedDesc := range expectedDescriptions {
			status := cfg.GetStatus(statusName)
			if status == nil {
				t.Errorf("GetStatus(%q) = nil, want non-nil", statusName)
				continue
			}
			if status.Description != expectedDesc {
				t.Errorf("Status %q description = %q, want %q", statusName, status.Description, expectedDesc)
			}
		}
	})

	t.Run("statuses in config file extend the defaults", func(t *testing.T) {
		// A status the config file names but the defaults do not know is
		// appended, not ignored (Task 2b: statuses became overridable).
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configYAML := `beans:
  prefix: "test-"
  id_length: 4
statuses:
  - name: custom-status
    color: pink
    description: "A configured status"
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// The configured status is now valid.
		if !loaded.IsValidStatus("custom-status") {
			t.Error("IsValidStatus(\"custom-status\") = false, want true (configured statuses are appended)")
		}

		// Hardcoded statuses should still work
		if !loaded.IsValidStatus("todo") {
			t.Error("IsValidStatus(\"todo\") = false, want true")
		}
	})
}

func TestFindConfig(t *testing.T) {
	t.Run("finds config in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte("beans:\n  prefix: test-\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		found, err := FindConfig(tmpDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}
		if found != configPath {
			t.Errorf("FindConfig() = %q, want %q", found, configPath)
		}
	})

	t.Run("finds config in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "sub", "dir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("MkdirAll error = %v", err)
		}

		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte("beans:\n  prefix: test-\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		found, err := FindConfig(subDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}
		if found != configPath {
			t.Errorf("FindConfig() = %q, want %q", found, configPath)
		}
	})

	t.Run("returns empty string when no config found", func(t *testing.T) {
		tmpDir := t.TempDir()

		found, err := FindConfigWithin(tmpDir, tmpDir)
		if err != nil {
			t.Fatalf("FindConfigWithin() error = %v", err)
		}
		if found != "" {
			t.Errorf("FindConfigWithin() = %q, want empty string", found)
		}
	})
}

func TestLoadFromDirectory(t *testing.T) {
	t.Run("loads config from directory with .beans.yml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `beans:
  path: custom-beans
  prefix: test-
  id_length: 6
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := LoadFromDirectory(tmpDir)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if cfg.Items.Path != "custom-beans" {
			t.Errorf("Beans.Path = %q, want \"custom-beans\"", cfg.Items.Path)
		}
		if cfg.Items.Prefix != "test-" {
			t.Errorf("Prefix = %q, want \"test-\"", cfg.Items.Prefix)
		}
		if cfg.Items.IDLength != 6 {
			t.Errorf("IDLength = %d, want 6", cfg.Items.IDLength)
		}
	})

	t.Run("returns default config when no config file exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg, err := LoadFromDirectoryWithin(tmpDir, tmpDir)
		if err != nil {
			t.Fatalf("LoadFromDirectoryWithin() error = %v", err)
		}
		if cfg.Items.Path != "" {
			t.Errorf("Items.Path = %q, want empty (no explicit path)", cfg.Items.Path)
		}
		if cfg.ConfigDir() != tmpDir {
			t.Errorf("ConfigDir() = %q, want %q", cfg.ConfigDir(), tmpDir)
		}
	})
}

func TestResolveItemsPath(t *testing.T) {
	t.Run("resolves relative path from config directory", func(t *testing.T) {
		cfg := &Config{
			Items: ItemsConfig{Path: "custom-beans"},
		}
		cfg.SetConfigDir("/project/root")

		got, err := cfg.ResolveItemsPath()
		want := "/project/root/custom-beans"
		if err != nil || got != want {
			t.Errorf("ResolveItemsPath() = %q, %v, want %q", got, err, want)
		}
	})

	t.Run("returns absolute path unchanged", func(t *testing.T) {
		cfg := &Config{
			Items: ItemsConfig{Path: "/absolute/path/to/beans"},
		}
		cfg.SetConfigDir("/project/root")

		got, err := cfg.ResolveItemsPath()
		want := "/absolute/path/to/beans"
		if err != nil || got != want {
			t.Errorf("ResolveItemsPath() = %q, %v, want %q", got, err, want)
		}
	})

	t.Run("uses default store directory", func(t *testing.T) {
		dir := t.TempDir()
		cfg := Default()
		cfg.SetConfigDir(dir)

		got, err := cfg.ResolveItemsPath()
		want := filepath.Join(dir, DefaultStoreDir)
		if err != nil || got != want {
			t.Errorf("ResolveItemsPath() = %q, %v, want %q", got, err, want)
		}
	})
}

func TestDefaultHasNoExplicitItemsPath(t *testing.T) {
	cfg := Default()
	if cfg.Items.Path != "" {
		t.Errorf("Default().Items.Path = %q, want empty (the default directory is resolved on disk)", cfg.Items.Path)
	}
}

func TestIsValidPriority(t *testing.T) {
	cfg := Default()

	tests := []struct {
		priority string
		want     bool
	}{
		{"critical", true},
		{"high", true},
		{"normal", true},
		{"low", true},
		{"deferred", true},
		{"", true}, // empty is valid (means no priority)
		{"invalid", false},
		{"CRITICAL", false}, // case sensitive
		{"medium", false},   // not a valid priority
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			got := cfg.IsValidPriority(tt.priority)
			if got != tt.want {
				t.Errorf("IsValidPriority(%q) = %v, want %v", tt.priority, got, tt.want)
			}
		})
	}
}

func TestPriorityList(t *testing.T) {
	cfg := Default()
	got := cfg.PriorityList()
	want := []string{"critical", "high", "normal", "low", "deferred"}

	if len(got) != len(want) {
		t.Fatalf("PriorityList() has %d entries, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("PriorityList()[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestPriorityNames(t *testing.T) {
	cfg := Default()
	got := cfg.PriorityNames()

	if len(got) != 5 {
		t.Fatalf("len(PriorityNames()) = %d, want 5", len(got))
	}
	expected := []string{"critical", "high", "normal", "low", "deferred"}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("PriorityNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestGetPriority(t *testing.T) {
	cfg := Default()

	t.Run("existing priority", func(t *testing.T) {
		p := cfg.GetPriority("high")
		if p == nil {
			t.Fatal("GetPriority(\"high\") = nil, want non-nil")
		}
		if p.Name != "high" {
			t.Errorf("Name = %q, want \"high\"", p.Name)
		}
		if p.Color != "yellow" {
			t.Errorf("Color = %q, want \"yellow\"", p.Color)
		}
	})

	t.Run("non-existing priority", func(t *testing.T) {
		p := cfg.GetPriority("invalid")
		if p != nil {
			t.Errorf("GetPriority(\"invalid\") = %v, want nil", p)
		}
	})

	t.Run("empty priority returns nil", func(t *testing.T) {
		p := cfg.GetPriority("")
		if p != nil {
			t.Errorf("GetPriority(\"\") = %v, want nil", p)
		}
	})
}

func TestPriorityDescriptions(t *testing.T) {
	cfg := Default()

	expectedDescriptions := map[string]string{
		"critical": "Urgent, blocking work. When possible, address immediately",
		"high":     "Important, should be done before normal work",
		"normal":   "Standard priority",
		"low":      "Less important, can be delayed",
		"deferred": "Explicitly pushed back, avoid doing unless necessary",
	}

	for priorityName, expectedDesc := range expectedDescriptions {
		p := cfg.GetPriority(priorityName)
		if p == nil {
			t.Errorf("GetPriority(%q) = nil, want non-nil", priorityName)
			continue
		}
		if p.Description != expectedDesc {
			t.Errorf("Priority %q description = %q, want %q", priorityName, p.Description, expectedDesc)
		}
	}
}

func TestDefaultPrioritiesCount(t *testing.T) {
	if len(DefaultPriorities) != 5 {
		t.Errorf("len(DefaultPriorities) = %d, want 5", len(DefaultPriorities))
	}
}

func TestSaveIncludesComments(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("myapp-")
	cfg.SetConfigDir(tmpDir)
	cfg.Items.Path = DefaultStoreDir

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configPath := filepath.Join(tmpDir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	content := string(data)

	// Verify header comment
	if !strings.Contains(content, "# Lif configuration") {
		t.Error("missing header comment 'Lif configuration'")
	}
	if !strings.Contains(content, "# See: https://github.com/xRiErOS/lif") {
		t.Error("missing header comment with URL")
	}

	// Verify field comments
	expectedComments := []string{
		"# Directory where item files are stored",
		"# Prefix for item IDs",
		"# Length of the random ID suffix",
		"# Default status for new items",
		"# Default type for new items",
		"# Port for the web UI (used by `lif-serve`)",
	}
	for _, comment := range expectedComments {
		if !strings.Contains(content, comment) {
			t.Errorf("missing comment: %s", comment)
		}
	}

	// Verify values are still correct by loading back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Items.Prefix != "myapp-" {
		t.Errorf("Prefix = %q, want \"myapp-\"", loaded.Items.Prefix)
	}
	if loaded.Items.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", loaded.Items.IDLength)
	}
	if loaded.Items.DefaultStatus != "todo" {
		t.Errorf("DefaultStatus = %q, want \"todo\"", loaded.Items.DefaultStatus)
	}
	if loaded.Items.DefaultType != "task" {
		t.Errorf("DefaultType = %q, want \"task\"", loaded.Items.DefaultType)
	}
	if loaded.GetServerPort() != DefaultServerPort {
		t.Errorf("ServerPort = %d, want %d", loaded.GetServerPort(), DefaultServerPort)
	}
}

func TestGetDefaultMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     PermissionMode
		expected PermissionMode
	}{
		{"empty defaults to act", "", PermissionModeAct},
		{"act", PermissionModeAct, PermissionModeAct},
		{"plan", PermissionModePlan, PermissionModePlan},
		{"invalid defaults to act", PermissionMode("invalid"), PermissionModeAct},
		{"yolo is backwards-compat alias for act", PermissionMode("yolo"), PermissionModeAct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Agent.DefaultMode = tt.mode
			got := cfg.GetDefaultMode()
			if got != tt.expected {
				t.Errorf("GetDefaultMode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsValidPermissionMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"act", true},
		{"yolo", true},
		{"plan", true},
		{"", false},
		{"invalid", false},
		{"ACT", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := IsValidPermissionMode(tt.mode)
			if got != tt.want {
				t.Errorf("IsValidPermissionMode(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestGetDefaultEffort(t *testing.T) {
	tests := []struct {
		effort   string
		expected string
	}{
		{"", ""},
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"max", "max"},
		{"ultra", "ultra"}, // raw value returned; validation is caller's responsibility
	}

	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			cfg := Default()
			cfg.Agent.DefaultEffort = tt.effort
			got := cfg.GetDefaultEffort()
			if got != tt.expected {
				t.Errorf("GetDefaultEffort() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsValidEffortLevel(t *testing.T) {
	tests := []struct {
		effort string
		want   bool
	}{
		{"low", true},
		{"medium", true},
		{"high", true},
		{"max", true},
		{"", false},
		{"ultra", false},
		{"High", false},
	}

	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			if got := IsValidEffortLevel(tt.effort); got != tt.want {
				t.Errorf("IsValidEffortLevel(%q) = %v, want %v", tt.effort, got, tt.want)
			}
		})
	}
}

func TestLoadAgentPermissionMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	configYAML := `beans:
  prefix: test-
agent:
  default_mode: plan
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GetDefaultMode() != PermissionModePlan {
		t.Errorf("GetDefaultMode() = %q, want %q", cfg.GetDefaultMode(), PermissionModePlan)
	}
}

func TestSaveIncludesAgentSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.Agent.DefaultMode = PermissionModePlan
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "agent:") {
		t.Error("expected agent section in saved config")
	}
	if !strings.Contains(content, "default_mode: plan") {
		t.Error("expected default_mode: plan in saved config")
	}
}

func TestSaveOmitsEmptyAgentSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.Agent.Enabled = nil    // explicitly clear
	cfg.Agent.DefaultMode = "" // explicitly clear
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	if strings.Contains(string(data), "agent:") {
		t.Error("expected agent section to be omitted when not configured")
	}
}

func TestDefaultIncludesAgentSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "agent:") {
		t.Error("expected default config to include agent section")
	}
	if !strings.Contains(content, "default_mode: act") {
		t.Error("expected default config to include default_mode: act")
	}
	if !strings.Contains(content, "enabled: true") {
		t.Error("expected default config to include enabled: true")
	}
}

func TestIsAgentEnabled(t *testing.T) {
	// Default config should have agent enabled
	cfg := Default()
	if !cfg.IsAgentEnabled() {
		t.Error("expected default config to have agent enabled")
	}

	// Explicitly disabled
	f := false
	cfg.Agent.Enabled = &f
	if cfg.IsAgentEnabled() {
		t.Error("expected agent to be disabled when set to false")
	}

	// Nil (unset) should default to true
	cfg.Agent.Enabled = nil
	if !cfg.IsAgentEnabled() {
		t.Error("expected agent to be enabled when Enabled is nil")
	}
}

func TestLoadAgentEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	// Write config with agent.enabled: false
	configContent := `
beans:
    prefix: "test-"
    id_length: 4
agent:
    enabled: false
`
	if err := os.WriteFile(filepath.Join(tmpDir, ConfigFileName), []byte(configContent), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.IsAgentEnabled() {
		t.Error("expected agent to be disabled when config has enabled: false")
	}
}

func TestSaveOmitsEmptyServerSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.Server.Port = 0 // zero value = omitted
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	if strings.Contains(string(data), "server:") {
		t.Error("expected server section to be omitted when port is 0")
	}
}

func TestGetWorktreeBaseRef(t *testing.T) {
	t.Run("returns default when not configured", func(t *testing.T) {
		cfg := Default()
		if cfg.GetWorktreeBaseRef() != DefaultWorktreeBaseRef {
			t.Errorf("GetWorktreeBaseRef() = %q, want %q", cfg.GetWorktreeBaseRef(), DefaultWorktreeBaseRef)
		}
	})

	t.Run("returns configured base ref", func(t *testing.T) {
		cfg := Default()
		cfg.Worktree.BaseRef = "origin/develop"
		if cfg.GetWorktreeBaseRef() != "origin/develop" {
			t.Errorf("GetWorktreeBaseRef() = %q, want \"origin/develop\"", cfg.GetWorktreeBaseRef())
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := `beans:
  prefix: test-
worktree:
  base_ref: origin/develop
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetWorktreeBaseRef() != "origin/develop" {
			t.Errorf("GetWorktreeBaseRef() = %q, want \"origin/develop\"", cfg.GetWorktreeBaseRef())
		}
	})
}

func TestSaveIncludesWorktreeSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.Worktree.BaseRef = "origin/develop"
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "worktree:") {
		t.Error("expected worktree section in saved config")
	}
	if !strings.Contains(content, "base_ref: origin/develop") {
		t.Error("expected base_ref: origin/develop in saved config")
	}
}

func TestWorktreeSetupAndRun(t *testing.T) {
	t.Run("returns empty strings by default", func(t *testing.T) {
		cfg := Default()
		if cfg.GetWorktreeSetup() != "" {
			t.Errorf("GetWorktreeSetup() = %q, want empty string", cfg.GetWorktreeSetup())
		}
		if cfg.GetWorktreeRun() != "" {
			t.Errorf("GetWorktreeRun() = %q, want empty string", cfg.GetWorktreeRun())
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := `beans:
  prefix: test-
worktree:
  setup: pnpm install
  run: mise dev
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetWorktreeSetup() != "pnpm install" {
			t.Errorf("GetWorktreeSetup() = %q, want \"pnpm install\"", cfg.GetWorktreeSetup())
		}
		if cfg.GetWorktreeRun() != "mise dev" {
			t.Errorf("GetWorktreeRun() = %q, want \"mise dev\"", cfg.GetWorktreeRun())
		}
	})

	t.Run("saves to config file", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := DefaultWithPrefix("test-")
		cfg.Worktree.Setup = "npm install"
		cfg.Worktree.Run = "npm run dev"
		cfg.SetConfigDir(tmpDir)

		if err := cfg.Save(tmpDir); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "setup: npm install") {
			t.Error("expected setup: npm install in saved config")
		}
		if !strings.Contains(content, "run: npm run dev") {
			t.Error("expected run: npm run dev in saved config")
		}
	})
}

func TestSaveAlwaysIncludesWorktreeStubs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.Worktree.BaseRef = "" // explicitly clear
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	content := string(data)
	// setup and run are always emitted as stubs so users discover them
	if !strings.Contains(content, "worktree:") {
		t.Error("expected worktree section to always be present")
	}
	if !strings.Contains(content, "setup:") {
		t.Error("expected setup stub in worktree section")
	}
	if !strings.Contains(content, "run:") {
		t.Error("expected run stub in worktree section")
	}
}

func TestGetWorktreeIntegrate(t *testing.T) {
	tests := []struct {
		name     string
		value    IntegrateMode
		expected IntegrateMode
	}{
		{"default (empty)", "", IntegrateModeLocal},
		{"local", IntegrateModeLocal, IntegrateModeLocal},
		{"pr", IntegrateModePR, IntegrateModePR},
		{"invalid value", "garbage", IntegrateModeLocal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Worktree.Integrate = tt.value
			if got := cfg.GetWorktreeIntegrate(); got != tt.expected {
				t.Errorf("GetWorktreeIntegrate() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestWorktreeIntegrateLoadAndSave(t *testing.T) {
	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := `beans:
  prefix: test-
worktree:
  integrate: pr
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetWorktreeIntegrate() != IntegrateModePR {
			t.Errorf("GetWorktreeIntegrate() = %q, want %q", cfg.GetWorktreeIntegrate(), IntegrateModePR)
		}
	})

	t.Run("saves to config file", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := DefaultWithPrefix("test-")
		cfg.Worktree.Integrate = IntegrateModePR
		cfg.SetConfigDir(tmpDir)

		if err := cfg.Save(tmpDir); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "integrate: pr") {
			t.Errorf("expected integrate: pr in saved config, got:\n%s", content)
		}
	})

	t.Run("default saves as local", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := DefaultWithPrefix("test-")
		cfg.SetConfigDir(tmpDir)

		if err := cfg.Save(tmpDir); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "integrate: local") {
			t.Errorf("expected integrate: local in saved config, got:\n%s", content)
		}
	})
}

func TestGetWorktreeFetchTimeout(t *testing.T) {
	t.Run("default is 10s", func(t *testing.T) {
		cfg := Default()
		if got := cfg.GetWorktreeFetchTimeout(); got != 10*time.Second {
			t.Errorf("GetWorktreeFetchTimeout() = %v, want 10s", got)
		}
	})

	t.Run("explicit zero disables fetch", func(t *testing.T) {
		cfg := Default()
		zero := 0
		cfg.Worktree.FetchTimeout = &zero
		if got := cfg.GetWorktreeFetchTimeout(); got != 0 {
			t.Errorf("GetWorktreeFetchTimeout() = %v, want 0", got)
		}
	})

	t.Run("custom value", func(t *testing.T) {
		cfg := Default()
		thirty := 30
		cfg.Worktree.FetchTimeout = &thirty
		if got := cfg.GetWorktreeFetchTimeout(); got != 30*time.Second {
			t.Errorf("GetWorktreeFetchTimeout() = %v, want 30s", got)
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := "beans:\n  prefix: test-\nworktree:\n  fetch_timeout: 5\n"
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got := cfg.GetWorktreeFetchTimeout(); got != 5*time.Second {
			t.Errorf("GetWorktreeFetchTimeout() = %v, want 5s", got)
		}
	})

	t.Run("loads zero from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := "beans:\n  prefix: test-\nworktree:\n  fetch_timeout: 0\n"
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got := cfg.GetWorktreeFetchTimeout(); got != 0 {
			t.Errorf("GetWorktreeFetchTimeout() = %v, want 0", got)
		}
	})
}

func TestGetServerPort(t *testing.T) {
	t.Run("returns default when not configured", func(t *testing.T) {
		cfg := Default()
		if cfg.GetServerPort() != DefaultServerPort {
			t.Errorf("GetServerPort() = %d, want %d", cfg.GetServerPort(), DefaultServerPort)
		}
	})

	t.Run("returns configured port", func(t *testing.T) {
		cfg := Default()
		cfg.Server.Port = 9000
		if cfg.GetServerPort() != 9000 {
			t.Errorf("GetServerPort() = %d, want 9000", cfg.GetServerPort())
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := `beans:
  prefix: test-
server:
  port: 3000
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetServerPort() != 3000 {
			t.Errorf("GetServerPort() = %d, want 3000", cfg.GetServerPort())
		}
	})
}

func TestGetProjectName(t *testing.T) {
	t.Run("returns empty string by default", func(t *testing.T) {
		cfg := Default()
		if cfg.GetProjectName() != "" {
			t.Errorf("GetProjectName() = %q, want empty string", cfg.GetProjectName())
		}
	})

	t.Run("returns configured name", func(t *testing.T) {
		cfg := Default()
		cfg.Project.Name = "my-project"
		if cfg.GetProjectName() != "my-project" {
			t.Errorf("GetProjectName() = %q, want \"my-project\"", cfg.GetProjectName())
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := `project:
  name: my-project
beans:
  prefix: test-
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetProjectName() != "my-project" {
			t.Errorf("GetProjectName() = %q, want \"my-project\"", cfg.GetProjectName())
		}
	})
}

func TestSaveIncludesProjectSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.Project.Name = "my-app"
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "project:") {
		t.Error("expected project section in saved config")
	}
	if !strings.Contains(content, "name: my-app") {
		t.Error("expected name: my-app in saved config")
	}
}

func TestSaveOmitsEmptyProjectSection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	if strings.Contains(string(data), "project:") {
		t.Error("expected project section to be omitted when not configured")
	}
}

func TestGetCORSOrigins(t *testing.T) {
	t.Run("returns defaults when not configured", func(t *testing.T) {
		cfg := Default()
		origins := cfg.GetCORSOrigins()
		if len(origins) != 2 {
			t.Fatalf("GetCORSOrigins() returned %d origins, want 2", len(origins))
		}
		if origins[0] != "http://localhost:*" {
			t.Errorf("origins[0] = %q, want %q", origins[0], "http://localhost:*")
		}
		if origins[1] != "http://127.0.0.1:*" {
			t.Errorf("origins[1] = %q, want %q", origins[1], "http://127.0.0.1:*")
		}
	})

	t.Run("returns configured origins", func(t *testing.T) {
		cfg := Default()
		cfg.Server.CORSOrigins = []string{"https://example.com"}
		origins := cfg.GetCORSOrigins()
		if len(origins) != 1 || origins[0] != "https://example.com" {
			t.Errorf("GetCORSOrigins() = %v, want [https://example.com]", origins)
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		configContent := `beans:
  prefix: test-
server:
  cors_origins:
    - "https://app.example.com"
    - "http://localhost:*"
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		origins := cfg.GetCORSOrigins()
		if len(origins) != 2 {
			t.Fatalf("GetCORSOrigins() returned %d origins, want 2", len(origins))
		}
		if origins[0] != "https://app.example.com" {
			t.Errorf("origins[0] = %q, want %q", origins[0], "https://app.example.com")
		}
	})
}

// TestSaveAtomic verifies that Save() writes atomically via temp file + rename.
// The test checks that no temp files are left behind after a successful save.
func TestSaveAtomic(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Items: ItemsConfig{
			Path:        ".beans",
			Prefix:      "test-",
			IDLength:    6,
			DefaultType: "task",
		},
	}
	cfg.SetConfigDir(tmpDir)

	// Save the config
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") || strings.Contains(entry.Name(), "tmp") {
			t.Errorf("Temp file left behind: %s", entry.Name())
		}
	}

	// Verify the target file exists
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}
}

// TestSavePreservesFileMode verifies that Save() preserves the existing file mode.
func TestSavePreservesFileMode(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Items: ItemsConfig{
			Path:   ".beans",
			Prefix: "test-",
		},
	}
	cfg.SetConfigDir(tmpDir)

	// Save the config once to create it
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}

	configPath := filepath.Join(tmpDir, ConfigFileName)

	// Change the file mode
	targetMode := os.FileMode(0600) // rw-------
	if err := os.Chmod(configPath, targetMode); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	// Verify the mode was set
	stat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if stat.Mode() != targetMode {
		t.Errorf("Initial mode = %#o, want %#o", stat.Mode(), targetMode)
	}

	// Save again with a different prefix
	cfg.Items.Prefix = "newprefix-"
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}

	// Check that the mode was preserved
	stat, err = os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() after second save error = %v", err)
	}
	if stat.Mode() != targetMode {
		t.Errorf("Preserved mode = %#o, want %#o", stat.Mode(), targetMode)
	}
}

// ConfigFile is what lets a caller tell "this repository declared its store"
// apart from "nothing was found, these are the defaults". Path resolution
// ranks a real declaration above the BEANS_PATH env var, so a defaulted
// config must never claim to have a file.
func TestConfigFile(t *testing.T) {
	t.Run("Default has no config file", func(t *testing.T) {
		if got := Default().ConfigFile(); got != "" {
			t.Errorf("Default().ConfigFile() = %q, want empty", got)
		}
	})

	t.Run("Load records the file it read", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ConfigFileName)
		if err := os.WriteFile(path, []byte("beans:\n  path: .beans\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.ConfigFile() != path {
			t.Errorf("ConfigFile() = %q, want %q", cfg.ConfigFile(), path)
		}
	})

	t.Run("Load of a missing file records nothing", func(t *testing.T) {
		cfg, err := Load(filepath.Join(t.TempDir(), ConfigFileName))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got := cfg.ConfigFile(); got != "" {
			t.Errorf("ConfigFile() = %q, want empty", got)
		}
	})

	t.Run("LoadFromDirectory records the file found upward", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ConfigFileName)
		if err := os.WriteFile(path, []byte("beans:\n  path: .beans\n"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		nested := filepath.Join(dir, "a", "b")
		if err := os.MkdirAll(nested, 0755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}

		cfg, err := LoadFromDirectory(nested)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if cfg.ConfigFile() != path {
			t.Errorf("ConfigFile() = %q, want %q", cfg.ConfigFile(), path)
		}
	})

	t.Run("LoadFromDirectory without a config file records nothing", func(t *testing.T) {
		cfg, err := LoadFromDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if got := cfg.ConfigFile(); got != "" {
			t.Errorf("ConfigFile() = %q, want empty", got)
		}
	})
}

func TestGetThemeDefaultsToMocha(t *testing.T) {
	if got := (&Config{}).GetTheme(); got != "mocha" {
		t.Errorf(`GetTheme() = %q, want "mocha"`, got)
	}
}

func TestGetThemeHonoursTheConfiguredValue(t *testing.T) {
	c := &Config{Display: DisplayConfig{Theme: "latte"}}
	if got := c.GetTheme(); got != "latte" {
		t.Errorf(`GetTheme() = %q, want "latte"`, got)
	}
}

func TestGetMaxWidthDefaultsTo110(t *testing.T) {
	if got := (&Config{}).GetMaxWidth(); got != 110 {
		t.Errorf("GetMaxWidth() = %d, want 110", got)
	}
}

func TestGetMaxWidthHonoursTheConfiguredValue(t *testing.T) {
	c := &Config{Display: DisplayConfig{MaxWidth: 140}}
	if got := c.GetMaxWidth(); got != 140 {
		t.Errorf("GetMaxWidth() = %d, want 140", got)
	}
}

func TestGetMaxWidthMinusOneMeansUncapped(t *testing.T) {
	c := &Config{Display: DisplayConfig{MaxWidth: -1}}
	if got := c.GetMaxWidth(); got != -1 {
		t.Errorf("GetMaxWidth() = %d, want -1", got)
	}
}

// Task 2b: statuses, types and priorities become overridable from
// Config.Statuses / Config.Types / Config.Priorities, merged entry by entry
// against the built-in defaults.

func TestTypeListFallsBackToTheDefaults(t *testing.T) {
	c := Default()
	if len(c.TypeList()) != len(DefaultTypes) {
		t.Errorf("TypeList() has %d entries, want the %d defaults",
			len(c.TypeList()), len(DefaultTypes))
	}
}

func TestConfigOverridesASingleColour(t *testing.T) {
	c := Default()
	c.Types = []TypeOverride{{Name: "bug", Color: "#ff0000"}}

	got := c.GetType("bug")
	if got == nil || got.Color != "#ff0000" {
		t.Fatalf("GetType(\"bug\").Color = %v, want the override", got)
	}
	// Everything not named keeps its default.
	if other := c.GetType("epic"); other == nil || other.Color != "blue" {
		t.Errorf("GetType(\"epic\").Color = %v, want the default \"blue\"", other)
	}
	if len(c.TypeList()) != len(DefaultTypes) {
		t.Errorf("an override changed the list length to %d", len(c.TypeList()))
	}
}

func TestAnOverrideKeepsUnsetFields(t *testing.T) {
	c := Default()
	c.Types = []TypeOverride{{Name: "epic", Color: "#00ff00"}}
	got := c.GetType("epic")
	if got == nil {
		t.Fatal("GetType(\"epic\") = nil, want the merged entry")
	}
	// Overriding only the colour must not blank out fields the override
	// left unset, such as the description.
	if got.Description == "" {
		t.Error("overriding only the colour dropped the description")
	}
}

func TestAnUnknownNameIsAppended(t *testing.T) {
	c := Default()
	c.Statuses = []StatusOverride{{Name: "blocked", Color: "red"}}
	if got := c.GetStatus("blocked"); got == nil {
		t.Error("a status the defaults do not know should be appended")
	}
	if len(c.StatusList()) != len(DefaultStatuses)+1 {
		t.Errorf("StatusList() has %d entries, want defaults plus one", len(c.StatusList()))
	}
}

func TestStatusNamesFollowTheMergedList(t *testing.T) {
	c := Default()
	c.Statuses = []StatusOverride{{Name: "blocked", Color: "red"}}
	found := false
	for _, n := range c.StatusNames() {
		if n == "blocked" {
			found = true
		}
	}
	if !found {
		t.Error("StatusNames() does not reflect the merged list")
	}
}

func TestPriorityOverride(t *testing.T) {
	c := Default()
	c.Priorities = []PriorityOverride{{Name: "critical", Color: "#ff00ff"}}
	if got := c.GetPriority("critical"); got == nil || got.Color != "#ff00ff" {
		t.Errorf("GetPriority(\"critical\").Color = %v, want the override", got)
	}
}

func TestGettersToleratePlainDefaults(t *testing.T) {
	c := &Config{}
	if got := c.GetType("task"); got == nil {
		t.Error("a bare Config must still resolve the built-in types")
	}
}

func TestSaveRoundTripsTypeStatusAndPriorityOverrides(t *testing.T) {
	// Save() is called by more than `init` (e.g. `lif rename`). Without
	// serializing these fields, a Save() on an already-loaded config would
	// silently erase whatever the user configured.
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.Types = []TypeOverride{{Name: "bug", Color: "#ff00ff"}}
	cfg.Statuses = []StatusOverride{{Name: "blocked", Color: "red", Archive: boolPtr(true)}}
	cfg.Priorities = []PriorityOverride{{Name: "critical", Color: "#ff0000"}}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := reloaded.GetType("bug"); got == nil || got.Color != "#ff00ff" {
		t.Errorf("reloaded GetType(\"bug\").Color = %v, want the saved override", got)
	}
	if got := reloaded.GetStatus("blocked"); got == nil || !got.Archive {
		t.Errorf("reloaded GetStatus(\"blocked\") = %v, want the saved override", got)
	}
	if got := reloaded.GetPriority("critical"); got == nil || got.Color != "#ff0000" {
		t.Errorf("reloaded GetPriority(\"critical\").Color = %v, want the saved override", got)
	}
}

// Fix round 1, Commit 1: a colour-only override must not silently flip
// Archive back to false (the field the previous, unconditional-assignment
// merge got wrong for "completed"). Archive is a pointer precisely so these
// two directions - "not mentioned" vs. "explicitly set" - are distinguishable.

func TestRecolouringAStatusPreservesItsArchiveFlag(t *testing.T) {
	c := Default()
	c.Statuses = []StatusOverride{{Name: "completed", Color: "#a6e3a1"}}

	got := c.GetStatus("completed")
	if got == nil {
		t.Fatal("GetStatus(\"completed\") = nil, want the merged entry")
	}
	if got.Color != "#a6e3a1" {
		t.Errorf("GetStatus(\"completed\").Color = %q, want the override", got.Color)
	}
	if !c.IsArchiveStatus("completed") {
		t.Error("IsArchiveStatus(\"completed\") = false, want true: a colour-only override must not touch Archive")
	}
}

func TestAnExplicitArchiveOverrideIsHonoured(t *testing.T) {
	c := Default()
	c.Statuses = []StatusOverride{{Name: "completed", Archive: boolPtr(false)}}

	if c.IsArchiveStatus("completed") {
		t.Error("IsArchiveStatus(\"completed\") = true, want false: an explicit archive: false must be honoured")
	}
}

// Fix round 1, Commit 2: before Task 2b, GetStatus/GetType/GetPriority/
// StatusNames/TypeNames read only the package-level Default* slices and
// never dereferenced their receiver, so they tolerated a nil *Config. Routing
// them through StatusList/TypeList/PriorityList (which read c.Statuses etc.)
// would panic on nil without an explicit guard - this is latent (no known
// call site passes a nil Config today), but it is a capability regression on
// a shared API, so the guard and the covering test are added independent of
// any live path.

func TestGettersToleratePlainDefaultsOnANilConfig(t *testing.T) {
	var c *Config

	if got := c.GetStatus("todo"); got == nil {
		t.Error("GetStatus(\"todo\") on a nil *Config = nil, want the built-in default")
	}
	if got := c.GetType("task"); got == nil {
		t.Error("GetType(\"task\") on a nil *Config = nil, want the built-in default")
	}
	if got := c.GetPriority("normal"); got == nil {
		t.Error("GetPriority(\"normal\") on a nil *Config = nil, want the built-in default")
	}
	if names := c.StatusNames(); len(names) != len(DefaultStatuses) {
		t.Errorf("StatusNames() on a nil *Config has %d entries, want the %d defaults", len(names), len(DefaultStatuses))
	}
	if names := c.TypeNames(); len(names) != len(DefaultTypes) {
		t.Errorf("TypeNames() on a nil *Config has %d entries, want the %d defaults", len(names), len(DefaultTypes))
	}
}

// The value-returning Get*/Is* accessors (as opposed to StatusList/TypeList/
// PriorityList, which already tolerated nil) panicked on a nil *Config
// because they dereferenced c.Items/c.Worktree/c.Agent/c.Project/c.Server/
// c.Display directly. Each must fall back to the same default it already
// returns for an unset field.
func TestValueGettersToleratesANilConfig(t *testing.T) {
	var c *Config

	if got := c.GetDefaultStatus(); got != "todo" {
		t.Errorf("GetDefaultStatus() = %q, want \"todo\"", got)
	}
	if got := c.GetDefaultType(); got != "" {
		t.Errorf("GetDefaultType() = %q, want \"\"", got)
	}
	if got := c.RequiredFieldsFor("completed"); got != nil {
		t.Errorf("RequiredFieldsFor(\"completed\") = %v, want nil", got)
	}
	if got := c.GetCommitField(); got != DefaultCommitField {
		t.Errorf("GetCommitField() = %q, want %q", got, DefaultCommitField)
	}
	if got := c.GetWorktreeBaseRef(); got != DefaultWorktreeBaseRef {
		t.Errorf("GetWorktreeBaseRef() = %q, want %q", got, DefaultWorktreeBaseRef)
	}
	if got := c.GetWorktreeSetup(); got != "" {
		t.Errorf("GetWorktreeSetup() = %q, want \"\"", got)
	}
	if got := c.GetWorktreeRun(); got != "" {
		t.Errorf("GetWorktreeRun() = %q, want \"\"", got)
	}
	if got := c.GetWorktreeFetchTimeout(); got != 10*time.Second {
		t.Errorf("GetWorktreeFetchTimeout() = %v, want 10s", got)
	}
	if got := c.GetWorktreeIntegrate(); got != IntegrateModeLocal {
		t.Errorf("GetWorktreeIntegrate() = %q, want %q", got, IntegrateModeLocal)
	}
	if got := c.IsAgentEnabled(); !got {
		t.Error("IsAgentEnabled() = false, want true")
	}
	if got := c.GetDefaultMode(); got != PermissionModeAct {
		t.Errorf("GetDefaultMode() = %q, want %q", got, PermissionModeAct)
	}
	if got := c.GetDefaultEffort(); got != "" {
		t.Errorf("GetDefaultEffort() = %q, want \"\"", got)
	}
	if got := c.GetProjectName(); got != "" {
		t.Errorf("GetProjectName() = %q, want \"\"", got)
	}
	if got := c.GetServerPort(); got != DefaultServerPort {
		t.Errorf("GetServerPort() = %d, want %d", got, DefaultServerPort)
	}
	if got := c.GetCORSOrigins(); len(got) != 2 {
		t.Errorf("GetCORSOrigins() = %v, want the two localhost defaults", got)
	}
	if got := c.GetTheme(); got != "mocha" {
		t.Errorf("GetTheme() = %q, want \"mocha\"", got)
	}
	if got := c.GetMaxWidth(); got != 110 {
		t.Errorf("GetMaxWidth() = %d, want 110", got)
	}
}

// Fix round 1, Commit 3: the same data-loss bug as Statuses/Types/Priorities,
// one struct over. toYAMLNode() did not know about DisplayConfig either, so
// Save() (e.g. via `lif rename`) silently dropped a configured theme/
// max_width from .beans.yml.

func TestSaveRoundTripsDisplaySettings(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.Display = DisplayConfig{Theme: "latte", MaxWidth: 140}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := reloaded.GetTheme(); got != "latte" {
		t.Errorf("reloaded GetTheme() = %q, want the saved \"latte\"", got)
	}
	if got := reloaded.GetMaxWidth(); got != 140 {
		t.Errorf("reloaded GetMaxWidth() = %d, want the saved 140", got)
	}
}

func TestSaveRoundTripsMaxWidthMinusOne(t *testing.T) {
	// -1 is a meaningful, non-zero override (disables the cap) and must not
	// be confused with "unset" the way omitempty would treat a plain 0.
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.Display = DisplayConfig{MaxWidth: -1}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := reloaded.GetMaxWidth(); got != -1 {
		t.Errorf("reloaded GetMaxWidth() = %d, want the saved -1", got)
	}
}

// Task 3: the built-in tables now name Catppuccin tones instead of hand-rolled
// colour words, so the mapping from status/type/priority to hue lives entirely
// in this config and is resolved elsewhere (internal/ui) against the active
// theme.

func TestTypeDefaultsCarryCatppuccinTones(t *testing.T) {
	want := map[string]string{
		"milestone": "mauve", "epic": "blue", "feature": "sapphire",
		"bug": "maroon", "task": "",
	}
	for _, tc := range DefaultTypes {
		if w, ok := want[tc.Name]; ok && tc.Color != w {
			t.Errorf("type %q colour = %q, want %q", tc.Name, tc.Color, w)
		}
	}
}

func TestOnlyContainersAreEmphasised(t *testing.T) {
	want := map[string]bool{
		"milestone": true, "epic": true,
		"feature": false, "bug": false, "task": false,
	}
	for _, tc := range DefaultTypes {
		if w, ok := want[tc.Name]; ok && tc.Emphasis != w {
			t.Errorf("type %q emphasis = %v, want %v", tc.Name, tc.Emphasis, w)
		}
	}
}

func TestStatusDefaultsFormAMaturityRamp(t *testing.T) {
	want := map[string]string{
		"draft": "overlay2", "todo": "green", "in-progress": "peach",
		"completed": "overlay1", "scrapped": "surface2",
	}
	for _, sc := range DefaultStatuses {
		if w, ok := want[sc.Name]; ok && sc.Color != w {
			t.Errorf("status %q colour = %q, want %q", sc.Name, sc.Color, w)
		}
	}
}

// Task 2b shipped, and had to fix, exactly this bug for StatusOverride.Archive:
// an unconditional assignment in the apply closure means a colour-only
// override silently clears a field the user never mentioned. TypeOverride.Emphasis
// needs the same pointer-guarded treatment.

func TestRecolouringATypePreservesItsEmphasisFlag(t *testing.T) {
	c := Default()
	c.Types = []TypeOverride{{Name: "milestone", Color: "#cba6f7"}}

	got := c.GetType("milestone")
	if got == nil {
		t.Fatal(`GetType("milestone") = nil, want the merged entry`)
	}
	if got.Color != "#cba6f7" {
		t.Errorf(`GetType("milestone").Color = %q, want the override`, got.Color)
	}
	if !got.Emphasis {
		t.Error(`GetType("milestone").Emphasis = false, want true: a colour-only override must not touch Emphasis`)
	}
}

func TestAnExplicitEmphasisOverrideIsHonoured(t *testing.T) {
	c := Default()
	c.Types = []TypeOverride{{Name: "milestone", Emphasis: boolPtr(false)}}

	if c.GetType("milestone").Emphasis {
		t.Error(`GetType("milestone").Emphasis = true, want false: an explicit emphasis: false must be honoured`)
	}
}

func TestSaveRoundTripsTypeEmphasisOverride(t *testing.T) {
	// Save() is called by more than `init` (e.g. `lif rename`). Without
	// serializing Emphasis, a Save() on an already-loaded config would
	// silently erase whatever the user configured.
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.Types = []TypeOverride{{Name: "feature", Emphasis: boolPtr(true)}}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := reloaded.GetType("feature"); got == nil || !got.Emphasis {
		t.Errorf("reloaded GetType(\"feature\").Emphasis = %v, want the saved override", got)
	}
}

// Plan task 1 of docs/topics/beans-type-profiles: the hierarchy moves off the
// type names and onto a numeric rank per type.

func TestRankOfReturnsTheBuiltInRanks(t *testing.T) {
	c := &Config{}
	for name, want := range map[string]int{
		"milestone": 1,
		"epic":      2,
		"feature":   3,
		"task":      4,
		"bug":       4,
	} {
		if got := c.RankOf(name); got != want {
			t.Errorf("RankOf(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestRankOfFallsBackToLeafRankForUnknownTypes(t *testing.T) {
	c := &Config{}
	if got := c.RankOf("chore"); got != LeafRank {
		t.Errorf("RankOf(\"chore\") = %d, want %d (LeafRank)", got, LeafRank)
	}
}

func TestRankOfHonoursAConfiguredRank(t *testing.T) {
	rank := 2
	c := &Config{Types: []TypeOverride{{Name: "package", Rank: &rank}}}
	if got := c.RankOf("package"); got != 2 {
		t.Errorf("RankOf(\"package\") = %d, want 2", got)
	}
}

// An explicit "rank: 0" is a real, distinct value - not the same as
// omitting rank entirely - and must survive the merge as 0, not be silently
// promoted to LeafRank the way an actually-unset rank is.
func TestExplicitRankZeroIsNotCoercedToLeafRank(t *testing.T) {
	rank := 0
	c := &Config{Types: []TypeOverride{{Name: "package", Rank: &rank}}}
	if got := c.RankOf("package"); got != 0 {
		t.Errorf("RankOf(\"package\") = %d, want 0 (explicit rank must be honoured)", got)
	}
	for _, ty := range c.TypeList() {
		if ty.Name == "package" && ty.Rank != 0 {
			t.Errorf("TypeList() carries rank %d for \"package\", want 0", ty.Rank)
		}
	}
}

func TestAppendedTypeWithoutRankLandsOnTheLeafRank(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "chore", Color: "peach"}}}
	if got := c.RankOf("chore"); got != LeafRank {
		t.Errorf("RankOf(\"chore\") = %d, want %d (LeafRank)", got, LeafRank)
	}
}

// AC-4 is about the merged list itself, not just about what RankOf reports:
// both getters normalise a zero rank to LeafRank on their own, so a test that
// only asks RankOf stays green even when the merge leaves the entry at 0. A
// rank-0 entry in TypeList outranks every container for every later reader.
func TestAppendedTypeCarriesTheLeafRankInTheMergedList(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "chore", Color: "peach"}}}

	for _, ty := range c.TypeList() {
		if ty.Name != "chore" {
			continue
		}
		if ty.Rank != LeafRank {
			t.Errorf("TypeList() entry \"chore\".Rank = %d, want %d (LeafRank)", ty.Rank, LeafRank)
		}
		return
	}
	t.Fatal("TypeList() carries no \"chore\" entry, want the appended type")
}

func TestColourOnlyOverrideKeepsTheBuiltInRank(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "epic", Color: "red"}}}
	if got := c.RankOf("epic"); got != 2 {
		t.Errorf("RankOf(\"epic\") = %d, want 2 - a colour override must not reset the rank", got)
	}
}

// An explicit `color: ""`, `description: ""` or `short: ""` in the YAML source
// is a real value - clear the built-in default - not the same as never
// naming the key at all, which leaves the default alone. This only applies
// to configs loaded from YAML: UnmarshalYAML is what records which keys
// were actually named (see StatusOverride.colorSet et al.); a struct built
// directly in Go can never make that distinction and keeps the old
// non-empty-always-applies behaviour, which every other override literal
// in this codebase already relies on.
func TestExplicitEmptyOverrideClearsTheBuiltInValue(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ConfigFileName)
	body := "beans:\n  prefix: test-\n" +
		"types:\n  - name: bug\n    color: \"\"\n    description: \"\"\n    short: \"\"\n" +
		"statuses:\n  - name: completed\n    color: \"\"\n    description: \"\"\n" +
		"priorities:\n  - name: critical\n    color: \"\"\n    description: \"\"\n"
	if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	bug := cfg.GetType("bug")
	if bug == nil {
		t.Fatal("GetType(\"bug\") = nil")
	}
	if bug.Color != "" {
		t.Errorf("bug.Color = %q, want \"\" (explicit clear)", bug.Color)
	}
	if bug.Description != "" {
		t.Errorf("bug.Description = %q, want \"\" (explicit clear)", bug.Description)
	}
	if bug.Short != "" {
		t.Errorf("bug.Short = %q, want \"\" (explicit clear)", bug.Short)
	}

	completed := cfg.GetStatus("completed")
	if completed == nil {
		t.Fatal("GetStatus(\"completed\") = nil")
	}
	if completed.Color != "" {
		t.Errorf("completed.Color = %q, want \"\" (explicit clear)", completed.Color)
	}
	if completed.Description != "" {
		t.Errorf("completed.Description = %q, want \"\" (explicit clear)", completed.Description)
	}

	critical := cfg.GetPriority("critical")
	if critical == nil {
		t.Fatal("GetPriority(\"critical\") = nil")
	}
	if critical.Color != "" {
		t.Errorf("critical.Color = %q, want \"\" (explicit clear)", critical.Color)
	}
	if critical.Description != "" {
		t.Errorf("critical.Description = %q, want \"\" (explicit clear)", critical.Description)
	}
}

// A Save() -> Load() round trip must not lose the explicit clear either:
// toYAMLNode() has to write `color: ""` back out, not skip the key the way
// it would for a genuinely-unset override.
func TestSaveRoundTripsAnExplicitlyEmptyOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)
	body := "beans:\n  prefix: test-\ntypes:\n  - name: bug\n    color: \"\"\n"
	if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if bug := reloaded.GetType("bug"); bug == nil || bug.Color != "" {
		t.Errorf("GetType(\"bug\").Color after round trip = %+v, want empty", bug)
	}
}

func TestTypesAtRankReturnsListOrder(t *testing.T) {
	rank := 2
	c := &Config{Types: []TypeOverride{{Name: "package", Rank: &rank}}}
	got := c.TypesAtRank(2)
	want := []string{"epic", "package"}
	if len(got) != len(want) {
		t.Fatalf("TypesAtRank(2) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("TypesAtRank(2)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// Save() builds the types: sequence node by node rather than by marshalling
// the struct, so a yaml tag alone does not carry Rank through a write-and-read
// cycle. Without this test the omission is invisible: the existing round-trip
// test sets no rank.
func TestSaveRoundTripsATypeRankOverride(t *testing.T) {
	tmpDir := t.TempDir()
	rank := 2

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.Types = []TypeOverride{{Name: "package", Rank: &rank}}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := reloaded.RankOf("package"); got != 2 {
		t.Errorf("reloaded RankOf(\"package\") = %d, want 2 - the saved rank must survive", got)
	}
}

// Plan task 3 of docs/topics/beans-type-profiles: a type can opt out of the
// aggregate views (roadmap, milestones) without losing its rank or colour.

func TestEveryBuiltInTypeIsVisibleByDefault(t *testing.T) {
	c := &Config{}
	for _, name := range c.TypeNames() {
		if !c.IsRoadmapType(name) {
			t.Errorf("IsRoadmapType(%q) = false, want true — built-in types stay visible", name)
		}
	}
}

func TestRoadmapFalseHidesAType(t *testing.T) {
	visible := false
	rank := 1
	c := &Config{Types: []TypeOverride{{Name: "bucket", Rank: &rank, Roadmap: &visible}}}
	if c.IsRoadmapType("bucket") {
		t.Error("a type with roadmap: false must not count as a roadmap type")
	}
}

// TypeList() merges each override into a fresh TypeConfig, but Roadmap used
// to hand out o.Roadmap itself rather than a copy - every other pointer
// field here (Rank, Emphasis) is deep-copied. Mutating a Roadmap value
// through the merged list must not reach back into the config's own
// TypeOverride.
func TestRoadmapPointerIsDeepCopiedNotAliased(t *testing.T) {
	visible := true
	override := TypeOverride{Name: "bucket", Roadmap: &visible}
	c := &Config{Types: []TypeOverride{override}}

	merged := c.GetType("bucket")
	if merged == nil || merged.Roadmap == nil {
		t.Fatal("GetType(\"bucket\").Roadmap = nil, want a copy of the override's pointer")
	}
	*merged.Roadmap = false

	if !*c.Types[0].Roadmap {
		t.Error("mutating the merged TypeConfig's Roadmap flipped c.Types[0].Roadmap - it must be an independent copy")
	}
}

func TestAppendedTypeIsVisibleWithoutTheKey(t *testing.T) {
	rank := 1
	c := &Config{Types: []TypeOverride{{Name: "release", Rank: &rank}}}
	if !c.IsRoadmapType("release") {
		t.Error("an appended type without roadmap: must default to visible")
	}
}

func TestColourOnlyOverrideKeepsVisibility(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "milestone", Color: "red"}}}
	if !c.IsRoadmapType("milestone") {
		t.Error("a colour override must not hide a type")
	}
}

// Save() builds the types: sequence node by node rather than by marshalling
// the struct, so a yaml tag alone does not carry Roadmap through a
// write-and-read cycle. Without this test the omission is invisible.
func TestSaveRoundTripsATypeRoadmapOverride(t *testing.T) {
	tmpDir := t.TempDir()
	visible := false

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.Types = []TypeOverride{{Name: "bug", Roadmap: &visible}}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if reloaded.IsRoadmapType("bug") {
		t.Error("reloaded IsRoadmapType(\"bug\") = true, want false - the saved roadmap flag must survive")
	}
}

// Task 7 of docs/topics/beans-type-profiles: the type column's single-letter
// code moves from a hardcoded internal/ui switch onto per-type config.

func TestShortOfPrefersTheConfiguredValue(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "chore", Short: "C"}}}
	if got := c.ShortOf("chore"); got != "C" {
		t.Errorf("ShortOf(\"chore\") = %q, want \"C\"", got)
	}
}

// Renamed from TestShortOfFallsBackToTheFirstLetter: DefaultTypes now carries
// an explicit Short: "M" for "milestone", so this pins the configured branch,
// not the first-letter fallback (see
// TestShortOfFallsBackForATypeWithNoConfiguredShort below for that one).
func TestShortOfReturnsTheConfiguredShortForABuiltInType(t *testing.T) {
	c := &Config{}
	if got := c.ShortOf("milestone"); got != "M" {
		t.Errorf("ShortOf(\"milestone\") = %q, want \"M\"", got)
	}
}

// A type with no configured short anywhere is what actually exercises the
// first-letter fallback (see the comment on
// TestShortOfReturnsTheConfiguredShortForABuiltInType above).
func TestShortOfFallsBackForATypeWithNoConfiguredShort(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "chore"}}}
	if got := c.ShortOf("chore"); got != "C" {
		t.Errorf("ShortOf(\"chore\") = %q, want \"C\"", got)
	}
}

func TestShortOfReturnsAQuestionMarkForAnUnknownType(t *testing.T) {
	c := &Config{}
	if got := c.ShortOf("unheard-of"); got != "?" {
		t.Errorf("ShortOf(\"unheard-of\") = %q, want \"?\"", got)
	}
}

// Save() builds the types: sequence node by node rather than by marshalling
// the struct, so a yaml tag alone does not carry Short through a
// write-and-read cycle. Without this test the omission is invisible.
func TestSaveRoundTripsATypeShortOverride(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	// "X" deliberately does not match "package"'s own first letter, so a
	// Save() that silently drops Short would still pass by falling back to
	// the first-letter default ("P") instead of failing loudly.
	cfg.Types = []TypeOverride{{Name: "package", Short: "X"}}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := reloaded.ShortOf("package"); got != "X" {
		t.Errorf("reloaded ShortOf(\"package\") = %q, want \"X\" - the saved short must survive", got)
	}
}

// Task 9 fix round 2: TypesExclusive is a new top-level key, not one of the
// TypeOverride fields, so it needs its own node in toYAMLNode()'s hand-built
// mapping next to "types" - the same trap that TestSaveRoundTripsATypeShortOverride
// and its siblings pin for the per-entry override fields.
func TestSaveRoundTripsTypesExclusive(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)
	cfg.TypesExclusive = true
	cfg.Types = []TypeOverride{{Name: "task", Rank: intPtr(LeafRank)}}

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reloaded.TypesExclusive {
		t.Error("reloaded TypesExclusive = false, want true - the saved flag must survive a write-and-read cycle")
	}
}

// A plain Save() (no TypesExclusive set) must not introduce the new key at
// all: every config written before this change, and every lif init without
// --profile, has TypesExclusive false and must round-trip to the same false.
func TestSaveOmitsTypesExclusiveWhenFalse(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultWithPrefix("test-")
	cfg.SetConfigDir(tmpDir)

	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "types_exclusive") {
		t.Errorf("Save() wrote a types_exclusive key for a config that never set it:\n%s", raw)
	}

	reloaded, err := Load(filepath.Join(tmpDir, ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if reloaded.TypesExclusive {
		t.Error("reloaded TypesExclusive = true, want false")
	}
}

// With TypesExclusive, TypeList() must be exactly the config's own Types -
// nothing merged in from DefaultTypes. A profile like "todo" carries only
// "task", so every other built-in name must fall back to the leaf rank as an
// unknown name would, and no type occupies rank 1.
func TestTypesExclusiveDropsTheBuiltInDefaults(t *testing.T) {
	types, ok := ProfileTypes("todo")
	if !ok {
		t.Fatal("todo profile missing")
	}
	cfg := &Config{TypesExclusive: true, Types: types}

	if got := cfg.RankOf("milestone"); got != LeafRank {
		t.Errorf("RankOf(\"milestone\") = %d, want %d (unknown name falls back to the leaf rank)", got, LeafRank)
	}
	if got := cfg.TypesAtRank(1); len(got) != 0 {
		t.Errorf("TypesAtRank(1) = %v, want empty - an exclusive todo config carries no rank-1 type", got)
	}
	if got := cfg.TypeList(); len(got) != 1 || got[0].Name != "task" {
		t.Errorf("TypeList() = %+v, want exactly one type named \"task\"", got)
	}
}

// The "complex" side of the same behaviour: an exclusive config with eight
// types must not carry the two rank-1/rank-2 names ("milestone", "epic")
// that DefaultTypes contributes under merge semantics.
func TestTypesExclusiveOmitsDefaultsNotNamedByTheProfile(t *testing.T) {
	types, ok := ProfileTypes("complex")
	if !ok {
		t.Fatal("complex profile missing")
	}
	cfg := &Config{TypesExclusive: true, Types: types}

	list := cfg.TypeList()
	if len(list) != 8 {
		t.Fatalf("TypeList() has %d types, want 8", len(list))
	}
	for _, name := range []string{"milestone", "epic"} {
		for _, ty := range list {
			if ty.Name == name {
				t.Errorf("TypeList() carries %q, which the complex profile does not name", name)
			}
		}
	}
}

// Task 9 fix round 3: TypeList() can now legitimately return an empty slice
// (types_exclusive: true with no types), which used to make Load's
// default_type fallback panic on an out-of-range index into that slice. This
// loads exactly that config and requires Load to return normally, leaving
// DefaultType empty rather than deriving one from nothing.
func TestLoadDoesNotPanicOnAnEmptyExclusiveTypeList(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ConfigFileName)
	content := "beans:\n  prefix: x-\ntypes_exclusive: true\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Items.DefaultType != "" {
		t.Errorf("Beans.DefaultType = %q, want empty - nothing to derive one from", cfg.Items.DefaultType)
	}
}

// TestShortOfSlicesByRuneNotByte pins the fallback short code against a
// multi-byte first character. It used to be t.Name[:1] -- a byte slice --
// so a type named "Änderung" produced half a rune and rendered as U+FFFD.
func TestShortOfSlicesByRuneNotByte(t *testing.T) {
	c := &Config{Types: []TypeOverride{{Name: "Änderung"}, {Name: "课题"}}}
	for name, want := range map[string]string{"Änderung": "Ä", "课题": "课"} {
		if got := c.ShortOf(name); got != want {
			t.Errorf("ShortOf(%q) = %q, want %q", name, got, want)
		}
	}
}
