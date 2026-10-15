package filelock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func lockedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "item.md")
	if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAcquireWaitsForTheHolderToRelease(t *testing.T) {
	path := lockedFile(t)
	first, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	got := make(chan error, 1)
	go func() {
		second, err := Acquire(path, 10*time.Second)
		second.Release()
		got <- err
	}()

	select {
	case err := <-got:
		t.Fatalf("a second Acquire() returned while the lock was held (error = %v)", err)
	case <-time.After(150 * time.Millisecond):
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	select {
	case err := <-got:
		if err != nil {
			t.Errorf("Acquire() after the release error = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire() did not return after the holder released the lock")
	}
}

func TestAcquireGivesUpWithErrTimeout(t *testing.T) {
	path := lockedFile(t)
	held, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer held.Release()

	if _, err := Acquire(path, 50*time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Errorf("Acquire() of a held lock error = %v, want ErrTimeout", err)
	}
}

func TestAcquireOfAMissingFileIsNotExist(t *testing.T) {
	_, err := Acquire(filepath.Join(t.TempDir(), "gone.md"), time.Second)
	if !os.IsNotExist(err) {
		t.Errorf("Acquire() error = %v, want a not-exist error", err)
	}
}

func TestReleaseOfNilLockIsSafe(t *testing.T) {
	var l *Lock
	if err := l.Release(); err != nil {
		t.Errorf("Release() on a nil Lock error = %v", err)
	}
}

// helperEnv names the file the helper process locks.
const helperEnv = "FILELOCK_TEST_HOLD"

// TestHelperHoldLock is not a test: it is the second process of
// TestLockEndsWithItsHolderProcess. It takes the lock, says so, and holds it
// until its stdin closes or the parent kills it.
func TestHelperHoldLock(t *testing.T) {
	path := os.Getenv(helperEnv)
	if path == "" {
		t.Skip("helper process of TestLockEndsWithItsHolderProcess")
	}
	if _, err := Acquire(path, 5*time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "helper:", err)
		os.Exit(2)
	}
	fmt.Println("locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

// The lock holds against another process, and the system takes it away when
// its holder dies, with no clean-up by the holder: a crashed lif must not
// leave an item locked.
func TestLockEndsWithItsHolderProcess(t *testing.T) {
	path := lockedFile(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldLock$")
	cmd.Env = append(os.Environ(), helperEnv+"="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper process said %q (error = %v), want \"locked\"", line, err)
	}

	if _, err := Acquire(path, 100*time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Errorf("Acquire() while another process holds the lock error = %v, want ErrTimeout", err)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	l, err := Acquire(path, 5*time.Second)
	if err != nil {
		t.Fatalf("Acquire() after the holder died error = %v", err)
	}
	l.Release()
}

// skipWhereOpenFilesCannotMove skips a test that renames or replaces a file
// while a lock handle is open on it. Unix allows that; Windows refuses it for
// a handle opened without FILE_SHARE_DELETE, which Go never sets.
func skipWhereOpenFilesCannotMove(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot rename or replace a file that is open")
	}
}

func TestReadFileReturnsTheContentOfTheLockedFile(t *testing.T) {
	path := lockedFile(t)
	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer l.Release()

	for i := range 2 { // a second read starts at the beginning again
		got, err := l.ReadFile(path)
		if err != nil || string(got) != "content\n" {
			t.Fatalf("read %d: ReadFile() = %q, %v; want %q", i, got, err, "content\n")
		}
	}
}

// A writer that waited for the lock may find the file renamed (a title change
// renames it once it is written), deleted or replaced. The lock then holds a
// file that has lost its name, and the caller must hear that, not read a
// stale copy and write the old name again.
func TestReadFileReportsAFileRenamedAfterTheLock(t *testing.T) {
	skipWhereOpenFilesCannotMove(t)
	path := lockedFile(t)
	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer l.Release()
	if err := os.Rename(path, path+".renamed"); err != nil {
		t.Fatal(err)
	}

	if _, err := l.ReadFile(path); !os.IsNotExist(err) {
		t.Errorf("ReadFile() of a renamed file error = %v, want a not-exist error", err)
	}
}

func TestReadFileReportsAFileReplacedAfterTheLock(t *testing.T) {
	skipWhereOpenFilesCannotMove(t)
	path := lockedFile(t)
	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer l.Release()
	// What an editor's atomic save or a git checkout does: a new file, put in
	// place of the old one by a rename.
	if err := os.WriteFile(path+".new", []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".new", path); err != nil {
		t.Fatal(err)
	}

	if _, err := l.ReadFile(path); !errors.Is(err, ErrReplaced) {
		t.Errorf("ReadFile() of a replaced file error = %v, want ErrReplaced", err)
	}
}
