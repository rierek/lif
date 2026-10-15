package commands

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer lets the test read what the subprocess has printed so far.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// AC5: the serve log lines carry the prefix [lif] instead of [beans], and the
// GraphQL playground is titled for the product. serve blocks until it is
// killed, so the test runs the real lif-serve binary as a subprocess.
func TestServeLogLinesCarryTheLifPrefix(t *testing.T) {
	serveBin := serveTestBinary(t)

	project := t.TempDir()
	store := filepath.Join(project, ".lif")
	// A leftover in-store worktrees directory makes serve print its warning.
	if err := os.MkdirAll(filepath.Join(store, ".worktrees", "old"), 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", project, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v\n%s", err, out)
	}
	// A GitHub remote plus a stand-in gh CLI on PATH makes serve report the
	// forge it detected.
	if out, err := exec.Command("git", "-C", project, "remote", "add", "origin", "https://github.com/o/r.git").CombinedOutput(); err != nil {
		t.Skipf("git remote add: %v\n%s", err, out)
	}
	stubDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubDir, "gh"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	var out syncBuffer
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(serveBin, "--lif-path", store, "--port", fmt.Sprint(port))
	cmd.Dir = project
	cmd.Env = append(os.Environ(),
		"HOME="+t.TempDir(),
		"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pw.Close()
	go func() { _, _ = io.Copy(&out, pr) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(out.String(), "[lif] Terminal: ") {
		if time.Now().After(deadline) {
			t.Fatalf("serve did not report its startup lines in time:\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	got := out.String()
	for _, want := range []string{
		"[lif] worktrees directory: ",
		"[lif] WARNING: found old worktrees in ",
		"[lif] detected forge: ",
		fmt.Sprintf("[lif] Listening on 127.0.0.1:%d (loopback only", port),
		fmt.Sprintf("[lif] Starting server at http://127.0.0.1:%d/", port),
		fmt.Sprintf("[lif] GraphQL Playground: http://127.0.0.1:%d/playground", port),
		"[lif] Allowed origins: ",
		"[lif] Terminal: enabled",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("serve output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[beans]") {
		t.Errorf("serve output still carries the prefix [beans]:\n%s", got)
	}

	// The startup line prints just before the listener accepts, so poll.
	var resp *http.Response
	for {
		resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/playground", port))
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET /playground: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer resp.Body.Close()
	page, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(page), "<title>Lif GraphQL</title>") {
		t.Errorf("playground page lacks the title %q:\n%.400s", "Lif GraphQL", page)
	}
}
