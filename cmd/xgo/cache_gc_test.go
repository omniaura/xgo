package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGCStaleCaches(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	parent := t.TempDir()
	mk := func(name string, mark time.Time, modTime time.Time) string {
		dir := filepath.Join(parent, name)
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
			t.Fatal(err)
		}
		if !mark.IsZero() {
			markFile := filepath.Join(dir, gcMarkFile)
			if err := os.WriteFile(markFile, nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(markFile, mark, mark); err != nil {
				t.Fatal(err)
			}
		}
		for _, p := range []string{filepath.Join(dir, "sub"), dir} {
			if err := os.Chtimes(p, modTime, modTime); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	old := now.Add(-staleBuildCacheAge - time.Hour)
	recent := now.Add(-time.Hour)

	inUse := mk("go1.27.2_in_use", time.Time{}, old)
	staleMarked := mk("go1.26.1_stale", old, old)
	recentMarked := mk("go1.26.2_recent", recent, old)
	staleUnmarked := mk("go1.25.0_unmarked", time.Time{}, old)
	recentUnmarked := mk("go1.25.1_unmarked_recent", time.Time{}, recent)
	foreign := mk("other", old, old)

	gcStaleCaches([]gcUnit{{dir: inUse, namePrefix: "go", maxAge: staleBuildCacheAge}}, now)

	for dir, wantExist := range map[string]bool{
		inUse:          true,
		staleMarked:    false,
		recentMarked:   true,
		staleUnmarked:  false,
		recentUnmarked: true,
		foreign:        true,
	} {
		_, err := os.Stat(dir)
		if exist := err == nil; exist != wantExist {
			t.Errorf("%s: exist=%v, want %v", filepath.Base(dir), exist, wantExist)
		}
	}
	if info, err := os.Stat(filepath.Join(inUse, gcMarkFile)); err != nil || !info.ModTime().Equal(now) {
		t.Errorf("in-use dir not marked: %v", err)
	}

	// scanned at most once per interval
	again := mk("go1.24.0_stale", old, old)
	gcStaleCaches([]gcUnit{{dir: inUse, namePrefix: "go", maxAge: staleBuildCacheAge}}, now.Add(time.Hour))
	if _, err := os.Stat(again); err != nil {
		t.Errorf("rescanned within gc interval: %v", err)
	}
	gcStaleCaches([]gcUnit{{dir: inUse, namePrefix: "go", maxAge: staleBuildCacheAge}}, now.Add(gcInterval+time.Hour))
	if _, err := os.Stat(again); err == nil {
		t.Errorf("stale dir kept after gc interval")
	}
}

func TestCacheGCUnitsSkipsExternalInstrumentDir(t *testing.T) {
	units := cacheGCUnits("/goroot-parent", "/tmp/xgo/go-instrument/x", "/tmp/xgo/go-instrument/x/build-cache", "", false)
	if len(units) != 1 || units[0].dir != "/tmp/xgo/go-instrument/x" {
		t.Fatalf("unexpected units: %+v", units)
	}
	units = cacheGCUnits("/home/.xgo/go-instrument/y", "/tmp/xgo/go-instrument/x", "/shared/go1.27.2_xgo1.2.9_abc", "/shared", true)
	if len(units) != 2 || units[0].dir != "/shared/go1.27.2_xgo1.2.9_abc" || units[0].nameContains != "_xgo" || units[1].dir != "/home/.xgo/go-instrument/y" {
		t.Fatalf("unexpected units: %+v", units)
	}
}
