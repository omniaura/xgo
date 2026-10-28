package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Locks live beside, never inside, the directory they protect. Removing or
// publishing a cache must not replace the inode on which other runs wait.
// Lock files are intentionally retained, including after GC.
var cacheLeasesMu sync.Mutex
var cacheLeases = map[*cacheLock]bool{}

type cacheLock struct {
	file *os.File
}

func lockCache(dir string, exclusive, wait bool) (*cacheLock, error) {
	return openCacheLock(dir, exclusive, wait, true)
}
func lockCacheForGC(dir string) (*cacheLock, error) { return openCacheLock(dir, true, false, false) }
func openCacheLock(dir string, exclusive, wait, record bool) (*cacheLock, error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(cacheLockPath(dir), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f, exclusive, wait); err != nil {
		f.Close()
		return nil, err
	}
	if record {
		if err := recordStorageResource(dir); err != nil {
			unlockFile(f)
			f.Close()
			return nil, err
		}
	}
	lease := &cacheLock{file: f}
	if record {
		cacheLeasesMu.Lock()
		cacheLeases[lease] = true
		cacheLeasesMu.Unlock()
	}
	return lease, nil
}

func (l *cacheLock) close() {
	cacheLeasesMu.Lock()
	defer cacheLeasesMu.Unlock()
	delete(cacheLeases, l)
	if l != nil && l.file != nil {
		unlockFile(l.file)
		l.file.Close()
		l.file = nil
	}
}

func cacheLockPath(dir string) string {
	return filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".lock")
}

// Child Go/setup processes retain the leases if the xgo worker is killed.
func inheritActiveCacheLeases(cmd *exec.Cmd) {
	cacheLeasesMu.Lock()
	defer cacheLeasesMu.Unlock()
	for lease := range cacheLeases {
		if lease.file != nil {
			inheritStorageLease(cmd, lease)
		}
	}
}
