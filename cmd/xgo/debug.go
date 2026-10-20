package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func setupDevDir(goroot string, printDir bool) error {
	cmdCompile := filepath.Join(goroot, "src", "cmd", "compile")
	vscodeDir := filepath.Join(cmdCompile, ".vscode")

	err := assertDir(cmdCompile)
	if err != nil {
		return fmt.Errorf("setup $GOROOT/src/cmd/compile: %w", err)
	}
	err = os.MkdirAll(vscodeDir, 0755)
	if err != nil {
		return err
	}
	if printDir {
		fmt.Printf("%s\n", cmdCompile)
	}
	// {
	// 	"go.goroot": "$GOROOT",
	// 	"gopls": {
	// 		"build.env": {
	// 			"GOROOT": "$GOROOT",
	// 			"PATH": "$GOROOT/bin:${env:PATH}"
	// 		}
	// 	}
	// }
	settingsFile := filepath.Join(vscodeDir, "settings.json")
	err = patchJSONPretty(settingsFile, func(settings *map[string]interface{}) error {
		if *settings == nil {
			*settings = make(map[string]interface{})
		}
		// NOTE: must use absolute path, not rel path
		// gorootRel := "../.."
		gorootRel := goroot
		(*settings)["go.goroot"] = gorootRel
		gopls, _ := (*settings)["gopls"].(map[string]interface{})
		if gopls == nil {
			gopls = make(map[string]interface{})
			(*settings)["gopls"] = gopls
		}
		buildEnv, _ := gopls["build.env"].(map[string]interface{})
		if buildEnv == nil {
			buildEnv = make(map[string]interface{})
			gopls["build.env"] = buildEnv
		}
		buildEnv["GOROOT"] = gorootRel
		buildEnv["PATH"] = fmt.Sprintf("%s/bin:${env:PATH}", gorootRel)
		return nil
	})

	if err != nil {
		return fmt.Errorf("updating %s: %w", settingsFile, err)
	}
	return nil
}
