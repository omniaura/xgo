package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type storageRoot struct {
	dir, prefix string
	sessions    bool
	temporary   bool
}
type storageEntry struct {
	dir       string
	size      int64
	used      time.Time
	sessions  bool
	temporary bool
}
type storageManager struct {
	policy        storagePolicy
	roots         []storageRoot
	free          func(string) (int64, error)
	sizeCacheFile string
	onJobDone     func()
}

func newStorageManager(p storagePolicy, xgoHome string) (*storageManager, error) {
	home, err := getOrMakeAbsXgoHome(xgoHome)
	if err != nil {
		return nil, err
	}
	tmp, err := getStableTmpDir()
	if err != nil {
		return nil, err
	}
	base := p.CacheDir
	if base == "" {
		base = filepath.Join(tmp, "xgo")
	}
	p.CacheDir = base
	m := &storageManager{policy: p, free: storageFreeBytes}
	for _, suffix := range []string{"", "-dev"} {
		m.roots = append(m.roots, storageRoot{filepath.Join(base, "go-instrument"+suffix), "go", false, false}, storageRoot{filepath.Join(home, "go-instrument"+suffix), "go", false, false})
	}
	if override := os.Getenv(XGO_BUILD_CACHE_DIR); override != "" {
		dir, err := filepath.Abs(override)
		if err != nil {
			return nil, err
		}
		m.roots = append(m.roots, storageRoot{dir, "go", false, false})
	}
	m.roots = append(m.roots, storageRoot{dir: filepath.Join(base, fmt.Sprintf("temps-%d", os.Getuid())), prefix: "pool-", temporary: true})
	m.roots = append(m.roots, storageRoot{filepath.Join(base, fmt.Sprintf("sessions-%d", os.Getuid())), "run-", true, true})
	for i, a := range m.roots {
		for j, b := range m.roots {
			if i == j || a.dir == b.dir {
				continue
			}
			rel, err := filepath.Rel(a.dir, b.dir)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("overlapping xgo storage roots: %s and %s", a.dir, b.dir)
			}
		}
	}
	var identity string
	for _, r := range m.roots {
		identity += r.dir + "\n"
	}
	m.sizeCacheFile = filepath.Join(base, fmt.Sprintf(".xgo-usage-%x.json", sha256.Sum256([]byte(identity))))
	return m, nil
}

func (m *storageManager) entries() ([]storageEntry, error) { return m.readEntries(false) }

func (m *storageManager) readEntries(cached bool) ([]storageEntry, error) {
	memo := map[string]storageSizeRecord{}
	if cached && m.sizeCacheFile != "" {
		data, err := os.ReadFile(m.sizeCacheFile)
		if err == nil && len(data) < 1<<20 {
			_ = json.Unmarshal(data, &memo)
		}
	}
	if memo == nil {
		memo = map[string]storageSizeRecord{}
	}
	next := map[string]storageSizeRecord{}
	changed := false
	defer func() {
		if changed && m.sizeCacheFile != "" {
			m.saveSizes(next)
		}
	}()
	var entries []storageEntry
	seen := map[string]bool{}
	for _, root := range m.roots {
		items, err := os.ReadDir(root.dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !item.IsDir() || !(strings.HasPrefix(item.Name(), root.prefix) || strings.HasPrefix(item.Name(), gcSetupPrefix) || strings.HasPrefix(item.Name(), gcTombPrefix)) {
				continue
			}
			dir := filepath.Join(root.dir, item.Name())
			if seen[dir] {
				continue
			}
			seen[dir] = true
			info, err := item.Info()
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			if !storageOwned(dir, info) {
				continue
			}
			var size int64
			signature := storageImmutableSignature(dir)
			record, ok := memo[dir]
			age := time.Since(record.Scanned)
			if !root.sessions && !root.temporary && ok && record.Bytes >= 0 && age >= 0 && (signature != "" && signature == record.Signature || age < 5*time.Second) {
				size = record.Bytes
				next[dir] = record
			} else {
				size, err = storageDirSize(dir)
				if cached && !root.sessions && !root.temporary && err == nil {
					next[dir] = storageSizeRecord{Bytes: size, Scanned: time.Now(), Signature: signature}
					changed = true
				}
			}
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			used, ok := lastUsedTime(dir)
			if !ok {
				used = info.ModTime()
			}
			entries = append(entries, storageEntry{dir, size, used, root.sessions, root.temporary})
		}
	}
	return entries, nil
}

// WalkDir does not follow symlinks. Count logical bytes conservatively: sparse
// files and shared APFS blocks must never make the budget look smaller.
func storageDirSize(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if e.Type().IsRegular() {
			info, err := e.Info()
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func existingStorageAncestor(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

type storagePressureError string

func (e storagePressureError) Error() string { return string(e) }

func (m *storageManager) pressure(entries []storageEntry) error {
	var total int64
	for _, e := range entries {
		total += e.size
	}
	var low error
	seen := map[string]bool{}
	for _, root := range m.roots {
		if m.policy.MinFreeBytes == 0 {
			break
		}
		dir := existingStorageAncestor(root.dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		free, err := m.free(dir)
		if err != nil {
			return fmt.Errorf("check free disk space at %s: %w", dir, err)
		}
		if free < m.policy.MinFreeBytes {
			low = storagePressureError(fmt.Sprintf("xgo low disk space at %s: %d bytes available, reserve %d", dir, free, m.policy.MinFreeBytes))
		}
	}
	if total > m.policy.MaxBytes {
		return storagePressureError(fmt.Sprintf("xgo storage budget exceeded: %d bytes used, limit %d", total, m.policy.MaxBytes))
	}
	return low
}

func sessionGroupAlive(dir string) bool {
	if !storageProcessGroups {
		return sessionLeaseAlive(dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".process-group"))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	var record storageGroupRecord
	if json.Unmarshal(data, &record) != nil {
		// Pre-release/legacy PID-only records remain conservative.
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		return err != nil || pid <= 0 || storageGroupAlive(pid)
	}
	if record.PID <= 0 {
		return true
	}
	if !storageGroupAlive(record.PID) {
		return false
	}
	if record.Identity == "" {
		return true
	}
	actual, err := storageProcessIdentity(record.PID)
	// The leader may have exited while descendants still own its group.
	if err != nil {
		return true
	}
	return actual == record.Identity
}

// A lock, not age/PID alone, establishes abandonment. Never delete unowned
// system go-build*/go-link* directories: legacy Go processes have no leases.
func (m *storageManager) remove(e storageEntry) bool { return m.removeIf(e, nil) }
func (m *storageManager) removeIf(e storageEntry, eligible func(string) bool) bool {
	leaseDir := e.dir
	if strings.HasPrefix(filepath.Base(e.dir), gcTombPrefix) {
		leaseDir = filepath.Join(filepath.Dir(e.dir), strings.TrimPrefix(filepath.Base(e.dir), gcTombPrefix))
	}
	// Older xgo versions did not take leases. An absent lock is unknown
	// ownership, not proof that an old build has finished.
	if !hasStorageLease(leaseDir) {
		return false
	}
	lease, err := lockCacheForGC(leaseDir)
	if err != nil {
		return false
	}
	defer lease.close()
	info, err := os.Lstat(e.dir)
	if err != nil || !info.IsDir() || !storageOwned(e.dir, info) {
		return false
	}
	if eligible != nil && !eligible(e.dir) {
		return false
	}
	if e.sessions {
		// On fallback platforms the exclusive lease just acquired proves
		// the session is unused; Unix also checks surviving process groups.
		if storageProcessGroups && sessionGroupAlive(e.dir) {
			return false
		}
		return removeStorageDir(e.dir)
	}
	for _, root := range m.roots {
		if root.sessions && resourceInLiveSession(leaseDir, root.dir) {
			return false
		}
	}
	return removeStorageDir(e.dir)
}

func (m *storageManager) enforce() error {
	entries, err := m.readEntries(true)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].used.Before(entries[j].used) })
	for _, e := range entries {
		if (e.sessions || e.temporary) && time.Since(e.used) >= m.policy.TempMaxAge {
			m.remove(e)
		}
	}
	entries, err = m.readEntries(true)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].used.Before(entries[j].used) })
	pressure := m.pressure(entries)
	if pressure == nil {
		return nil
	}
	var shortage storagePressureError
	if !errors.As(pressure, &shortage) {
		return pressure
	}
	for _, e := range entries {
		var total int64
		for _, current := range entries {
			total += current.size
		}
		var free int64
		if m.policy.MinFreeBytes > 0 {
			var err error
			free, err = m.free(existingStorageAncestor(filepath.Dir(e.dir)))
			if err != nil {
				return err
			}
		}
		if total <= m.policy.MaxBytes && free >= m.policy.MinFreeBytes {
			continue
		}
		// Recent directories protect the create-before-lock publication gap.
		if time.Since(e.used) < time.Minute {
			continue
		}
		if m.remove(e) {
			entries, err = m.readEntries(true)
			if err != nil {
				return err
			}
			if m.pressure(entries) == nil {
				return nil
			}
		}
	}
	entries, err = m.readEntries(true)
	if err != nil {
		return err
	}
	if err = m.pressure(entries); err != nil {
		return fmt.Errorf("%w; active/recent jobs are protected; free disk space or adjust --cache-max-size/--cache-min-free", err)
	}
	return nil
}

func (m *storageManager) newSession() (string, *cacheLock, error) {
	root := m.roots[len(m.roots)-1].dir
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(root, "run-")
	if err != nil {
		return "", nil, err
	}
	lease, err := lockCache(dir, false, true)
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	return dir, lease, nil
}

func superviseBuild(cmdName string, args []string) error {
	flags := args
	if cmdName != "exec" {
		flags = append(parseXgoFlags(os.Getenv("XGO_FLAGS")), args...)
	}
	opts, err := parseOptions(cmdName, flags)
	if err != nil {
		return err
	}
	p, err := loadStoragePolicy(opts.projectDir, opts.storageFlags)
	if err != nil {
		return err
	}
	m, err := newStorageManager(p, opts.xgoHome)
	if err != nil {
		return err
	}
	if err = m.enforce(); err != nil {
		return err
	}
	dir, lease, err := m.newSession()
	if err != nil {
		return err
	}
	defer lease.close()
	defer func() {
		if !storageProcessGroups {
			lease.close()
		}
		if !sessionGroupAlive(dir) {
			_ = os.RemoveAll(dir)
		}
	}()
	pool, poolLease, err := m.prepareTempPool()
	if err != nil {
		return err
	}
	cleanupPool := func() {
		markTempPoolUsed(pool)
		poolLease.close()
		m.removeIf(storageEntry{dir: pool, temporary: true}, emptyTempPool)
	}
	defer cleanupPool()
	m.onJobDone = cleanupPool
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	child := exec.Command(executable, append([]string{"__storage-worker", cmdName}, args...)...)
	child.Env = append(os.Environ(), "XGO_STORAGE_SESSION="+dir, "XGO_STORAGE_ROOT="+filepath.Dir(m.roots[len(m.roots)-1].dir), "XGO_STORAGE_TEMP="+pool, "GOTMPDIR="+pool, "TMPDIR="+pool, "TMP="+pool, "TEMP="+pool)
	inheritStorageLease(child, poolLease)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	return m.runCommand(child, dir, lease)
}

func (m *storageManager) runCommand(child *exec.Cmd, dir string, lease *cacheLock) error {
	var err error
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupts)
	isolateStorageCommand(child)
	inheritStorageLease(child, lease)
	if err = child.Start(); err != nil {
		return err
	}
	// If the supervisor dies, the child's lease and process group still
	// protect the session, including compilers that outlive the xgo worker.
	identity, recordErr := recordStorageGroup(dir, child.Process.Pid)
	if recordErr != nil {
		stopStorageCommand(child)
		_ = child.Wait()
		return recordErr
	}
	stopJob := func() {
		if storageProcessGroups {
			actual, lookupErr := storageProcessIdentity(child.Process.Pid)
			if lookupErr == nil && actual != identity {
				return
			}
			if lookupErr != nil && !storageLeaderGone(child.Process.Pid) {
				_ = child.Process.Kill()
				return
			}
		}
		stopStorageCommand(child)
	}
	restoreTerminal := storageForeground(child)
	defer restoreTerminal()
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	ticker := time.NewTicker(m.policy.CheckInterval)
	defer ticker.Stop()
	freeInterval := m.policy.CheckInterval
	if freeInterval > 500*time.Millisecond {
		freeInterval = 500 * time.Millisecond
	}
	freeTicker := time.NewTicker(freeInterval)
	defer freeTicker.Stop()
	// Usage walks may be slow on large caches. Keep statfs checks responsive
	// independently, so a slow scan cannot consume the free-space reserve.
	scanned := make(chan error, 1)
	scanning := false
	defer func() {
		if scanning {
			<-scanned
		}
	}()
	abort := func(err error) error { stopJob(); <-done; return err }
	for {
		select {
		case sig := <-interrupts:
			stopJob()
			<-done
			stopJob()
			return fmt.Errorf("xgo build interrupted: %s", sig)
		case err = <-done:
			// Reap background descendants owned by this invocation too.
			stopJob()
			if scanning {
				<-scanned
				scanning = false
			}
			if m.onJobDone != nil {
				m.onJobDone()
			}
			if checkErr := m.enforce(); checkErr != nil {
				return errors.Join(err, checkErr)
			}
			return err
		case <-freeTicker.C:
			if err = m.pressure(nil); err != nil {
				return abort(err)
			}
		case <-ticker.C:
			if !scanning {
				scanning = true
				go func() { scanned <- m.enforce() }()
			}
		case err = <-scanned:
			scanning = false
			if err != nil {
				return abort(err)
			}
		}
	}
}

func storageJSON(v any) error { return json.NewEncoder(os.Stdout).Encode(v) }

func hasStorageLease(dir string) bool {
	info, err := os.Lstat(cacheLockPath(dir))
	return err == nil && info.Mode().IsRegular() && storageOwned(cacheLockPath(dir), info)
}

// Published toolchains are immutable and revision-keyed. Mutable Go caches
// are sampled at most five seconds apart. Sessions are always scanned live.
// This amortizes full walks across warm CLI invocations; statfs is never memoized.
type storageSizeRecord struct {
	Bytes     int64
	Scanned   time.Time
	Signature string
}

func storageImmutableSignature(dir string) string {
	if strings.HasSuffix(filepath.Base(filepath.Dir(dir)), "-dev") {
		return ""
	}
	info, err := os.Stat(filepath.Join(dir, setupManifest))
	if err != nil {
		return ""
	}
	parent, err := os.Stat(dir)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d/%d/%d", info.ModTime().UnixNano(), info.Size(), parent.ModTime().UnixNano())
}
func (m *storageManager) saveSizes(records map[string]storageSizeRecord) {
	data, err := json.Marshal(records)
	if err != nil {
		return
	}
	parent := filepath.Dir(m.sizeCacheFile)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return
	}
	f, err := os.CreateTemp(parent, ".xgo-usage-*")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return
	}
	if err = f.Close(); err != nil {
		return
	}
	_ = os.Rename(f.Name(), m.sizeCacheFile)
}

var storageResourceMu sync.Mutex

// Keep ownership metadata after leases disappear: a compiler may survive both
// its worker and the Go driver. Its process group still protects these paths.
func recordStorageResource(dir string) error {
	session := os.Getenv("XGO_STORAGE_SESSION")
	if session == "" || dir == session {
		return nil
	}
	storageResourceMu.Lock()
	defer storageResourceMu.Unlock()
	file := filepath.Join(session, ".resources.json")
	resources := map[string]bool{}
	data, err := os.ReadFile(file)
	if err == nil {
		if err = json.Unmarshal(data, &resources); err != nil {
			return err
		}
		if resources == nil {
			return fmt.Errorf("invalid session resource manifest")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if resources[absolute] {
		return nil
	}
	resources[absolute] = true
	data, err = json.Marshal(resources)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(session, ".resources-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), file)
}

func resourceInLiveSession(dir, root string) bool {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "run-") {
			continue
		}
		session := filepath.Join(root, entry.Name())
		if !sessionGroupAlive(session) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(session, ".resources.json"))
		// No resource can be used before its lease and manifest are recorded.
		if os.IsNotExist(err) {
			continue
		}
		// A live job with unreadable ownership is conservatively protected.
		if err != nil {
			return true
		}
		resources := map[string]bool{}
		if json.Unmarshal(data, &resources) != nil || resources == nil {
			return true
		}
		if resources[absolute] {
			return true
		}
	}
	return false
}

func cacheInLiveSession(dir string) bool {
	base := os.Getenv("XGO_STORAGE_ROOT")
	if base == "" {
		tmp, err := getStableTmpDir()
		if err != nil {
			return true
		}
		base = filepath.Join(tmp, "xgo")
	}
	return resourceInLiveSession(dir, filepath.Join(base, fmt.Sprintf("sessions-%d", os.Getuid())))
}

func sessionLeaseAlive(dir string) bool {
	lease, err := lockCacheForGC(dir)
	if err != nil {
		return true
	}
	lease.close()
	return false
}

// Unpublish under the exclusive lease before recursive deletion. An interrupted
// or permission-denied removal must not leave a "complete" half-toolchain.
func removeStorageDir(dir string) bool {
	tomb := dir
	if !strings.HasPrefix(filepath.Base(dir), gcTombPrefix) {
		tomb = filepath.Join(filepath.Dir(dir), gcTombPrefix+filepath.Base(dir))
		if err := os.Rename(dir, tomb); err != nil {
			return false
		}
	}
	return os.RemoveAll(tomb) == nil
}

// Stable environment paths preserve Go's test-input hashes (notably t.TempDir).
// Individual Go work directories remain random within this pool. A shared
// lease protects every live job; reclamation needs all leases and groups gone.
func (m *storageManager) prepareTempPool() (string, *cacheLock, error) {
	var dir string
	for _, root := range m.roots {
		if root.temporary && !root.sessions {
			dir = filepath.Join(root.dir, "pool-v1")
			break
		}
	}
	if dir == "" {
		return "", nil, fmt.Errorf("missing managed temporary pool")
	}
	// Reap a prior abandoned pool only when exclusive ownership can be proven.
	m.removeIf(storageEntry{dir: dir, temporary: true}, emptyTempPool)
	lease, err := lockCache(dir, false, true)
	if err != nil {
		return "", nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		lease.close()
		return "", nil, err
	}
	markTempPoolUsed(dir)
	return dir, lease, nil
}

// Preserve fresh explicit Go -work output and program-created temp files until
// retention/budget collection. Test cache hits don't depend on pool inode times.
func emptyTempPool(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() != gcMarkFile {
			return false
		}
	}
	return true
}

func markTempPoolUsed(dir string) {
	now := time.Now()
	markUsed(dir, now)
	// Retention can be shorter than markUsed's hourly cache-mark throttle.
	_ = os.Chtimes(filepath.Join(dir, gcMarkFile), now, now)
}

type storageGroupRecord struct {
	PID      int    `json:"pid"`
	Identity string `json:"identity,omitempty"`
}

func recordStorageGroup(dir string, pid int) (string, error) {
	record := storageGroupRecord{PID: pid}
	var err error
	if storageProcessGroups {
		record.Identity, err = storageProcessIdentity(pid)
		if err != nil {
			return "", err
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".group-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return record.Identity, os.Rename(f.Name(), filepath.Join(dir, ".process-group"))
}
