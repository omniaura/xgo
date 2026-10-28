//go:build !darwin && !linux

package main

import (
	"os"
	"os/exec"
)

func storageOwned(info os.FileInfo) bool                  { return true }
func inheritStorageLease(cmd *exec.Cmd, lease *cacheLock) {}

// Disk reserve enforcement and process-group ownership are supported on macOS/Linux.
func storageFreeBytes(dir string) (int64, error) { return 1<<63 - 1, nil }
func isolateStorageCommand(cmd *exec.Cmd)        {}
func stopStorageCommand(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
func storageGroupAlive(pid int) bool { return true }

func storageForeground(cmd *exec.Cmd) func() { return func() {} }
