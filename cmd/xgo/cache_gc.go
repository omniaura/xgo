package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xhd2015/xgo/cmd/xgo/pathsum"
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
	gcTombPrefix   = ".xgo-gc-removing-"
	gcMarkInterval = 1 * time.Hour
	gcInterval     = 24 * time.Hour

	// matches the age after which `go` trims an unused cache entry
	staleBuildCacheAge = 5 * 24 * time.Hour
	// an instrumented GOROOT is small but costs a minute to recreate
	staleInstrumentDirAge = 14 * 24 * time.Hour
	// an instrumented GOROOT from an xgo that did not mark it, whose build
	// cache is gone too, has no reliable last-use time
	staleUnmarkedInstrumentDirAge = 30 * 24 * time.Hour
)

type gcUnit struct {
	// the directory in use by this run
	dir string
	// only siblings of dir whose name has namePrefix and contains
	// nameContains are candidates
	namePrefix   string
	nameContains string
	maxAge       time.Duration
	// noMark: only collect siblings, dir is not in use by this run
	noMark bool
	// for instrumented GOROOTs: the parent of their default build caches.
	// Using a GOROOT does not touch it, so for one without a mark the
	// activity of its build cache tells when it was last used.
	cacheParent string
}

// gcStaleCaches marks every unit's dir as used and removes stale siblings.
// It is best-effort: errors only skip the affected directory.
func gcStaleCaches(units []gcUnit, now time.Time) {
	for _, u := range units {
		if !u.noMark {
			markUsed(u.dir, now)
		}
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
		recordGC(parent, now)
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), gcTombPrefix) {
				// left by an interrupted removal
				os.RemoveAll(filepath.Join(parent, e.Name()))
				continue
			}
			if !e.IsDir() || !strings.HasPrefix(e.Name(), u.namePrefix) || !strings.Contains(e.Name(), u.nameContains) {
				continue
			}
			dir := filepath.Join(parent, e.Name())
			if dir == u.dir {
				continue
			}
			if !u.isStale(dir, now) {
				continue
			}
			// Another run may start using dir right now. Move it out of the
			// way first, so no run can find it while it is half removed,
			// then check its mark again and restore it if it was just used.
			tomb := filepath.Join(parent, gcTombPrefix+e.Name())
			if os.Rename(dir, tomb) != nil {
				continue
			}
			if t, ok := markTime(tomb); ok && now.Sub(t) < u.maxAge {
				os.Rename(tomb, dir)
				continue
			}
			logDebug("gc: remove %s", dir)
			os.RemoveAll(tomb)
		}
	}
}

func (u gcUnit) isStale(dir string, now time.Time) bool {
	if t, ok := markTime(dir); ok {
		return now.Sub(t) >= u.maxAge
	}
	lastUsed, ok := lastUsedTime(dir)
	if !ok {
		return false
	}
	if u.cacheParent == "" {
		return now.Sub(lastUsed) >= u.maxAge
	}
	cacheDir, ok := instrumentCacheDirOf(u.cacheParent, dir)
	if ok {
		if t, ok := lastUsedTime(cacheDir); ok {
			if t.After(lastUsed) {
				lastUsed = t
			}
			return now.Sub(lastUsed) >= u.maxAge
		}
	}
	return now.Sub(lastUsed) >= staleUnmarkedInstrumentDirAge
}

// instrumentCacheDirOf returns the default build cache dir xgo uses for the
// instrumented GOROOT dir (named <goVersion>_<path sum>), see main.go.
func instrumentCacheDirOf(cacheParent string, instrumentDir string) (string, bool) {
	name := filepath.Base(instrumentDir)
	i := strings.IndexByte(name, '_')
	if i < 0 {
		return "", false
	}
	mapped, err := pathsum.PathSum(name[:i+1], instrumentDir)
	if err != nil {
		return "", false
	}
	return filepath.Join(cacheParent, mapped), true
}

func markTime(dir string) (time.Time, bool) {
	info, err := os.Stat(filepath.Join(dir, gcMarkFile))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
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

// shouldGC reports whether parent was not scanned within gcInterval.
func shouldGC(parent string, now time.Time) bool {
	info, err := os.Stat(filepath.Join(parent, gcStampFile))
	if err != nil {
		return true
	}
	d := now.Sub(info.ModTime())
	return d >= gcInterval || d <= -gcMarkInterval
}

func recordGC(parent string, now time.Time) {
	stamp := filepath.Join(parent, gcStampFile)
	if os.WriteFile(stamp, []byte(now.Format(time.RFC3339)), 0644) == nil {
		os.Chtimes(stamp, now, now)
	}
}

// lastUsedTime returns when dir was last used by an xgo run. Directories
// written before xgo started marking them fall back to the newest
// modification time among dir and its direct entries, which for a build
// cache includes the trim stamp go refreshes daily while the cache is used.
func lastUsedTime(dir string) (time.Time, bool) {
	if t, ok := markTime(dir); ok {
		return t, true
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
// With XGO_BUILD_CACHE_DIR, default per-GOROOT caches left by earlier runs
// are still collected.
func cacheGCUnits(instrumentDir string, instrumentCacheDir string, buildCacheDir string, buildCacheOverride string, xgoManagedInstrumentDir bool) []gcUnit {
	defaultCache := gcUnit{dir: instrumentCacheDir, namePrefix: "go", maxAge: staleBuildCacheAge}
	var units []gcUnit
	if buildCacheOverride != "" {
		defaultCache.noMark = true
		units = append(units, gcUnit{dir: buildCacheDir, namePrefix: "go", nameContains: "_xgo", maxAge: staleBuildCacheAge})
	}
	units = append(units, defaultCache)
	if xgoManagedInstrumentDir {
		units = append(units, gcUnit{dir: instrumentDir, namePrefix: "go", maxAge: staleInstrumentDirAge, cacheParent: filepath.Dir(instrumentCacheDir)})
	}
	return units
}
