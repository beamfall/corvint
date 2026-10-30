package main

import (
	"path/filepath"

	"github.com/Beamfall/corvint/internal/worksource"
)

// workScratchRoot resolves the checked scratch parent (WQO-V0-094/095).
func workScratchRoot() (string, error) {
	parent, err := worksource.ScratchParent()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(parent)
}
