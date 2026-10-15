// Package filelock takes an exclusive lock on an existing file that every
// other process of this machine using the package respects. The system drops
// the lock when its holder exits, so a crashed process cannot leave one
// behind. The lock excludes other lock holders only: it does not stop anyone
// from reading or writing the content of the file.
package filelock

import (
	"errors"
	"os"
	"time"
)

// ErrTimeout is returned by Acquire when another process holds the lock for
// the whole wait.
var ErrTimeout = errors.New("timed out waiting for the file lock")

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
