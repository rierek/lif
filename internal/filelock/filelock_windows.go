//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockOffset is the single byte the lock sits on, 1 GiB into the file and far
// behind the end of any item file. Windows byte range locks are mandatory for
// the bytes they cover: a lock on the content would make every other read of
// the file fail, and the write of the holder itself, which goes through a
// second handle.
const lockOffset = 1 << 30

// tryLock takes LockFileEx on f without waiting; held reports that another
// handle has the lock. The system drops the lock with the handle, which it
// closes with its process.
func tryLock(f *os.File) (held bool, err error) {
	err = windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &windows.Overlapped{Offset: lockOffset})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return true, nil
	}
	return false, err
}

func unlock(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &windows.Overlapped{Offset: lockOffset})
}
