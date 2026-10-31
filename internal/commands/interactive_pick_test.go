package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

func TestPickLabel(t *testing.T) {
	b := &item.Item{ID: "x-1", Status: "todo", Title: "Login page"}
	if got, want := pickLabel(b), "x-1  todo  Login page"; got != want {
		t.Errorf("pickLabel() = %q, want %q", got, want)
	}
}

func TestPickItemSelects(t *testing.T) {
	items := []*item.Item{
		{ID: "a-1", Status: "todo", Title: "First"},
		{ID: "a-2", Status: "todo", Title: "Second"},
	}
	var chrome bytes.Buffer
	got, err := selectItem("Items", items, feedCreatePrograms(t, []string{keyDown, keyEnter}), &chrome)
	if err != nil {
		t.Fatalf("selectItem() error = %v", err)
	}
	if got.ID != "a-2" {
		t.Errorf("picked %s, want a-2", got.ID)
	}
	if !strings.Contains(chrome.String(), "a-1  todo  First") {
		t.Errorf("chrome lacks the label of the first item")
	}
}

func TestPickItemCancel(t *testing.T) {
	items := []*item.Item{{ID: "a-1", Status: "todo", Title: "First"}}
	_, err := selectItem("Items", items, feedCreatePrograms(t, []string{keyCtrlC}), &bytes.Buffer{})
	if err != errInteractiveAborted {
		t.Fatalf("err = %v, want errInteractiveAborted", err)
	}
}

func TestAskQueryPrefill(t *testing.T) {
	q := "log"
	if err := askQuery(&q, feedCreatePrograms(t, []string{"i", "n", keyEnter}), &bytes.Buffer{}); err != nil {
		t.Fatalf("askQuery() error = %v", err)
	}
	if q != "login" {
		t.Errorf("query = %q, want login", q)
	}
}

func TestPickRequiresTerminal(t *testing.T) {
	// go test runs with stdin and stdout off a terminal.
	for _, name := range []string{"list", "roadmap"} {
		err := requirePickTerminal(name)
		if err == nil || !strings.Contains(err.Error(), "lif "+name+" -i needs a terminal") {
			t.Errorf("requirePickTerminal(%q) = %v, want needs-a-terminal error", name, err)
		}
	}
}

func TestListInteractiveRefusals(t *testing.T) {
	ciFixture(t)
	t.Cleanup(func() { listInteractive, listJSON = false, false })
	listInteractive = true

	listJSON = true
	if err := listCmd.RunE(listCmd, nil); err == nil || !strings.Contains(err.Error(), "--json") {
		t.Errorf("-i --json error = %v, want --json conflict", err)
	}
	listJSON = false
	if err := listCmd.RunE(listCmd, nil); err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Errorf("-i without tty error = %v, want needs-a-terminal", err)
	}
}

func TestRoadmapInteractiveRefusals(t *testing.T) {
	ciFixture(t)
	t.Cleanup(func() { roadmapInteractive, roadmapJSON = false, false })
	roadmapInteractive = true

	roadmapJSON = true
	if err := roadmapCmd.RunE(roadmapCmd, nil); err == nil || !strings.Contains(err.Error(), "--json") {
		t.Errorf("-i --json error = %v, want --json conflict", err)
	}
	roadmapJSON = false
	if err := roadmapCmd.RunE(roadmapCmd, []string{"pp-m1"}); err == nil || !strings.Contains(err.Error(), "root ID") {
		t.Errorf("-i with ID error = %v, want ID conflict", err)
	}
	if err := roadmapCmd.RunE(roadmapCmd, nil); err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Errorf("-i without tty error = %v, want needs-a-terminal", err)
	}
}
