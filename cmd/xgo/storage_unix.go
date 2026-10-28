//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

const storageProcessGroups = true

func storageOwned(_ string, info os.FileInfo) bool {
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
	if cmd.Process != nil && cmd.Process.Pid > 1 {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
func storageGroupAlive(pid int) bool {
	// kill(-1, ...) means all permitted processes, not process group 1.
	// Supervised children have IDs > 1; unknown/legacy init-group records
	// must remain protected rather than probing the wrong group.
	if pid <= 1 {
		return true
	}
	err := syscall.Kill(-pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}

// Preserve interactive `xgo run` stdin after moving the job into its own
// process group. Ignore SIGTTOU while the supervisor restores the terminal.
func storageForeground(cmd *exec.Cmd) (func(), error) {
	fd := int(os.Stdin.Fd())
	previous, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil || previous != syscall.Getpgrp() {
		return func() {}, nil
	}
	signal.Ignore(syscall.SIGTTOU)
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, cmd.Process.Pid); err != nil {
		signal.Reset(syscall.SIGTTOU)
		// A short-lived command may have already exited before terminal handoff.
		if storageLeaderGone(cmd.Process.Pid) {
			return func() {}, nil
		}
		return func() {}, fmt.Errorf("give xgo job terminal foreground: %w", err)
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGCONT)
	return func() { _ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, previous); signal.Reset(syscall.SIGTTOU) }, nil
}

func storageLeaderGone(pid int) bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }
