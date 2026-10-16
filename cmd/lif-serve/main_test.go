package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestLifBinaryPrefersTheOneBesideTheExecutable(t *testing.T) {
	beside, onPath := t.TempDir(), t.TempDir()
	writeExecutable(t, filepath.Join(beside, "lif"))
	writeExecutable(t, filepath.Join(onPath, "lif"))
	t.Setenv("PATH", onPath)

	got := lifBinary(filepath.Join(beside, "lif-serve"))

	if want := filepath.Join(beside, "lif"); got != want {
		t.Errorf("lifBinary = %q, want %q", got, want)
	}
}

func TestLifBinaryFallsBackToPath(t *testing.T) {
	elsewhere, onPath := t.TempDir(), t.TempDir()
	writeExecutable(t, filepath.Join(onPath, "lif"))
	t.Setenv("PATH", onPath)

	got := lifBinary(filepath.Join(elsewhere, "lif-serve"))

	if want := filepath.Join(onPath, "lif"); got != want {
		t.Errorf("lifBinary = %q, want %q", got, want)
	}
}

func TestLifBinaryIsEmptyWhenThereIsNone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if got := lifBinary(filepath.Join(t.TempDir(), "lif-serve")); got != "" {
		t.Errorf("lifBinary = %q, want empty: a hook then fails with %q", got, "lif binary not found")
	}
}
