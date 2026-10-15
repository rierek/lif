package commands

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/portalloc"
)

// envValue returns the value of name in a KEY=VALUE list and whether it is set.
func envValue(env []string, name string) (string, bool) {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, name+"="); ok {
			return v, true
		}
	}
	return "", false
}

// A workspace shell session and its run session ("<id>__run") both get
// LIF_WORKSPACE_PORT and BEANS_WORKSPACE_PORT, with the workspace's port.
func TestWorkspaceEnvFunc_exportsBothPortVariables(t *testing.T) {
	alloc := portalloc.NewDefault()
	alloc.Allocate("ws-other") // so that ws-1 does not hold the first port
	want := strconv.Itoa(alloc.Allocate("ws-1"))
	envFor := workspaceEnvFunc(alloc)

	for _, sessionID := range []string{"ws-1", "ws-1__run"} {
		t.Run(sessionID, func(t *testing.T) {
			env := envFor(sessionID)
			for _, name := range []string{"LIF_WORKSPACE_PORT", "BEANS_WORKSPACE_PORT"} {
				got, ok := envValue(env, name)
				if !ok || got != want {
					t.Errorf("%s = %q (set: %v), want %q; env = %v", name, got, ok, want, env)
				}
			}
		})
	}
}

// A session without an allocated workspace gets no port variable.
func TestWorkspaceEnvFunc_unknownWorkspaceHasNoPort(t *testing.T) {
	envFor := workspaceEnvFunc(portalloc.NewDefault())
	if env := envFor("unknown__run"); len(env) != 0 {
		t.Errorf("env = %v, want none", env)
	}
}
