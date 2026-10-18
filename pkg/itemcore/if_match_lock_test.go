package itemcore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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

// An update without --if-match checks the etag of its own copy, so it takes
// the item lock too: it waits for a holder and then writes.
func TestUpdateWithoutIfMatchWaitsForItemLock(t *testing.T) {
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

	done := make(chan error, 1)
	go func() { done <- core.Update(b, nil) }()
	select {
	case err := <-done:
		t.Fatalf("Update() returned while the lock was held: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	held.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Update() after release error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Update() did not return after the lock was released")
	}
}

// skipWhereOpenFilesCannotMove skips a test that renames or replaces a file
// while another handle is open on it. Unix allows that; Windows refuses it
// for a handle opened without FILE_SHARE_DELETE, which Go never sets.
func skipWhereOpenFilesCannotMove(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot rename or replace a file that is open")
	}
}

// waitingUpdate holds the item lock the way another lif process would and
// starts an --if-match update of the item, which has to wait for it. It
// returns the held lock and the channel the result of the update arrives on,
// once the update has had time to reach the lock.
func waitingUpdate(t *testing.T, core *Core, path, etag string) (*filelock.Lock, chan error) {
	t.Helper()
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
	return held, done
}

// requireMismatch fails unless err is the conflict error an --if-match update
// reports, and returns it.
func requireMismatch(t *testing.T, err error) *ETagMismatchError {
	t.Helper()
	var mismatch *ETagMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Update() error = %v, want *ETagMismatchError", err)
	}
	return mismatch
}

// requireItemFiles fails unless dir holds exactly the item files named.
func requireItemFiles(t *testing.T, dir string, want ...string) {
	t.Helper()
	got, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		got[i] = filepath.Base(got[i])
	}
	if !slices.Equal(got, want) {
		t.Errorf("item files = %v, want %v: the update that lost wrote a file name that was not its own", got, want)
	}
}

// beans-9r10 (review B1): the holder of the lock may be a title change, which
// renames the file once it is written. The writer that waited then finds its
// file gone. That is a conflict like any other. It used to read the etag from
// memory instead, pass, and write the old name again next to the new one.
func TestUpdateIfMatchWaitingForAWriterThatRenamesTheFileIsAConflict(t *testing.T) {
	skipWhereOpenFilesCannotMove(t)
	core, path, etag := lockedItem(t)
	held, done := waitingUpdate(t, core, path, etag)

	if err := os.Rename(path, filepath.Join(filepath.Dir(path), "lock-aaa1--new-title.md")); err != nil {
		t.Fatal(err)
	}
	held.Release()

	select {
	case err := <-done:
		requireMismatch(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Update() did not return after the lock was released")
	}
	requireItemFiles(t, filepath.Dir(path), "lock-aaa1--new-title.md")
}

// A file put in the place of the locked one, by an editor's atomic save or a
// git checkout, is not the file the lock protects, even when its bytes are the
// same and so is its etag. The waiting writer must not write into it.
func TestUpdateIfMatchWaitingForAFileThatIsReplacedIsAConflict(t *testing.T) {
	skipWhereOpenFilesCannotMove(t)
	core, path, etag := lockedItem(t)
	held, done := waitingUpdate(t, core, path, etag)

	same, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".new", same, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".new", path); err != nil {
		t.Fatal(err)
	}
	held.Release()

	select {
	case err := <-done:
		requireMismatch(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Update() did not return after the lock was released")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, same) {
		t.Errorf("the update wrote into the replacement file:\n%s", got)
	}
}

// The process that renamed or deleted the file may be done before this one
// even takes the lock; this one still holds the path it read at the start.
func TestUpdateIfMatchOfAFileThatWentAwayBeforeTheLockIsAConflict(t *testing.T) {
	for name, goAway := range map[string]func(path string) error{
		"renamed": func(path string) error {
			return os.Rename(path, filepath.Join(filepath.Dir(path), "lock-aaa1--new-title.md"))
		},
		"deleted": os.Remove,
	} {
		t.Run(name, func(t *testing.T) {
			core, path, etag := lockedItem(t)
			if err := goAway(path); err != nil {
				t.Fatal(err)
			}
			b, err := core.Get("lock-aaa1")
			if err != nil {
				t.Fatal(err)
			}
			b.Priority = "high"

			mismatch := requireMismatch(t, core.Update(b, &etag))

			if !strings.Contains(mismatch.Error(), "renamed, replaced or deleted") {
				t.Errorf("error = %q, want it to say the item file is gone", mismatch)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("the old file name exists again (stat error = %v)", err)
			}
		})
	}
}

func TestETagMismatchErrorNamesTheCurrentETagWhenThereIsOne(t *testing.T) {
	got := (&ETagMismatchError{Provided: "aaaa", Current: "bbbb"}).Error()
	if want := "etag mismatch: provided aaaa, current is bbbb"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// beans-9r10 (review N6): a holder that never lets go ends the wait with an
// error that says why, not with a conflict: the etag was never compared. The
// file stays as it was.
func TestUpdateIfMatchGivesUpOnAHolderThatNeverReleases(t *testing.T) {
	core, path, etag := lockedItem(t)
	b, err := core.Get("lock-aaa1")
	if err != nil {
		t.Fatal(err)
	}
	b.Priority = "high"
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	held, err := filelock.Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer held.Release()
	shortenItemLockTimeout(t, 50*time.Millisecond)

	err = core.Update(b, &etag)

	if !errors.Is(err, filelock.ErrTimeout) {
		t.Fatalf("Update() error = %v, want it to wrap filelock.ErrTimeout", err)
	}
	var mismatch *ETagMismatchError
	if errors.As(err, &mismatch) {
		t.Errorf("Update() reported a conflict, but the etag was never compared: %v", err)
	}
	if want := "item lock-aaa1 is locked by another process"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, before) {
		t.Errorf("the item file changed although the update gave up:\n%s", after)
	}
}

// shortenItemLockTimeout makes the update give up after d instead of 10 s.
func shortenItemLockTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	old := itemLockTimeout
	itemLockTimeout = d
	t.Cleanup(func() { itemLockTimeout = old })
}

// Any other reason the lock cannot be taken is reported as it is, with the
// item, and is no conflict either. A file that cannot be opened for writing is
// one: the write would fail on it anyway, so the update stops before it.
func TestUpdateIfMatchReportsAFileItCannotLock(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only file is opened for writing here")
	}
	core, path, etag := lockedItem(t)
	b, err := core.Get("lock-aaa1")
	if err != nil {
		t.Fatal(err)
	}
	b.Priority = "high"
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	err = core.Update(b, &etag)

	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Update() error = %v, want it to wrap fs.ErrPermission", err)
	}
	var mismatch *ETagMismatchError
	if errors.As(err, &mismatch) {
		t.Errorf("Update() reported a conflict for a file it could not open: %v", err)
	}
	if want := "locking item lock-aaa1"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
}
