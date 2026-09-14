//go:build linux

package store

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// fileLock is an advisory flock wrapper for Linux.
type fileLock struct {
	f *os.File
}

// acquireFileLock opens the lock file and takes a shared or exclusive lock.
// It retries until timeout expires so concurrent CLI/HTTP processes serialize.
func acquireFileLock(path string, exclusive bool, timeout time.Duration) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	deadline := time.Now().Add(timeout)
	for {
		err = syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return &fileLock{f: f}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = f.Close()
			return nil, fmt.Errorf("locking file: %w", err)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("timed out waiting for lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// release drops the lock and closes the file.
func (l *fileLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}
