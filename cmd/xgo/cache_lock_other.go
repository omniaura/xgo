//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !windows

package main

import (
	"os"
	"time"
)

// On platforms without native advisory locks, serialize readers too. Never
// reclaim an apparently stale lock: a paused process may still be using it.
// After an unclean exit, the .held file must be removed manually.
func lockFile(f *os.File, exclusive, wait bool) error {
	for {
		h, err := os.OpenFile(f.Name()+".held", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			return h.Close()
		}
		if !os.IsExist(err) || !wait {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func unlockFile(f *os.File) {
	os.Remove(f.Name() + ".held")
}
