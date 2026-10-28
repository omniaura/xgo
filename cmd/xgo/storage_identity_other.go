//go:build !darwin && !linux

package main

import "os"

func storageProcessIdentity(pid int) (string, error) { return "", os.ErrInvalid }

func storageLeaderGone(pid int) bool { return false }
