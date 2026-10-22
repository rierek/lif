package tui

// app_test.go — Q01 (bean bt-7jr8, T8-review): Init() must not panic against
// a nil data.Store -- it must surface a beansLoadedMsg carrying an error
// instead of nil-dereferencing inside loadCmd -> Client.List -> Client.run
// (cmd.Dir = c.RepoDir on a nil *Client).

import "testing"

// TestInitNilClientReturnsErrorMsgInsteadOfPanicking guards Q01: newModel(nil,
// ...) is otherwise only prevented by convention at Run()'s call site --
// Init() itself must turn a would-be nil-deref panic into a normal,
// status-line-surfaced load error.
func TestInitNilClientReturnsErrorMsgInsteadOfPanicking(t *testing.T) {
	m := newModel(nil, "/tmp/does-not-matter")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() with a nil client must still return a cmd (never nil -- caller expects a msg)")
	}
	msg := cmd() // must not panic
	loaded, ok := msg.(beansLoadedMsg)
	if !ok || loaded.err == nil {
		t.Fatalf("Init() with nil client should yield a beansLoadedMsg carrying an error, got %#v", msg)
	}
}
