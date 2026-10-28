//go:build windows

package main

import (
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

const storageProcessGroups = false

// Fail closed for unknown/group ownership. A shared cache must not cause
// another user's directories to be adopted merely because they are writable.
func storageOwned(dir string, _ os.FileInfo) bool {
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	return err == nil && owner.Equals(user.User.Sid)
}

func storageFreeBytes(dir string) (int64, error) {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var available uint64
	if err = windows.GetDiskFreeSpaceEx(name, &available, nil, nil); err != nil {
		return 0, err
	}
	if available > 1<<63-1 {
		return 1<<63 - 1, nil
	}
	return int64(available), nil
}

func inheritStorageLease(cmd *exec.Cmd, lease *cacheLock) {}
func isolateStorageCommand(cmd *exec.Cmd)                 {}
func stopStorageCommand(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
func storageGroupAlive(pid int) bool                  { return false }
func storageForeground(cmd *exec.Cmd) (func(), error) { return func() {}, nil }
