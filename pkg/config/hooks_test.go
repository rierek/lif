package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const hkTasks = "tasks:\n    wt-open:\n        steps: [\"true\"]\n"

// hkValid is the schema example. Two deviations from the literal schema text:
// event is [updated] because from is only valid there, and message sits on a
// second hook, a gate, because it is only valid with stage: before.
const hkValid = hkTasks + `hooks:
    open-worktree:
        stage: after
        event: [updated]
        changed: [status]
        from: [todo]
        to: [in-progress]
        type: [feature, task]
        tags: [needs-wt]
        tags_added: [accepted]
        tags_removed: [blocked]
        run: wt-open
        args: [id]
    gate:
        stage: before
        event: [updated]
        run: wt-open
        message: "Stop."
`

// hkProject loads a project config whose single hook x has the given body
// lines (each indented under x).
func hkProject(t *testing.T, lines string) (*Config, string, error) {
	t.Helper()
	var b strings.Builder
	b.WriteString(hkTasks + "hooks:\n    x:\n")
	for _, l := range strings.Split(strings.TrimSpace(lines), "\n") {
		b.WriteString("        " + l + "\n")
	}
	return tkLoadProject(t, b.String())
}

func hkWantErr(t *testing.T, lines, want string) {
	t.Helper()
	_, path, err := hkProject(t, lines)
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

func hkWantOK(t *testing.T, lines string) {
	t.Helper()
	if _, _, err := hkProject(t, lines); err != nil {
		t.Fatalf("Load() error = %v, want ok", err)
	}
}

func hkWantHooks() map[string]Hook {
	return map[string]Hook{
		"open-worktree": {
			Stage: "after", Event: []string{"updated"}, Changed: []string{"status"},
			From: []string{"todo"}, To: []string{"in-progress"}, Type: []string{"feature", "task"},
			Tags: []string{"needs-wt"}, TagsAdded: []string{"accepted"}, TagsRemoved: []string{"blocked"},
			Run: "wt-open", Args: []string{"id"},
		},
		"gate": {Stage: "before", Event: []string{"updated"}, Run: "wt-open", Message: "Stop."},
	}
}

func TestV01_SchemaExampleLoads(t *testing.T) {
	cfg, _, err := tkLoadProject(t, hkValid)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if want := hkWantHooks(); !reflect.DeepEqual(cfg.Hooks, want) {
		t.Errorf("Hooks = %+v, want %+v", cfg.Hooks, want)
	}
	if !cfg.Hooks["gate"].IsGate() || cfg.Hooks["open-worktree"].IsGate() {
		t.Errorf("IsGate: gate=%v open-worktree=%v, want true false", cfg.Hooks["gate"].IsGate(), cfg.Hooks["open-worktree"].IsGate())
	}
}

func TestV02_SaveRoundTripsHooks(t *testing.T) {
	cfg, path, err := tkLoadProject(t, hkValid)
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
	if !strings.Contains(string(raw), "open-worktree:") {
		t.Errorf("saved file lacks open-worktree:\n%s", raw)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	if !reflect.DeepEqual(reloaded.Hooks, cfg.Hooks) {
		t.Errorf("reloaded hooks = %+v, want %+v", reloaded.Hooks, cfg.Hooks)
	}
}

func TestV03_UnknownHookKey(t *testing.T) {
	hkWantErr(t, "on: [updated]\nrun: wt-open", `unknown hook key "on"`)
}

func TestV04_UnknownEvent(t *testing.T) {
	hkWantErr(t, "event: [status]\nrun: wt-open", `unknown event "status"`)
}

func TestV05_UnknownStage(t *testing.T) {
	hkWantErr(t, "stage: sideways\nevent: [updated]\nrun: wt-open", `unknown stage "sideways"`)
}

func TestV06_BeforeNeedsUpdatedOnly(t *testing.T) {
	hkWantErr(t, "stage: before\nevent: [created, updated]\nrun: wt-open", "before is only valid with event: [updated]")
}

func TestV07_MessageNeedsBefore(t *testing.T) {
	hkWantErr(t, "event: [updated]\nrun: wt-open\nmessage: hi", "message: only valid with stage: before")
}

func TestV08_FromNeedsUpdatedOnly(t *testing.T) {
	hkWantErr(t, "event: [created, updated]\nfrom: [todo]\nrun: wt-open", "from: only valid with event: [updated]")
}

func TestV09_TitleNotWatchable(t *testing.T) {
	hkWantErr(t, "event: [updated]\nchanged: [title]\nrun: wt-open", `"title" is not watchable`)
}

func TestV10_ExtraKeyWatchable(t *testing.T) {
	hkWantOK(t, "event: [updated]\nchanged: [target_entry]\nrun: wt-open")
}

func TestV11_UnknownItemField(t *testing.T) {
	hkWantErr(t, "event: [updated]\nrun: wt-open\nargs: [priority]", `unknown item field "priority"`)
}

func TestV12_ExtraAndPrevArgs(t *testing.T) {
	hkWantOK(t, "event: [updated]\nrun: wt-open\nargs: [target_entry, prev_target_entry, prev_status]")
}

func TestV13_UnknownStatusAndType(t *testing.T) {
	hkWantErr(t, "event: [updated]\nto: [nope]\nrun: wt-open", `unknown status "nope"`)
	hkWantErr(t, "event: [updated]\ntype: [nope]\nrun: wt-open", `unknown type "nope"`)
}

func TestV14_InvalidTag(t *testing.T) {
	hkWantErr(t, "event: [updated]\ntags_added: [Bad]\nrun: wt-open", `invalid tag "Bad"`)
}

func TestV15_NoRun(t *testing.T) {
	hkWantErr(t, "event: [updated]", "no run task")
}

func TestV16_GlobalSkipsStatusCheck(t *testing.T) {
	tkSetHome(t, "hooks:\n    x:\n        event: [updated]\n        to: [any-status]\n        run: t\n")
	g, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() error = %v", err)
	}
	if got := g.Hooks["x"].To; !reflect.DeepEqual(got, []string{"any-status"}) {
		t.Errorf("To = %v, want [any-status]", got)
	}
}

func TestV17_GlobalUnknownTopLevelKey(t *testing.T) {
	tkSetHome(t, "snippets: {}\n")
	_, err := LoadGlobal()
	if err == nil || !strings.Contains(err.Error(), `unknown top-level key "snippets" (known: tasks, hooks)`) {
		t.Errorf("LoadGlobal() error = %v, want unknown top-level key \"snippets\" (known: tasks, hooks)", err)
	}
}

func TestV18_HookNotMapping(t *testing.T) {
	_, _, err := tkLoadProject(t, "hooks:\n    x: \"echo hi\"\n")
	if err == nil || !strings.Contains(err.Error(), "a hook must be a mapping with event and run") {
		t.Errorf("Load() error = %v, want a hook must be a mapping with event and run", err)
	}
}

func TestV19_InvalidHookName(t *testing.T) {
	_, _, err := tkLoadProject(t, hkTasks+"hooks:\n    Bad Name:\n        event: [updated]\n        run: wt-open\n")
	if err == nil || !strings.Contains(err.Error(), "hooks.Bad Name: invalid hook name") {
		t.Errorf("Load() error = %v, want hooks.Bad Name: invalid hook name", err)
	}
}

func TestV20_EmptyEvent(t *testing.T) {
	hkWantErr(t, "event: []\nrun: wt-open", "hooks.x: no event")
}

func TestV21_InvalidRunName(t *testing.T) {
	hkWantErr(t, "event: [updated]\nrun: \"Bad Name\"", `hooks.x.run: invalid task name "Bad Name"`)
}

func TestV22_InvalidTagLists(t *testing.T) {
	hkWantErr(t, "event: [updated]\ntags: [Bad]\nrun: wt-open", `hooks.x.tags: invalid tag "Bad"`)
	hkWantErr(t, "event: [updated]\ntags_removed: [Bad]\nrun: wt-open", `hooks.x.tags_removed: invalid tag "Bad"`)
}

func TestV23_GlobalHookErrorStartsWithPath(t *testing.T) {
	tkSetHome(t, "hooks:\n    x:\n        event: []\n        run: t\n")
	path, err := GlobalTasksPath()
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadGlobal()
	if err == nil || !strings.HasPrefix(err.Error(), path) {
		t.Errorf("LoadGlobal() error = %v, want it to start with %q", err, path)
	}
}

func TestV24_KeyWithSpaceNotWatchable(t *testing.T) {
	hkWantErr(t, "event: [updated]\nchanged: [\"my key\"]\nrun: wt-open", `"my key" is not watchable`)
	hkWantErr(t, "event: [updated]\nrun: wt-open\nargs: [\"my key\"]", `unknown item field "my key"`)
}

func TestHookTimeoutDefaultsAndRejectsNegative(t *testing.T) {
	hkWantErr(t, "event: [updated]\nrun: wt-open\ntimeout: -1", "timeout: must be 0")
	if got := (Hook{}).TimeoutDuration(); got != DefaultHookTimeout {
		t.Errorf("default = %v", got)
	}
	zero, five := 0, 5
	if got := (Hook{Timeout: &zero}).TimeoutDuration(); got != 0 {
		t.Errorf("0 = %v", got)
	}
	if got := (Hook{Timeout: &five}).TimeoutDuration(); got != 5*time.Second {
		t.Errorf("5 = %v", got)
	}
}

// TRUST-13: id, slug, path and body are never extra keys, so a hook on them would never fire.
func TestV09_ManagedNamesNotWatchable(t *testing.T) {
	for _, key := range []string{"id", "slug", "path", "body"} {
		hkWantErr(t, "event: [updated]\nchanged: ["+key+"]\nrun: wt-open", `"`+key+`" is not watchable`)
	}
}
