package testvaliditydoc

import (
	"github.com/Beamfall/corvint/internal/testvaliditymodel"
)

// ProjectPinned is testvaliditymodel.ProjectPinned, kept under its old name.
func ProjectPinned(input Input, lookup func(string) ([]byte, bool)) Document {
	return testvaliditymodel.ProjectPinned(input, lookup)
}
