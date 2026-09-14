//go:build !linux

package store

import (
	"fmt"
	"time"
)

type fileLock struct{}

func acquireFileLock(path string, exclusive bool, timeout time.Duration) (*fileLock, error) {
	return nil, fmt.Errorf("file locking is not supported on this platform")
}

func (l *fileLock) release() {}
