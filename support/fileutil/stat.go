package fileutil

import (
	"errors"
	"os"
)

func IsFile(file string) (bool, error) {
	return FileExists(file)
}

func FileExists(file string) (bool, error) {
	stat, err := os.Stat(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !stat.IsDir(), nil
}
