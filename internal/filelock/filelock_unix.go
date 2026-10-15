//go:build !windows

package filelock

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes flock(2) on f without waiting; held reports that another
// open file description has it. The lock is advisory and tied to the open
// file, which the kernel closes with its process, like internal/search does
// for its index.
func tryLock(f *os.File) (held bool, err error) {
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			return false, nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EWOULDBLOCK):
			return true, nil
		}
		return false, err
	}
}

func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
