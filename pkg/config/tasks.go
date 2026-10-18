package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// GlobalTasksFileName is the name of the global task file inside HomeDir.
const GlobalTasksFileName = "tasks.yml"

var (
	taskNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	// Lowercase only, so an argument can never overwrite PATH, HOME or LIF_*.
	taskArgRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
)

// Task is one entry under tasks: — a named list of shell steps.
type Task struct {
	Desc  string   `yaml:"desc,omitempty"`
	Args  []string `yaml:"args,omitempty"` // required positional args, exported to steps as shell variables
	Deps  []string `yaml:"deps,omitempty"` // task names run first, without args
	Steps []string `yaml:"steps"`          // each step is one `sh -c` script; multi-line allowed
}

// UnmarshalYAML decodes a Task and rejects non-mappings and unknown keys, so a
// typo such as "step:" fails loudly instead of yielding a task without steps.
func (t *Task) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a task must be a mapping with steps", value.Line)
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		switch keyNode.Value {
		case "desc", "args", "deps", "steps":
		default:
			return fmt.Errorf("line %d: unknown task key %q (known: desc, args, deps, steps)", keyNode.Line, keyNode.Value)
		}
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value != "steps" || value.Content[i+1].Kind != yaml.SequenceNode {
			continue
		}
		for j, step := range value.Content[i+1].Content {
			if step.Kind != yaml.ScalarNode || step.Tag != "!!str" {
				return fmt.Errorf("line %d: steps[%d]: step must be a string (quote it)", step.Line, j)
			}
		}
	}
	type plain Task
	var raw plain
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*t = Task(raw)
	return nil
}

// validateTasks returns the first problem in tasks, visiting names sorted.
// Deps are not resolved here: a global task may name a project task, so they
// resolve at run time against the merged set.
func validateTasks(tasks map[string]Task) error {
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t := tasks[name]
		if !taskNameRe.MatchString(name) {
			return fmt.Errorf(`tasks.%s: invalid task name (use lowercase letters, digits, "-" and "_")`, name)
		}
		if len(t.Steps) == 0 {
			return fmt.Errorf("tasks.%s: no steps", name)
		}
		for i, step := range t.Steps {
			if strings.TrimSpace(step) == "" {
				return fmt.Errorf("tasks.%s.steps[%d]: empty step", name, i)
			}
		}
		seen := make(map[string]bool, len(t.Args))
		for _, arg := range t.Args {
			if !taskArgRe.MatchString(arg) {
				return fmt.Errorf(`tasks.%s.args: invalid argument name %q (use lowercase letters, digits and "_")`, name, arg)
			}
			if seen[arg] {
				return fmt.Errorf("tasks.%s.args: duplicate argument name %q", name, arg)
			}
			seen[arg] = true
		}
		for _, dep := range t.Deps {
			if dep == "" {
				return fmt.Errorf("tasks.%s.deps: empty task name", name)
			}
		}
	}
	return nil
}

// topValueNode returns the value node of the top-level key, or nil.
func topValueNode(doc *yaml.Node, key string) *yaml.Node {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	top := doc.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == key {
			return top.Content[i+1]
		}
	}
	return nil
}

// GlobalTasksPath returns ~/.lif/tasks.yml.
func GlobalTasksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, HomeDir, GlobalTasksFileName), nil
}

// Global is the content of the global file ~/.lif/tasks.yml.
type Global struct {
	Tasks map[string]Task `yaml:"tasks"`
	Hooks map[string]Hook `yaml:"hooks"`
}

// LoadGlobal reads the global file. No HOME or no file means the zero Global.
func LoadGlobal() (Global, error) {
	path, err := GlobalTasksPath()
	if err != nil {
		return Global{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Global{}, nil
		}
		return Global{}, fmt.Errorf("%s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Global{}, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Kind == 0 {
		return Global{}, nil
	}
	if doc.Content[0].Kind == yaml.MappingNode {
		top := doc.Content[0]
		for i := 0; i+1 < len(top.Content); i += 2 {
			if key := top.Content[i].Value; key != "tasks" && key != "hooks" {
				return Global{}, fmt.Errorf("%s: unknown top-level key %q (known: tasks, hooks)", path, key)
			}
		}
	}
	var g Global
	if err := doc.Decode(&g); err != nil {
		return Global{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := validateTasks(g.Tasks); err != nil {
		return Global{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := validateHooks(g.Hooks, nil); err != nil {
		return Global{}, fmt.Errorf("%s: %w", path, err)
	}
	return g, nil
}
