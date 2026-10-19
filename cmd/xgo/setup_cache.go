package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const setupManifest = ".xgo-complete.json"

type setupEntry struct {
	Path string
	Size int64
	Dir  bool
}

type setupRecord struct {
	Revision string
	Entries  []setupEntry
}

func instrumentRevisionKey(revision string, filePatches bool) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:file-patches=%t", revision, filePatches))))[:16]
}

// The final marker inventories the tree, not just the root directory: tmp
// cleaners and interrupted manual cleanup can leave a plausible but broken
// GOROOT. Validate each entry before handing it to go list or the compiler.
func completeInstrumentDir(dir, revision string) bool {
	data, err := os.ReadFile(filepath.Join(dir, setupManifest))
	if err != nil {
		return false
	}
	var record setupRecord
	if json.Unmarshal(data, &record) != nil || record.Revision != revision || len(record.Entries) == 0 {
		return false
	}
	// Read directory entries in batches instead of issuing one stat syscall
	// per source file on every warm invocation. Tool binaries get size checks
	// too; the source inventory detects missing packages/files/directories.
	expected := make(map[string]bool, len(record.Entries))
	for _, entry := range record.Entries {
		expected[entry.Path] = entry.Dir
		if !entry.Dir && (strings.Contains(filepath.ToSlash(entry.Path), "/bin/") || strings.Contains(filepath.ToSlash(entry.Path), "/pkg/tool/")) {
			info, err := os.Stat(filepath.Join(dir, entry.Path))
			if err != nil || info.Size() != entry.Size {
				return false
			}
		}
	}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if wantDir, ok := expected[rel]; ok {
			if d.IsDir() != wantDir {
				return fmt.Errorf("incomplete setup: %s", rel)
			}
			delete(expected, rel)
		} else if rel != "." && d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	return err == nil && len(expected) == 0
}

func writeSetupManifest(dir, revision string) error {
	record := setupRecord{Revision: revision}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == setupManifest || rel == gcMarkFile {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		record.Entries = append(record.Entries, setupEntry{Path: rel, Size: info.Size(), Dir: info.IsDir()})
		return nil
	})
	if err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, setupManifest), data, 0644)
}

// Return a shared lease held until the child Go command exits. Repair/reset
// waits for all users, builds privately, and publishes only a complete tree.
func prepareInstrumentDir(dir, revision string, reset bool, build func(string) error) (*cacheLock, error) {
	lease, err := lockCache(dir, false, true)
	if err != nil {
		return nil, err
	}
	if !reset && completeInstrumentDir(dir, revision) {
		return lease, nil
	}
	lease.close()
	lease, err = lockCache(dir, true, true)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			lease.close()
		}
	}()
	if reset || !completeInstrumentDir(dir, revision) {
		stage, err := os.MkdirTemp(filepath.Dir(dir), ".xgo-setup-*")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(stage)
		if err := build(stage); err != nil {
			return nil, err
		}
		if err := writeSetupManifest(stage, revision); err != nil {
			return nil, err
		}
		if err := os.RemoveAll(dir); err != nil {
			return nil, err
		}
		if err := os.Rename(stage, dir); err != nil {
			return nil, err
		}
	}
	// Reacquire and validate under a shared lease. flock conversion is not
	// atomic on every Unix; another reset/GC may win the intervening gap.
	lease.close()
	ok = true
	return prepareInstrumentDir(dir, revision, false, build)
}
