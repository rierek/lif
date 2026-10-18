package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const tkValid = `tasks:
    po-accept:
        desc: x
        args: [id]
        deps: [test]
        steps: ["echo $id"]
    test:
        steps: ["true"]
`

// tkLoadProject writes body as the project config and loads it.
func tkLoadProject(t *testing.T, body string) (*Config, string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	return cfg, path, err
}

// tkWantProjectErr asserts Load fails with an error containing want and
// starting with the config path (TK13).
func tkWantProjectErr(t *testing.T, body, want string) {
	t.Helper()
	_, path, err := tkLoadProject(t, body)
	if err == nil {
		t.Fatalf("Load() error = nil, want %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Load() error = %q, want it to contain %q", err, want)
	}
	if !strings.HasPrefix(err.Error(), path) {
		t.Errorf("Load() error = %q, want it to start with %q", err, path)
	}
}

// tkSetHome points HOME at a fresh dir and optionally writes the global file.
func tkSetHome(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if body != "" {
		dir := filepath.Join(home, HomeDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, GlobalTasksFileName), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestTK01_ValidTaskParses(t *testing.T) {
	cfg, _, err := tkLoadProject(t, tkValid)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Task{Desc: "x", Args: []string{"id"}, Deps: []string{"test"}, Steps: []string{"echo $id"}}
	if got := cfg.Tasks["po-accept"]; !reflect.DeepEqual(got, want) {
		t.Errorf("po-accept = %+v, want %+v", got, want)
	}
	if got := cfg.Tasks["test"].Steps; !reflect.DeepEqual(got, []string{"true"}) {
		t.Errorf("test steps = %v, want [true]", got)
	}
}

func TestTK02_SaveRoundTripsTasks(t *testing.T) {
	cfg, path, err := tkLoadProject(t, tkValid)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := cfg.Save(filepath.Dir(path)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "po-accept:") {
		t.Errorf("saved file lacks po-accept:\n%s", raw)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if !reflect.DeepEqual(reloaded.Tasks, cfg.Tasks) {
		t.Errorf("reloaded tasks = %+v, want %+v", reloaded.Tasks, cfg.Tasks)
	}
}

func TestTK03_SaveKeepsTaskOrderAndComments(t *testing.T) {
	body := `tasks:
    # keep
    test:
        steps: ["true"]
    po-accept:
        steps: ["echo hi"]
`
	cfg, path, err := tkLoadProject(t, body)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := cfg.Save(filepath.Dir(path)); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "# keep") {
		t.Errorf("saved file lost the comment:\n%s", text)
	}
	if strings.Index(text, "test:") > strings.Index(text, "po-accept:") || !strings.Contains(text, "test:") {
		t.Errorf("saved file reordered tasks:\n%s", text)
	}
}

func TestTK04_UnknownTaskKey(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x:\n        step: [\"true\"]\n", `unknown task key "step"`)
}

func TestTK05_TaskNotAMapping(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x: \"echo hi\"\n", "a task must be a mapping with steps")
}

func TestTK06_InvalidTaskName(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    Bad Name:\n        steps: [\"true\"]\n", "invalid task name")
}

func TestTK07_NoSteps(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x:\n        steps: []\n", "tasks.x: no steps")
}

func TestTK08_EmptyStep(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x:\n        steps: [\"  \"]\n", "tasks.x.steps[0]: empty step")
}

func TestTK09_InvalidArgumentName(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x:\n        args: [ID]\n        steps: [\"true\"]\n", `invalid argument name "ID"`)
}

func TestTK10_DuplicateArgumentName(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x:\n        args: [a, a]\n        steps: [\"true\"]\n", `duplicate argument name "a"`)
}

func TestTK11_EmptyDepName(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    x:\n        deps: [\"\"]\n        steps: [\"true\"]\n", "tasks.x.deps: empty task name")
}

func TestTK12_MissingDepLoads(t *testing.T) {
	cfg, _, err := tkLoadProject(t, "tasks:\n    x:\n        deps: [missing]\n        steps: [\"true\"]\n")
	if err != nil {
		t.Fatalf("Load() error = %v, want nil: deps resolve at run time", err)
	}
	if got := cfg.Tasks["x"].Deps; !reflect.DeepEqual(got, []string{"missing"}) {
		t.Errorf("deps = %v, want [missing]", got)
	}
}

func TestTK13_ProjectErrorStartsWithPath(t *testing.T) {
	// A validation error and a decode error both carry the config path.
	tkWantProjectErr(t, "tasks:\n    x:\n        steps: []\n", "tasks.x: no steps")
	tkWantProjectErr(t, "tasks:\n    x:\n        step: [\"true\"]\n", "unknown task key")
}

func TestTK14_NoGlobalFile(t *testing.T) {
	tkSetHome(t, "")
	got, err := LoadGlobal()
	if err != nil || len(got.Tasks) != 0 || len(got.Hooks) != 0 {
		t.Errorf("no file: LoadGlobal() = %v, %v, want none, nil", got, err)
	}

	t.Setenv("HOME", "")
	got, err = LoadGlobal()
	if err != nil || len(got.Tasks) != 0 || len(got.Hooks) != 0 {
		t.Errorf("HOME unset: LoadGlobal() = %v, %v, want none, nil", got, err)
	}
}

func TestTK15_GlobalTaskLoads(t *testing.T) {
	tkSetHome(t, "tasks:\n    g:\n        steps: [\"true\"]\n")
	got, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() error = %v", err)
	}
	if len(got.Tasks) != 1 || !reflect.DeepEqual(got.Tasks["g"].Steps, []string{"true"}) {
		t.Errorf("LoadGlobal() = %+v, want one task g", got)
	}
}

func TestTK17_GlobalErrorStartsWithPath(t *testing.T) {
	tkSetHome(t, "tasks:\n    x:\n        steps: []\n")
	path, err := GlobalTasksPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		"tasks:\n    x:\n        steps: []\n",
		"tasks:\n    x:\n        step: [\"true\"]\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadGlobal()
		if err == nil || !strings.HasPrefix(err.Error(), path) {
			t.Errorf("LoadGlobal() error = %v, want it to start with %q", err, path)
		}
	}
}

func TestTK_NonStringStepRejected(t *testing.T) {
	tkWantProjectErr(t, "tasks:\n    a:\n        steps: [1, true]\n", "steps[0]: step must be a string")
	if _, _, err := tkLoadProject(t, "tasks:\n    a:\n        steps:\n            - '1'\n            - |\n              echo hi\n"); err != nil {
		t.Fatalf("string steps rejected: %v", err)
	}
}
