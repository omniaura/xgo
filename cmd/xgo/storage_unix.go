//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func storageOwned(info os.FileInfo) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(s.Uid) == os.Getuid()
}
func inheritStorageLease(cmd *exec.Cmd, lease *cacheLock) {
	cmd.ExtraFiles = append(cmd.ExtraFiles, lease.file)
}

func storageFreeBytes(dir string) (int64, error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(dir, &s); err != nil {
		return 0, err
	}
	return int64(s.Bavail) * int64(s.Bsize), nil
}

func isolateStorageCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func stopStorageCommand(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
func storageGroupAlive(pid int) bool {
	err := syscall.Kill(-pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}

// Preserve interactive `xgo run` stdin after moving the job into its own
// process group. Ignore SIGTTOU while the supervisor restores the terminal.
func storageForeground(cmd *exec.Cmd) func() {
	fd := int(os.Stdin.Fd())
	previous, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		return func() {}
	}
	signal.Ignore(syscall.SIGTTOU)
	if unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, cmd.Process.Pid) == nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGCONT)
	}
	return func() { _ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, previous); signal.Reset(syscall.SIGTTOU) }
}
