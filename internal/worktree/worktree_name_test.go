package worktree

import "testing"

func TestCreateRejectsUnsafeName(t *testing.T) {
	m := &Manager{}
	for _, n := range []string{"../x", "/abs", "-x", "a/b", ".hidden", "..", "a..b/../c"} {
		if _, err := m.Create(n); err == nil {
			t.Errorf("Create(%q) accepted", n)
		}
	}
}
