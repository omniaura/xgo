package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorageDefaultPolicy(t *testing.T) {
	p, err := loadStoragePolicy(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxBytes != 20<<30 || p.MinFreeBytes != 10<<30 || p.CheckInterval != 2*time.Second || p.TempMaxAge != 24*time.Hour {
		t.Fatalf("defaults: %+v", p)
	}
}

func TestStorageEvictionUsesAgeRatherThanDirectoryName(t *testing.T) {
	m := testStorageManager(t)
	newer, lease := storageFixture(t, m, 0, "go-a-new", 700000, time.Hour)
	lease.close()
	older, lease := storageFixture(t, m, 0, "go-z-old", 700000, 2*time.Hour)
	lease.close()
	if err := m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, newer, true)
	storageExists(t, older, false)
}

func TestStorageStatFailureDoesNotEvict(t *testing.T) {
	m := testStorageManager(t)
	dir, lease := storageFixture(t, m, 0, "go-old", 2<<20, time.Hour)
	lease.close()
	m.free = func(string) (int64, error) { return 0, fmt.Errorf("statfs unavailable") }
	if err := m.enforce(); err == nil {
		t.Fatal("ignored statfs failure")
	}
	storageExists(t, dir, true)
}

func TestStorageChecksEveryVolume(t *testing.T) {
	m := testStorageManager(t)
	m.policy.MinFreeBytes = 100
	m.free = func(dir string) (int64, error) {
		if dir == m.roots[1].dir {
			return 0, nil
		}
		return 200, nil
	}
	if err := m.enforce(); err == nil || !strings.Contains(err.Error(), "low disk space") {
		t.Fatalf("missed temp volume shortage: %v", err)
	}
}

func TestStorageAccountingRefreshesSessionsAndExpiredCaches(t *testing.T) {
	m := testStorageManager(t)
	m.sizeCacheFile = filepath.Join(t.TempDir(), "usage.json")
	cache, cacheLease := storageFixture(t, m, 0, "go-cache", 100, time.Hour)
	cacheLease.close()
	session, _ := storageFixture(t, m, 1, "run-active", 100, time.Hour)
	if _, err := m.readEntries(true); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{cache, session} {
		f, err := os.OpenFile(filepath.Join(dir, "data"), os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.Truncate(200); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	entries, err := m.readEntries(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.dir == session && e.size != 200 {
			t.Fatal("session usage was memoized")
		}
	}
	// Expired accounting must rediscover writes in an existing cache file.
	records := map[string]storageSizeRecord{cache: {Bytes: 100, Scanned: time.Now().Add(-time.Minute)}}
	m.saveSizes(records)
	entries, err = m.readEntries(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.dir == cache && e.size != 200 {
			t.Fatal("expired cache accounting was reused")
		}
	}
}

func testStorageManager(t *testing.T) *storageManager {
	t.Helper()
	return &storageManager{policy: storagePolicy{MaxBytes: 1 << 20, TempMaxAge: time.Hour, CheckInterval: 100 * time.Millisecond}, roots: []storageRoot{{t.TempDir(), "go", false}, {t.TempDir(), "run-", true}}, free: func(string) (int64, error) { return 1 << 40, nil }}
}

func storageFixture(t *testing.T, m *storageManager, root int, name string, size int64, age time.Duration) (string, *cacheLock) {
	t.Helper()
	dir := filepath.Join(m.roots[root].dir, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	f.Close()
	markUsed(dir, time.Now().Add(-age))
	lease, err := lockCache(dir, false, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.close)
	return dir, lease
}

func storageExists(t *testing.T, dir string, want bool) {
	t.Helper()
	_, err := os.Stat(dir)
	if (err == nil) != want {
		t.Fatalf("%s exists=%v, want %v (err=%v)", dir, err == nil, want, err)
	}
}

func TestStorageBudgetEvictsOldestAcrossRoots(t *testing.T) {
	m := testStorageManager(t)
	old, lease := storageFixture(t, m, 0, "go-old", 800000, 3*time.Hour)
	lease.close()
	active, _ := storageFixture(t, m, 0, "go-active", 700000, time.Hour)
	temp, lease := storageFixture(t, m, 1, "run-abandoned", 100000, 2*time.Hour)
	lease.close()
	if err := m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, old, false)
	storageExists(t, temp, false)
	storageExists(t, active, true)
}

func TestStorageActiveAndUnknownCachesAreProtected(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "leased", true: "legacy-without-lease"}[unknown], func(t *testing.T) {
			m := testStorageManager(t)
			dir, lease := storageFixture(t, m, 0, "go-active", 2<<20, 48*time.Hour)
			if unknown {
				lease.close()
				if err := os.Remove(cacheLockPath(dir)); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.enforce(); err == nil || !strings.Contains(err.Error(), "budget exceeded") {
				t.Fatalf("expected budget error, got %v", err)
			}
			storageExists(t, dir, true)
		})
	}
}

func TestStorageLowSpaceReclaimsAndRefusesProtectedWork(t *testing.T) {
	m := testStorageManager(t)
	m.policy.MinFreeBytes = 100
	dir, lease := storageFixture(t, m, 0, "go-old", 1, 2*time.Hour)
	lease.close()
	m.free = func(string) (int64, error) {
		if _, err := os.Stat(dir); err == nil {
			return 0, nil
		}
		return 200, nil
	}
	if err := m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, dir, false)
	active, _ := storageFixture(t, m, 0, "go-active", 1, time.Hour)
	m.free = func(string) (int64, error) { return 0, nil }
	if err := m.enforce(); err == nil || !strings.Contains(err.Error(), "low disk space") {
		t.Fatalf("expected reserve refusal, got %v", err)
	}
	storageExists(t, active, true)
}

func TestStorageSymlinksAndForeignDirectories(t *testing.T) {
	m := testStorageManager(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(m.roots[0].dir, "go-symlink")); err != nil {
		t.Skip(err)
	}
	dir, lease := storageFixture(t, m, 0, "go-old", 2<<20, time.Hour)
	lease.close()
	if err := os.Symlink(outside, filepath.Join(dir, "outside")); err != nil {
		t.Fatal(err)
	}
	foreign, lease := storageFixture(t, m, 0, "other", 2<<20, time.Hour)
	lease.close()
	if err := m.enforce(); err != nil {
		t.Fatal(err)
	}
	storageExists(t, dir, false)
	storageExists(t, foreign, true)
	storageExists(t, filepath.Join(outside, "keep"), true)
}

func TestStorageConfigurationPrecedenceAndValidation(t *testing.T) {
	project := t.TempDir()
	nested := filepath.Join(project, "sub")
	os.Mkdir(nested, 0700)
	os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/project\n"), 0600)
	file := filepath.Join(project, ".xgo.yaml")
	os.WriteFile(file, []byte("cache-max-size: 3GiB\ncache-min-free: 1GiB\n"), 0600)
	p, err := loadStoragePolicy(nested, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxBytes != 3<<30 || p.MinFreeBytes != 1<<30 {
		t.Fatalf("project config: %+v", p)
	}
	t.Setenv("XGO_CACHE_MAX_SIZE", "4GiB")
	p, err = loadStoragePolicy(nested, nil)
	if err != nil || p.MaxBytes != 4<<30 {
		t.Fatalf("env precedence: %+v %v", p, err)
	}
	p, err = loadStoragePolicy(nested, map[string]string{"cache-max-size": "5GiB"})
	if err != nil || p.MaxBytes != 5<<30 {
		t.Fatalf("flag precedence: %+v %v", p, err)
	}
	for _, flags := range []map[string]string{{"config": filepath.Join(project, "missing.yaml")}, {"cache-max-size": "-1"}, {"cache-max-size": "0"}, {"cache-max-size": "9223372036854775807GiB"}, {"cache-min-free": "typo"}, {"disk-check-interval": "0s"}, {"temp-max-age": "1s"}} {
		if _, err := loadStoragePolicy(project, flags); err == nil {
			t.Fatalf("accepted invalid configuration %v", flags)
		}
	}
	os.WriteFile(file, []byte("cache-max-szie: 3GiB\n"), 0600)
	if _, err := loadStoragePolicy(project, nil); err == nil {
		t.Fatal("accepted unknown key")
	}
}

func TestStorageFlagPassthrough(t *testing.T) {
	for _, cmd := range []string{"build", "test", "run", "exec", "setup"} {
		opts, err := parseOptions(cmd, []string{"--cache-max-size=3GiB", "--cache-min-free", "1GiB", "--config", "project.yaml", "./example"})
		if err != nil {
			t.Fatal(err)
		}
		if opts.storageFlags["cache-max-size"] != "3GiB" || opts.storageFlags["cache-min-free"] != "1GiB" || opts.storageFlags["config"] != "project.yaml" {
			t.Fatalf("%s: %+v", cmd, opts.storageFlags)
		}
	}
	opts, err := parseOptions("run", []string{"./example", "--cache-max-size=program-argument"})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.storageFlags) != 0 || len(opts.remainArgs) != 2 {
		t.Fatalf("program args consumed: %+v", opts)
	}
	opts, err = parseOptions("test", []string{"./...", "-args", "--cache-max-size=program-argument"})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.storageFlags) != 0 || len(opts.testArgs) != 1 {
		t.Fatalf("test args consumed: %+v", opts)
	}
}
