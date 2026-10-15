// Package filelock takes an exclusive lock on an existing file that every
// other process of this machine using the package respects. The system drops
// the lock when its holder exits, so a crashed process cannot leave one
// behind. The lock excludes other lock holders only: it does not stop anyone
// from reading or writing the content of the file.
package filelock

import (
	"errors"
	"io"
	"os"
	"time"
)

// ErrTimeout is returned by Acquire when another process holds the lock for
// the whole wait.
var ErrTimeout = errors.New("timed out waiting for the file lock")

// ErrReplaced is returned by ReadFile when path no longer names the locked
// file: another process put a different file under that name.
var ErrReplaced = errors.New("the locked file was replaced")

// pollInterval is the pause between two attempts on a held lock.
const pollInterval = 10 * time.Millisecond

// Lock is a held file lock.
type Lock struct {
	f *os.File
}

// Acquire opens the existing file at path and takes an exclusive lock on it.
// It waits up to timeout for another holder to release the lock and then
// returns ErrTimeout. A missing file is the error of os.OpenFile.
func Acquire(path string, timeout time.Duration) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		held, err := tryLock(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if !held {
			return &Lock{f: f}, nil
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, ErrTimeout
		}
		time.Sleep(pollInterval)
	}
}

// Release drops the lock and closes the file. It is safe on a nil Lock.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ReadFile returns the content of the locked file, provided path still names
// it. It reads through the locked handle, so the bytes are those of the file
// the lock protects, and it checks the name afterwards, as close to the
// caller's next step as it can. A file that was renamed away or deleted since
// Acquire gives the error of os.Stat (os.IsNotExist), a different file under
// the name ErrReplaced: either way the lock no longer guards what the name
// leads to, and a write to path would not be protected by it.
func (l *Lock) ReadFile(path string) ([]byte, error) {
	if _, err := l.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(l.f)
	if err != nil {
		return nil, err
	}
	named, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	locked, err := l.f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(locked, named) {
		return nil, ErrReplaced
	}
	return data, nil
}
