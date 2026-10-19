package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheLockProcess(t *testing.T) {
	if dir := os.Getenv("XGO_LOCK_TEST_DIR"); dir != "" {
		lease, err := lockCache(dir, true, false)
		if err == nil {
			lease.close()
			os.Exit(2)
		}
		return
	}
	dir := filepath.Join(t.TempDir(), "cache")
	lease, err := lockCache(dir, false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCacheLockProcess$")
	cmd.Env = append(os.Environ(), "XGO_LOCK_TEST_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child did not observe lease: %v: %s", err, out)
	}
	lease.close()
	exclusive, err := lockCache(dir, true, false)
	if err != nil {
		t.Fatal(err)
	}
	exclusive.close()
}

func TestPrepareInstrumentDirConcurrentAndRepair(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "goroot")
	var builds int32
	build := func(stage string) error {
		atomic.AddInt32(&builds, 1)
		if err := os.MkdirAll(filepath.Join(stage, "src", "html", "template"), 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(stage, "src", "html", "template", "template.go"), []byte("package template"), 0644)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := prepareInstrumentDir(dir, "revision", false, build)
			if err != nil {
				t.Error(err)
				return
			}
			defer lease.close()
			if !completeInstrumentDir(dir, "revision") {
				t.Error("published incomplete tree")
			}
		}()
	}
	wg.Wait()
	if builds != 1 {
		t.Fatalf("built %d times", builds)
	}
	if err := os.RemoveAll(filepath.Join(dir, "src", "html")); err != nil {
		t.Fatal(err)
	}
	lease, err := prepareInstrumentDir(dir, "revision", false, build)
	if err != nil {
		t.Fatal(err)
	}
	lease.close()
	if builds != 2 || !completeInstrumentDir(dir, "revision") {
		t.Fatal("did not repair partially deleted tree")
	}
	if completeInstrumentDir(dir, "other-revision") {
		t.Fatal("accepted wrong revision")
	}
}

func TestPrepareInstrumentDirFailureAndReset(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "goroot")
	build := func(stage string) error {
		return os.WriteFile(filepath.Join(stage, "file"), []byte("ready"), 0644)
	}
	lease, err := prepareInstrumentDir(dir, "r", false, build)
	if err != nil {
		t.Fatal(err)
	}
	lease.close()
	_, err = prepareInstrumentDir(dir, "r", true, func(stage string) error {
		return fmt.Errorf("interrupted setup")
	})
	if err == nil || !completeInstrumentDir(dir, "r") {
		t.Fatal("failed staging build damaged published tree")
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() && e.Name() != "goroot" {
			t.Fatalf("leaked stage: %s", e.Name())
		}
	}
}

func TestGCSkipsLiveLease(t *testing.T) {
	parent := t.TempDir()
	current := filepath.Join(parent, "go_current")
	live := filepath.Join(parent, "go_live")
	for _, dir := range []string{current, live} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	markUsed(live, now.Add(-2*staleBuildCacheAge))
	lease, err := lockCache(live, false, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.close()
	units := []gcUnit{{dir: current, namePrefix: "go", maxAge: staleBuildCacheAge}}
	gcStaleCaches(units, now)
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("GC deleted live directory: %v", err)
	}
	lease.close()
	gcStaleCaches(units, now.Add(2*gcInterval))
	if _, err := os.Stat(live); !os.IsNotExist(err) {
		t.Fatalf("GC did not collect released directory: %v", err)
	}
}

func TestInstrumentRevisionKey(t *testing.T) {
	if instrumentRevisionKey("a", true) == instrumentRevisionKey("b", true) ||
		instrumentRevisionKey("a", true) == instrumentRevisionKey("a", false) {
		t.Fatal("setup variants must coexist")
	}
}

func TestResetWaitsForConsumer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "goroot")
	build := func(stage string) error {
		return os.WriteFile(filepath.Join(stage, "file"), []byte("ready"), 0644)
	}
	lease, err := prepareInstrumentDir(dir, "r", false, build)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		next, err := prepareInstrumentDir(dir, "r", true, func(stage string) error {
			close(entered)
			return build(stage)
		})
		if next != nil {
			next.close()
		}
		done <- err
	}()
	select {
	case <-entered:
		t.Error("reset started while a consumer held a lease")
	case <-time.After(50 * time.Millisecond):
	}
	lease.close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reset did not resume after consumer exited")
	}
}
