package testvaliditymodel

import (
	"path"

	"github.com/Beamfall/corvint/internal/rubytestreceipt"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// InclusionSite reports the including spec and scoped id path of an example whose receipt id
// names another spec file than the one declaring it: a shared example run through an inclusion.
func InclusionSite(document rubytestreceipt.Document, example rubytestreceipt.Example) (string, []int, bool) {
	file, ids, ok := testvalidity.ParseRSpecScopedID(example.RSpecID)
	if !ok {
		return "", nil, false
	}
	spec := path.Join(document.ProjectDirectory, path.Clean(file))
	return spec, ids, spec != example.File
}
