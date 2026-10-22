package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const tuiBody = `tui:
  actions:
    - task: accept
      key: A
      label: accept
      statuses: [to-review]
    - task: build
`

func TestTUIActionsLoadAndSurviveSave(t *testing.T) {
	cfg, path, err := tkLoadProject(t, tuiBody)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []TUIAction{
		{Task: "accept", Key: "A", Label: "accept", Statuses: []string{"to-review"}},
		{Task: "build"},
	}
	if !reflect.DeepEqual(cfg.TUI.Actions, want) {
		t.Fatalf("Actions = %+v, want %+v", cfg.TUI.Actions, want)
	}
	if err := cfg.Save(filepath.Dir(path)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if !reflect.DeepEqual(reloaded.TUI.Actions, want) {
		t.Errorf("reloaded Actions = %+v, want %+v", reloaded.TUI.Actions, want)
	}
}
