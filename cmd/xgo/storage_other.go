//go:build !darwin && !linux && !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const storageProcessGroups = false

func storageOwned(_ string, info os.FileInfo) bool        { return false }
func inheritStorageLease(cmd *exec.Cmd, lease *cacheLock) {}

// Disk reserve enforcement and process-group ownership are supported on macOS/Linux.
func storageFreeBytes(dir string) (int64, error) {
	return 0, fmt.Errorf("disk free-space checks are unsupported on %s; explicitly set --cache-min-free=0 to disable the reserve", runtime.GOOS)
}
func isolateStorageCommand(cmd *exec.Cmd) {}
func stopStorageCommand(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
func storageGroupAlive(pid int) bool { return false }

func storageForeground(cmd *exec.Cmd) func() { return func() {} }
