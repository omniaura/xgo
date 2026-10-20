package exec_tool

import (
	"github.com/omniaura/xgo/support/fileutil"
)

func patchJSONPretty(settingsFile string, fn func(conf *VscodeLaunchConfigMap) error) error {
	return fileutil.PatchJSONPretty(settingsFile, fn)
}
