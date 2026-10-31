package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func loadTypesYAML(t *testing.T, body string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(p)
}

func TestTypeParentsLoadKeepsAbsentApartFromEmpty(t *testing.T) {
	cfg, err := loadTypesYAML(t, "beans:\n  prefix: x-\ntypes:\n  - name: task\n    parents: [epic]\n  - name: epic\n    parents: []\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetType("task").Parents; !slices.Equal(got, []string{"epic"}) {
		t.Errorf("task parents = %v", got)
	}
	if got := cfg.GetType("epic").Parents; got == nil || len(got) != 0 {
		t.Errorf("epic parents = %#v, want non-nil empty", got)
	}
	if cfg.GetType("feature").Parents != nil {
		t.Error("feature names no parents; want nil")
	}
}

func TestTypeParentsLoadRejects(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"unknown":           {"types:\n  - name: task\n    parents: [nope]\n", `unknown type "nope"`},
		"equal rank":        {"types:\n  - name: epic\n    parents: [feature]\n", "does not rank below"},
		"self":              {"types:\n  - name: task\n    parents: [bug]\n", "does not rank below"},
		"exclusive unknown": {"types_exclusive: true\ntypes:\n  - name: a\n    rank: 2\n    parents: [epic]\n", `unknown type "epic"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadTypesYAML(t, "beans:\n  prefix: x-\n"+tc.body)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestTypeParentsSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	list, none := []string{"epic"}, []string{}
	cfg := &Config{
		Items: ItemsConfig{Path: ".beans", Prefix: "x-", IDLength: 4},
		Types: []TypeOverride{{Name: "task", Parents: &list}, {Name: "epic", Parents: &none}},
	}
	cfg.SetConfigDir(dir)
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := Load(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if p := got.GetType("task").Parents; !slices.Equal(p, list) {
		t.Errorf("task parents = %v", p)
	}
	if p := got.GetType("epic").Parents; p == nil || len(p) != 0 {
		t.Errorf("epic parents = %#v, want non-nil empty", p)
	}
}

func TestTypeListParentsDoNotAliasOverride(t *testing.T) {
	list := []string{"epic"}
	cfg := &Config{Types: []TypeOverride{{Name: "task", Parents: &list}}}
	cfg.GetType("task").Parents[0] = "x"
	if list[0] != "epic" {
		t.Error("merged Parents aliases the override")
	}
}
