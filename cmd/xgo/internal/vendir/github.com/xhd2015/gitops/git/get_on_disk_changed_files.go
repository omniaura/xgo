package git

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

type onDiskChangedFileOption interface {
	applyOnDiskChangedFileOptions(*onDiskChangedFileOptionsConfig)
}

type onDiskChangedFileOptionsConfig struct {
	compareWith        string
	resolvePathsToFiles bool
}

type CompareWithOption struct {
	Commit string
}

func (c CompareWithOption) applyOnDiskChangedFileOptions(o *onDiskChangedFileOptionsConfig) {
	o.compareWith = c.Commit
}

func CompareWith(commit string) CompareWithOption {
	return CompareWithOption{Commit: commit}
}

type ResolvePathsToFilesOption struct{}

func (ResolvePathsToFilesOption) applyOnDiskChangedFileOptions(o *onDiskChangedFileOptionsConfig) {
	o.resolvePathsToFiles = true
}

func ResolvePathsToFiles() ResolvePathsToFilesOption {
	return ResolvePathsToFilesOption{}
}

func GetOnDiskChangedFiles(dir string, opts ...onDiskChangedFileOption) ([]string, error) {
	var cfg onDiskChangedFileOptionsConfig
	for _, opt := range opts {
		opt.applyOnDiskChangedFileOptions(&cfg)
	}

	var result []string
	var err error

	if cfg.compareWith != "" {
		result, err = getChangedFilesAgainst(dir, cfg.compareWith)
	} else {
		result, err = getStatusFiles(dir)
	}
	if err != nil {
		return nil, err
	}

	if cfg.resolvePathsToFiles && len(result) > 0 {
		result, err = expandDirsToFiles(dir, result)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func getStatusFiles(dir string) ([]string, error) {
	output, err := cmd.Dir(dir).Output("git", "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	rawLines := strings.Split(output, "\n")
	seen := make(map[string]bool, len(rawLines))
	var result []string
	for _, line := range rawLines {
		if len(line) < 4 {
			continue
		}
		xy := line[0:2]
		pathPart := line[3:]

		if xy[0] == 'D' || xy[1] == 'D' {
			continue
		}

		if xy[0] == 'R' {
			idx := strings.LastIndex(pathPart, " -> ")
			if idx >= 0 {
				pathPart = pathPart[idx+4:]
			}
		}

		pathPart = strings.TrimSpace(pathPart)
		if pathPart == "" {
			continue
		}
		if seen[pathPart] {
			continue
		}
		seen[pathPart] = true
		result = append(result, pathPart)
	}
	if result == nil {
		return nil, nil
	}
	return result, nil
}

func expandDirsToFiles(dir string, paths []string) ([]string, error) {
	seen := make(map[string]bool, len(paths))
	var result []string
	for _, p := range paths {
		abs := filepath.Join(dir, p)
		info, err := os.Lstat(abs)
		if err != nil {
			continue
		}
		if info.IsDir() {
			output, err := cmd.Dir(dir).Output("git", "ls-files", "--others", "--exclude-standard", "--full-name", p)
			if err != nil {
				return nil, err
			}
			for _, f := range splitLinesFilterEmpty(output) {
				if seen[f] {
					continue
				}
				seen[f] = true
				result = append(result, f)
			}
		} else {
			if seen[p] {
				continue
			}
			seen[p] = true
			result = append(result, p)
		}
	}
	if result == nil {
		return nil, nil
	}
	return result, nil
}

func getChangedFilesAgainst(dir string, compareCommit string) ([]string, error) {
	fileDetails, err := DiffCommitFiles(dir, COMMIT_WORKING, compareCommit, nil)
	if err != nil {
		return nil, err
	}
	var result []string
	for file, detail := range fileDetails {
		if detail.Deleted || detail.Unchanged() {
			continue
		}
		result = append(result, file)
	}
	return result, nil
}
