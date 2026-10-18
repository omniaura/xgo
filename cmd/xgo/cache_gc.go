package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// xgo keeps one instrumented GOROOT per Go toolchain and one instrumented
// build cache per instrumented GOROOT (or, with XGO_BUILD_CACHE_DIR, per xgo
// revision). Go trims a build cache only while something keeps using it, so
// a toolchain or xgo upgrade orphans the previous ones forever. Every run
// marks what it uses, and at most once per gcInterval removes siblings that
// have not been used for their max age.
const (
	gcMarkFile     = ".xgo-last-used"
	gcStampFile    = ".xgo-gc.txt"
	gcMarkInterval = 1 * time.Hour
	gcInterval     = 24 * time.Hour

	// matches the age after which `go` trims an unused cache entry
	staleBuildCacheAge = 5 * 24 * time.Hour
	// an instrumented GOROOT is small but costs a minute to recreate
	staleInstrumentDirAge = 14 * 24 * time.Hour
)

type gcUnit struct {
	// the directory in use by this run
	dir string
	// only siblings of dir whose name has namePrefix and contains
	// nameContains are candidates
	namePrefix   string
	nameContains string
	maxAge       time.Duration
}

// gcStaleCaches marks every unit's dir as used and removes stale siblings.
// It is best-effort: errors only skip the affected directory.
func gcStaleCaches(units []gcUnit, now time.Time) {
	for _, u := range units {
		markUsed(u.dir, now)
	}
	for _, u := range units {
		parent := filepath.Dir(u.dir)
		if !shouldGC(parent, now) {
			continue
		}
		entries, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), u.namePrefix) || !strings.Contains(e.Name(), u.nameContains) {
				continue
			}
			dir := filepath.Join(parent, e.Name())
			if dir == u.dir {
				continue
			}
			lastUsed, ok := lastUsedTime(dir)
			if !ok || now.Sub(lastUsed) < u.maxAge {
				continue
			}
			logDebug("gc: remove %s, last used %s", dir, lastUsed.Format(time.RFC3339))
			os.RemoveAll(dir)
		}
	}
}

func markUsed(dir string, now time.Time) {
	mark := filepath.Join(dir, gcMarkFile)
	if info, err := os.Stat(mark); err == nil && now.Sub(info.ModTime()) < gcMarkInterval {
		return
	}
	if os.Chtimes(mark, now, now) == nil {
		return
	}
	if os.WriteFile(mark, nil, 0644) == nil {
		os.Chtimes(mark, now, now)
	}
}

// shouldGC reports whether parent was not scanned within gcInterval, and
// records this scan.
func shouldGC(parent string, now time.Time) bool {
	stamp := filepath.Join(parent, gcStampFile)
	if info, err := os.Stat(stamp); err == nil {
		if d := now.Sub(info.ModTime()); d < gcInterval && d > -gcMarkInterval {
			return false
		}
	}
	if err := os.WriteFile(stamp, []byte(now.Format(time.RFC3339)), 0644); err != nil {
		return false
	}
	return true
}

// lastUsedTime returns when dir was last used by an xgo run. Directories
// written before xgo started marking them fall back to the newest
// modification time among dir and its direct entries, which for a build
// cache includes the trim stamp go refreshes daily while the cache is used.
func lastUsedTime(dir string) (time.Time, bool) {
	if info, err := os.Stat(filepath.Join(dir, gcMarkFile)); err == nil {
		return info.ModTime(), true
	}
	info, err := os.Stat(dir)
	if err != nil {
		return time.Time{}, false
	}
	latest := info.ModTime()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}
	for _, e := range entries {
		ei, err := e.Info()
		if err != nil {
			return time.Time{}, false
		}
		if ei.ModTime().After(latest) {
			latest = ei.ModTime()
		}
		if e.IsDir() && e.Name() == "build-cache" {
			// the default per-GOROOT cache dir wraps the GOCACHE
			if t, ok := lastUsedTime(filepath.Join(dir, e.Name())); ok && t.After(latest) {
				latest = t
			}
		}
	}
	return latest, true
}

// cacheGCUnits lists what this run uses: the build cache (the per-GOROOT
// cache dir, or the namespace under XGO_BUILD_CACHE_DIR) and, unless GOROOT
// was instrumented elsewhere, the instrumented GOROOT under the xgo home.
func cacheGCUnits(instrumentDir string, instrumentCacheDir string, buildCacheDir string, buildCacheOverride string, xgoManagedInstrumentDir bool) []gcUnit {
	units := []gcUnit{{dir: instrumentCacheDir, namePrefix: "go", maxAge: staleBuildCacheAge}}
	if buildCacheOverride != "" {
		units[0] = gcUnit{dir: buildCacheDir, namePrefix: "go", nameContains: "_xgo", maxAge: staleBuildCacheAge}
	}
	if xgoManagedInstrumentDir {
		units = append(units, gcUnit{dir: instrumentDir, namePrefix: "go", maxAge: staleInstrumentDirAge})
	}
	return units
}

