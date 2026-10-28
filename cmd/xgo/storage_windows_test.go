//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsStorageNativeSpaceAndOwnership(t *testing.T) {
	dir := t.TempDir()
	free, err := storageFreeBytes(dir)
	if err != nil || free <= 0 || free == 1<<63-1 {
		t.Fatalf("native available space: %d %v", free, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if storageOwned(filepath.Join(dir, "does-not-exist"), info) {
		t.Fatal("assumed ownership when the native lookup failed")
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	// Elevated/shared group-owned directories must fail closed too.
	if storageOwned(dir, info) != owner.Equals(user.User.Sid) {
		t.Fatal("owner SID was not respected")
	}
}
