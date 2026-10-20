package loadcov

import (
	"fmt"

	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/load/git"
	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/model"
)

func LoadGitDiff(dir string, ref string, diffBase string, relFiles []string) (*model.ProjectAnnotation, error) {
	if diffBase == "" {
		return nil, fmt.Errorf("requires diffBase")
	}

	return git.LoadFiles(dir, ref, diffBase, relFiles)
}
