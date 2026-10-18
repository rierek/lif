//go:build !windows

package tasks

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
)

// runAsync runs task x with ctx and returns a channel for the result.
func runAsync(t *testing.T, ctx context.Context, root, step string) (<-chan error, *bytes.Buffer) {
	t.Helper()
	set := Set{Root: root, Project: map[string]config.Task{"x": steps(step)}}
	plan, err := set.Plan("x", false)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	res := make(chan error, 1)
	go func() {
		res <- RunContext(ctx, root, plan, nil, IO{Stdout: &out, Stderr: &out}, nil)
	}()
	return res, &out
}

func TestTP22_SignalIsForwardedAndCodeKept(t *testing.T) {
	root := t.TempDir()
	ready := filepath.Join(root, "ready")
	res, out := runAsync(t, context.Background(), root,
		`trap 'echo cleanup; exit 7' TERM; touch `+ready+`; sleep 20 & wait`)
	waitFor(t, ready)
	syscall.Kill(os.Getpid(), syscall.SIGTERM) // lif survives: Run handles it
	err := <-res
	var se *StepError
	if !errors.As(err, &se) || se.Code != 7 || !errors.Is(err, ErrInterrupted) {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(out.String(), "cleanup") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestTP23_SignalReachesChildren(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "pid")
	res, _ := runAsync(t, context.Background(), root, `sleep 20 & echo $! > `+pidFile+`; wait`)
	waitFor(t, pidFile)
	raw, _ := os.ReadFile(pidFile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
	<-res
	time.Sleep(100 * time.Millisecond)
	if syscall.Kill(pid, 0) == nil {
		syscall.Kill(pid, syscall.SIGKILL)
		t.Fatal("child survived the signal")
	}
}

func TestTP24_ContextKillsGroup(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "pid")
	cause := errors.New("timed out")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 300*time.Millisecond, cause)
	defer cancel()
	start := time.Now()
	res, _ := runAsync(t, ctx, root, `sleep 20 & echo $! > `+pidFile+`; wait`)
	err := <-res
	if !errors.Is(err, cause) || time.Since(start) > 5*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(start))
	}
	raw, _ := os.ReadFile(pidFile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	time.Sleep(100 * time.Millisecond)
	if syscall.Kill(pid, 0) == nil {
		syscall.Kill(pid, syscall.SIGKILL)
		t.Fatal("child survived the timeout")
	}
}

func waitFor(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if b, err := os.ReadFile(path); err == nil && (len(b) > 0 || strings.HasSuffix(path, "ready")) {
			time.Sleep(50 * time.Millisecond) // the trap is set before touch
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
}
