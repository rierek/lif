package itemcore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rierek/lif/internal/filelock"
)

// lockedItem creates one item and returns its Core, the path of its file and
// its etag.
func lockedItem(t *testing.T) (core *Core, path, etag string) {
	t.Helper()
	core, itemsDir := setupTestCore(t)
	b := createTestItem(t, core, "lock-aaa1", "Locked", "todo")
	return core, filepath.Join(itemsDir, b.Path), b.ETag()
}

// beans-9r10: an update with --if-match checks the etag and then writes. Under
// the in-process lock alone two processes can both pass the check; the item
// file's lock makes the pair atomic. A holder of that lock keeps an
// --if-match update waiting, and the update then checks the file as it is
// after the holder is done.
func TestUpdateIfMatchWaitsForTheItemLockAndThenChecksTheFile(t *testing.T) {
	core, path, etag := lockedItem(t)
	b, err := core.Get("lock-aaa1")
	if err != nil {
		t.Fatal(err)
	}
	b.Priority = "high"

	held, err := filelock.Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- core.Update(b, &etag) }()

	select {
	case err := <-done:
		t.Fatalf("Update() with --if-match returned while another process held the item lock (error = %v)", err)
	case <-time.After(150 * time.Millisecond):
	}

	// The holder changes the file, as the process that won the race would.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("Written by the lock holder.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	held.Release()

	select {
	case err := <-done:
		var mismatch *ETagMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("Update() after the holder wrote error = %v, want *ETagMismatchError", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Update() did not return after the lock was released")
	}
}

// An update without --if-match has no check to protect: it takes no lock, so
// a stuck holder cannot keep it waiting.
func TestUpdateWithoutIfMatchTakesNoItemLock(t *testing.T) {
	core, path, _ := lockedItem(t)
	b, err := core.Get("lock-aaa1")
	if err != nil {
		t.Fatal(err)
	}
	b.Priority = "high"

	held, err := filelock.Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer held.Release()

	done := make(chan error, 1)
	go func() { done <- core.Update(b, nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Update() without --if-match error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Update() without --if-match waited for the item lock")
	}
}
