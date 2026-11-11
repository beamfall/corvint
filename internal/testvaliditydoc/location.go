package testvaliditydoc

import (
	"path/filepath"

	"github.com/Beamfall/corvint/internal/testvaliditymodel"
)

// Location is testvaliditymodel.Location, kept under its old name.
type Location = testvaliditymodel.Location

// LocationNotRecorded is testvaliditymodel.LocationNotRecorded, kept under its old name.
const LocationNotRecorded = testvaliditymodel.LocationNotRecorded

// LocationRootUnbound is testvaliditymodel.LocationRootUnbound, kept under its old name.
const LocationRootUnbound = testvaliditymodel.LocationRootUnbound

// LocationFileUnbound is testvaliditymodel.LocationFileUnbound, kept under its old name.
const LocationFileUnbound = testvaliditymodel.LocationFileUnbound

// LocationNotDeclaration is testvaliditymodel.LocationNotDeclaration, kept under its old name.
const LocationNotDeclaration = testvaliditymodel.LocationNotDeclaration

// LocationSourceNotRead is testvaliditymodel.LocationSourceNotRead, kept under its old name.
const LocationSourceNotRead = testvaliditymodel.LocationSourceNotRead

// LocationSourceUnplaced is testvaliditymodel.LocationSourceUnplaced, kept under its old name.
const LocationSourceUnplaced = testvaliditymodel.LocationSourceUnplaced

// LocationTitleNotLiteral is testvaliditymodel.LocationTitleNotLiteral, kept under its old name.
const LocationTitleNotLiteral = testvaliditymodel.LocationTitleNotLiteral

// LocationSharedExample is testvaliditymodel.LocationSharedExample, kept under its old name.
const LocationSharedExample = testvaliditymodel.LocationSharedExample

// LocationLineAmbiguous is testvaliditymodel.LocationLineAmbiguous, kept under its old name.
const LocationLineAmbiguous = testvaliditymodel.LocationLineAmbiguous

// LocationDeclarationAmbiguous is testvaliditymodel.LocationDeclarationAmbiguous, kept under its old name.
const LocationDeclarationAmbiguous = testvaliditymodel.LocationDeclarationAmbiguous

// Locate binds the repository-relative file of each JS/TS test whose anchor is its declaration
// (LPCV-V0-145): the one retained test file the anchor names, relative to root. root is resolved
// as discovery resolves it; an unresolvable root leaves repository-root-unbound. Other providers'
// locations need no root and are returned unchanged.
func Locate(document Document, input Input, root string) Document {
	if input.JS == nil || root == "" {
		return document
	}
	roots := []string{}
	if absolute, err := filepath.Abs(root); err == nil {
		roots = append(roots, absolute)
		if resolved, err := filepath.EvalSymlinks(absolute); err == nil && resolved != absolute {
			roots = append(roots, resolved)
		}
	}
	if len(roots) == 0 {
		return document
	}
	tests := append([]Test{}, document.Tests...)
	for index, outcome := range input.JS.Tests {
		if index >= len(tests) || tests[index].Location == nil || !testvaliditymodel.JSDeclarationAnchor(*input.JS, outcome) {
			continue
		}
		located := *tests[index].Location
		located.Absent = copyAbsent(located.Absent)
		located.Absent[testvaliditymodel.LocationFile] = LocationFileUnbound
		for _, candidate := range roots {
			if key, ok := boundTestFile(input.JS.Identity.TestFileDigests, candidate, outcome.Anchor.File); ok {
				relative, _ := filepath.Rel(candidate, key)
				located.SetFile(filepath.ToSlash(relative))
				break
			}
		}
		tests[index].Location = &located
	}
	document.Tests = tests
	return document
}

func copyAbsent(absent map[string]string) map[string]string {
	out := make(map[string]string, len(absent))
	for key, reason := range absent {
		out[key] = reason
	}
	return out
}
