//go:build darwin || linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStorageAbandonedSessionProtectsLiveProcessGroup(t *testing.T) {
	m := testStorageManager(t)
	dir, lease := storageFixture(t, m, 1, "run-old", 1, 48*time.Hour)
	lease.close()
	os.WriteFile(filepath.Join(dir, ".process-group"), []byte(strconv.Itoa(syscall.Getpgrp())), 0600)
	if err := m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, dir, true)
	os.Remove(filepath.Join(dir, ".process-group"))
	if err := m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, dir, false)
}

func TestStorageGuardStopsOwnedJobAndPreservesOtherJob(t *testing.T) {
	m := testStorageManager(t)
	other := exec.Command("sh", "-c", "sleep 60")
	isolateStorageCommand(other)
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stopStorageCommand(other); other.Wait() }()
	dir, lease, err := m.newSession()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.close()
	job := exec.Command("sh", "-c", `dd if=/dev/zero of="$1/data" bs=1048576 count=2 2>/dev/null; sleep 60`, "sh", dir)
	start := time.Now()
	err = m.runCommand(job, dir, lease)
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Fatalf("expected guard error, got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("guard did not stop the job promptly")
	}
	if !storageGroupAlive(other.Process.Pid) {
		t.Fatal("guard killed another job")
	}
	if err := other.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("other job is gone: %v", err)
	}
}

func TestStorageInheritedLeaseSurvivesParentDescriptorClose(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	lease, err := lockCache(dir, false, true)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command("sh", "-c", "sleep 60")
	isolateStorageCommand(child)
	inheritStorageLease(child, lease)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if child.ProcessState == nil {
			stopStorageCommand(child)
			child.Wait()
		}
	}()
	// Simulate abrupt parent death: close without explicit LOCK_UN.
	lease.file.Close()
	lease.file = nil
	if exclusive, err := lockCache(dir, true, false); err == nil {
		exclusive.close()
		t.Fatal("live child lost its inherited cache lease")
	}
	stopStorageCommand(child)
	child.Wait()
	exclusive, err := lockCache(dir, true, false)
	if err != nil {
		t.Fatal(err)
	}
	exclusive.close()
}

func TestStorageReserveGuardStopsRunningJob(t *testing.T) {
	m := testStorageManager(t)
	m.policy.MinFreeBytes = 100
	dir, lease, err := m.newSession()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.close()
	// Simulate another process exhausting the reserve after work has started.
	m.free = func(string) (int64, error) {
		if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
			return 0, nil
		}
		return 200, nil
	}
	job := exec.Command("sh", "-c", `touch "$1/started"; sleep 60`, "sh", dir)
	if err = m.runCommand(job, dir, lease); err == nil || !strings.Contains(err.Error(), "low disk space") {
		t.Fatalf("expected running reserve guard, got %v", err)
	}
}

func TestStorageOrphanCompilerGroupProtectsItsCaches(t *testing.T) {
	for _, tomb := range []bool{false, true} {
		t.Run(map[bool]string{false: "cache", true: "renamed-tomb"}[tomb], func(t *testing.T) {
			m := testStorageManager(t)
			cache, lease := storageFixture(t, m, 0, "go-cache", 2<<20, time.Hour)
			lease.close()
			owned := cache
			if tomb {
				owned = filepath.Join(filepath.Dir(cache), gcTombPrefix+filepath.Base(cache))
				if err := os.Rename(cache, owned); err != nil {
					t.Fatal(err)
				}
			}
			session, lease := storageFixture(t, m, 1, "run-orphan", 1, time.Hour)
			lease.close()
			os.WriteFile(filepath.Join(session, ".process-group"), []byte(strconv.Itoa(syscall.Getpgrp())), 0600)
			t.Setenv("XGO_STORAGE_SESSION", session)
			if err := recordStorageResource(cache); err != nil {
				t.Fatal(err)
			}
			// No cache reader holds a lease. The surviving job group still owns it.
			if err := m.enforce(); err == nil {
				t.Fatal("orphan compiler ownership was ignored")
			}
			storageExists(t, owned, true)
			os.Remove(filepath.Join(session, ".process-group"))
			if err := m.enforce(); err != nil {
				t.Fatal(err)
			}
			storageExists(t, owned, false)
		})
	}
}

func TestStorageUninstrumentedSessionDoesNotBlockCacheReclamation(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-resources", true: "corrupt-resources"}[corrupt], func(t *testing.T) {
			m := testStorageManager(t)
			cache, lease := storageFixture(t, m, 0, "go-unused", 2<<20, time.Hour)
			lease.close()
			session, _ := storageFixture(t, m, 1, "run-live", 1, time.Hour)
			os.WriteFile(filepath.Join(session, ".process-group"), []byte(strconv.Itoa(syscall.Getpgrp())), 0600)
			if corrupt {
				os.WriteFile(filepath.Join(session, ".resources.json"), []byte("{corrupt"), 0600)
			}
			err := m.enforce()
			if corrupt && err == nil {
				t.Fatal("malformed ownership was ignored")
			}
			if !corrupt && err != nil {
				t.Fatal(err)
			}
			storageExists(t, cache, corrupt)
			storageExists(t, session, true)
		})
	}
}

func TestStorageTempPoolProtectsOrphanProcessGroups(t *testing.T) {
	m := testStorageWithPool(t)
	pool, lease, err := m.prepareTempPool()
	if err != nil {
		t.Fatal(err)
	}
	lease.close()
	old := time.Now().Add(-3 * time.Hour)
	os.Chtimes(filepath.Join(pool, gcMarkFile), old, old)
	session, lease := storageFixture(t, m, 2, "run-orphan", 1, time.Hour)
	lease.close()
	os.WriteFile(filepath.Join(session, ".process-group"), []byte(strconv.Itoa(syscall.Getpgrp())), 0600)
	t.Setenv("XGO_STORAGE_SESSION", session)
	if err = recordStorageResource(pool); err != nil {
		t.Fatal(err)
	}
	if err = m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, pool, true)
	os.Remove(filepath.Join(session, ".process-group"))
	if err = m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, pool, false)
}

func TestStorageRejectsRecycledGroupLeaderIdentity(t *testing.T) {
	dir := t.TempDir()
	child := exec.Command("sh", "-c", "sleep 60")
	isolateStorageCommand(child)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stopStorageCommand(child); child.Wait() }()
	identity, err := recordStorageGroup(dir, child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if identity == "" || !storageGroupAlive(child.Process.Pid) || !sessionGroupAlive(dir) {
		t.Fatal("live recorded group was not protected")
	}
	data, err := json.Marshal(storageGroupRecord{PID: child.Process.Pid, Identity: identity + "-recycled"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, ".process-group"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if sessionGroupAlive(dir) {
		t.Fatal("unrelated process identity kept an abandoned session alive")
	}
}

func TestStorageFinalEnforcementPreservesChildFailure(t *testing.T) {
	m := testStorageManager(t)
	m.policy.CheckInterval = time.Hour
	dir, lease, err := m.newSession()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.close()
	child := exec.Command("sh", "-c", `dd if=/dev/zero of="$1/data" bs=1048576 count=2 2>/dev/null; exit 7`, "sh", dir)
	err = m.runCommand(child, dir, lease)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 || !strings.Contains(err.Error(), "budget exceeded") {
		t.Fatalf("lost child or enforcement failure: %v", err)
	}
	code, stderr := parseErr(err)
	if code != 7 || !strings.Contains(string(stderr), "budget exceeded") {
		t.Fatalf("lost CLI diagnostics: %d %s", code, stderr)
	}
}

func TestStorageWrappedProcessErrorRetainsCapturedStderr(t *testing.T) {
	_, err := exec.Command("sh", "-c", "echo captured-child-diagnostic >&2; exit 7").Output()
	if err == nil {
		t.Fatal("child unexpectedly succeeded")
	}
	combined := errors.Join(fmt.Errorf("worker: %w", err), errors.New("storage context"))
	code, diagnostic := parseErr(combined)
	if code != 7 || strings.Count(string(diagnostic), "captured-child-diagnostic") != 1 || !strings.Contains(string(diagnostic), "storage context") {
		t.Fatalf("lost or duplicated diagnostics: %d %s", code, diagnostic)
	}
}
