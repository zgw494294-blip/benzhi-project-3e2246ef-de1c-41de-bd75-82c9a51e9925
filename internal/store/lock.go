package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// withDirectoryLock serializes commits across processes that share the same
// data directory. The event log is the source of truth, but two repository
// instances opened against one directory would otherwise read stale in-memory
// sequence numbers and append duplicate global sequences. An exclusive flock on
// a dedicated lock file guarantees that only one process computes the next
// sequence and appends at a time.
func withDirectoryLock(directory string, action func() error) error {
	lockPath := filepath.Join(directory, "repository.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("打开仓储锁: %w", err)
	}
	defer file.Close()
	if err := flockExclusive(file); err != nil {
		return fmt.Errorf("获取仓储锁: %w", err)
	}
	defer flockUnlock(file)
	return action()
}

func flockExclusive(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func flockUnlock(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
