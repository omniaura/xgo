package overlay

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// resolvedDirs caches filepath.EvalSymlinks of directories. Only results
// are cached, not errors: xgo creates directories while it runs.
var resolvedDirs sync.Map // string -> string

// evalSymlinks is filepath.EvalSymlinks with the directory of path resolved
// once: every overlay lookup canonicalizes its file, and resolving the
// whole path took one lstat per component, for each lookup of each file.
func evalSymlinks(path string) (string, error) {
	path = filepath.Clean(path)
	// on windows EvalSymlinks also normalizes the case of each component
	if runtime.GOOS == "windows" || !filepath.IsAbs(path) {
		return filepath.EvalSymlinks(path)
	}
	dir := filepath.Dir(path)
	if dir == path {
		return filepath.EvalSymlinks(path)
	}
	var resolvedDir string
	if v, ok := resolvedDirs.Load(dir); ok {
		resolvedDir = v.(string)
	} else {
		var err error
		resolvedDir, err = filepath.EvalSymlinks(dir)
		if err != nil {
			return "", err
		}
		resolvedDirs.Store(dir, resolvedDir)
	}
	file := filepath.Join(resolvedDir, filepath.Base(path))
	info, err := os.Lstat(file)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return filepath.EvalSymlinks(file)
	}
	return file, nil
}
