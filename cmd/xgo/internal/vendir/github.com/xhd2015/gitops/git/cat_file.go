package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

func MustCatFile(dir string, ref string, file string) (content string, err error) {
	var ok bool
	ok, content, err = CatFile(dir, ref, file)
	if err != nil {
		return
	}
	if !ok {
		err = fmt.Errorf("%s does not exist in %s", file, ref)
		return
	}
	return
}
func CatFile(dir string, ref string, file string) (ok bool, content string, err error) {
	if ref == "" {
		err = fmt.Errorf("requires ref")
		return
	}
	if file == "" {
		err = fmt.Errorf("requires file")
		return
	}
	if err = checkCatFilePath(file); err != nil {
		return
	}
	if ref == COMMIT_WORKING {
		var full string
		full, err = confinedFile(dir, file)
		if err != nil {
			return
		}
		var info os.FileInfo
		info, fileErr := os.Lstat(full)
		if fileErr != nil {
			if os.IsNotExist(fileErr) {
				err = nil
				return
			}
			err = fileErr
			return
		}
		if info.Mode()&os.ModeSymlink != 0 {
			err = fmt.Errorf("symlink not allowed")
			return
		}
		contentBytes, fileErr := os.ReadFile(full)
		if fileErr != nil {
			if os.IsNotExist(fileErr) {
				err = nil
				return
			}
			err = fileErr
			return
		}
		ok = true
		content = string(contentBytes)
		return
	}
	var stderrBuf strings.Builder
	content, err = cmd.Dir(dir).Stderr(&stderrBuf).Output("git", "cat-file", "-p", ref+":"+file)
	stderr := stderrBuf.String()
	// example
	//    fatal: path 'go.mod2' does not exist in 'master'
	// example2:
	//    exists on disk, but not in
	ok = true
	if strings.Contains(stderr, "does not exist in") || !hasFile(dir, ref, file) {
		ok = false
		err = nil
		return
	}

	return
}

// see: https://stackoverflow.com/questions/18461761/git-check-whether-file-exists-in-some-version
func hasFile(dir string, ref string, file string) bool {
	err := cmd.Dir(dir).Run("git", "cat-file", "-e", ref+":"+file)
	return err == nil
}

func checkCatFilePath(file string) error {
	if filepath.IsAbs(file) {
		return fmt.Errorf("invalid file path")
	}
	cleaned := filepath.ToSlash(filepath.Clean(file))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("path escapes repo dir")
	}
	return nil
}

func confinedFile(dir, file string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("requires dir")
	}
	if err := checkCatFilePath(file); err != nil {
		return "", err
	}
	root, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return "", err
	}
	full := filepath.Clean(filepath.Join(root, file))
	sep := string(os.PathSeparator)
	if full != root && !strings.HasPrefix(full, root+sep) {
		return "", fmt.Errorf("path escapes repo dir")
	}
	return full, nil
}
