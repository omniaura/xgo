package main

import (
	"github.com/omniaura/xgo/support/fileutil"
)

func patchJSONPretty(settingsFile string, fn func(settings *map[string]interface{}) error) error {
	return fileutil.PatchJSONPretty(settingsFile, fn)
}
