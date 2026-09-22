// Package sim proves a parsed CEM patch against exact base-tree bytes, per
// interop/cem-0.1/ALGORITHMS.md "Patch/base simulation". It is a simulation
// only: the patch is never executed, and old bytes are never invented — every
// context and removed payload must match the base byte-for-byte.
package sim

import (
	"bytes"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

// BlobSource reads base-tree blobs through Git object identity.
type BlobSource interface {
	// BaseBlob returns the bounded blob bytes and tree mode at path in the
	// base tree and whether the path exists as a regular blob.
	BaseBlob(path string) (data []byte, mode string, exists bool, err error)
}

func mismatch(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.BaseMismatch, format, args...)
}

// Simulate verifies every file group of the patch against the base tree.
func Simulate(parsed *patch.Patch, source BlobSource) error {
	for _, group := range parsed.Groups {
		if err := simulateGroup(group, source); err != nil {
			return err
		}
	}
	return nil
}

func simulateGroup(group *patch.Group, source BlobSource) error {
	oldBytes, err := groupOldBytes(group, source)
	if err != nil {
		return err
	}
	if err := requireDestinationAbsent(group, source); err != nil {
		return err
	}
	newBytes, err := ApplyHunks(group, oldBytes)
	if err != nil {
		return err
	}
	if group.Kind == patch.KindDelete && len(newBytes) != 0 {
		return mismatch("delete of %q does not produce empty bytes", *group.OldPath)
	}
	return nil
}

func groupOldBytes(group *patch.Group, source BlobSource) ([]byte, error) {
	if group.Kind == patch.KindCreate {
		return nil, nil
	}
	data, mode, exists, err := source.BaseBlob(*group.OldPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, mismatch("old path %q is absent from the base tree", *group.OldPath)
	}
	if group.Kind == patch.KindDelete && mode != group.Mode { // decision 0192
		return nil, cemcode.New(cemcode.DiffMetadataMismatch,
			"deleted file mode %s disagrees with base mode %s of %q", group.Mode, mode, *group.OldPath)
	}
	return data, nil
}

func requireDestinationAbsent(group *patch.Group, source BlobSource) error {
	if group.Kind != patch.KindCreate && group.Kind != patch.KindRename {
		return nil
	}
	_, _, exists, err := source.BaseBlob(*group.NewPath)
	if err != nil {
		return err
	}
	if exists {
		return mismatch("destination %q already exists in the base tree", *group.NewPath)
	}
	return nil
}

// ApplyHunks copies untouched base lines, compares every context and removed
// payload byte-for-byte, appends context and added payloads, and verifies
// every old and new cursor.
func ApplyHunks(group *patch.Group, oldBytes []byte) ([]byte, error) {
	oldLines, err := patch.SplitLF(oldBytes)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	oldNext := int64(1)
	newEmitted := int64(0)
	for _, hunk := range group.Hunks {
		contextStart := hunk.OldRange.Start
		if hunk.OldRange.Count == 0 {
			contextStart = hunk.OldRange.Start + 1
			// An insertion consumes no line, so the range check below cannot
			// bound it: it must name a line the base file actually has.
			if hunk.OldRange.Start > int64(len(oldLines)) {
				return nil, mismatch("hunk insertion point is beyond the base file")
			}
		}
		if contextStart < oldNext {
			return nil, mismatch("hunk old ranges overlap")
		}
		if hunk.OldRange.Start+hunk.OldRange.Count-1 > int64(len(oldLines)) {
			return nil, mismatch("hunk old range exceeds the base file")
		}
		for ; oldNext < contextStart; oldNext++ {
			out.Write(oldLines[oldNext-1])
			newEmitted++
		}
		expectedNew := hunk.NewRange.Start
		if hunk.NewRange.Count > 0 {
			expectedNew--
		}
		if newEmitted != expectedNew {
			return nil, mismatch("hunk new cursor is inconsistent")
		}
		for _, line := range hunk.Body {
			if line.Prefix != '+' {
				if oldNext > int64(len(oldLines)) {
					return nil, mismatch("hunk consumes beyond the base file")
				}
				if !bytes.Equal(line.OldPayload(), oldLines[oldNext-1]) {
					return nil, mismatch("hunk %s payload disagrees with the base bytes", string(line.Prefix))
				}
				oldNext++
			}
			if line.Prefix != '-' {
				out.Write(line.NewPayload())
				newEmitted++
			}
		}
	}
	for ; oldNext <= int64(len(oldLines)); oldNext++ {
		out.Write(oldLines[oldNext-1])
	}
	return out.Bytes(), nil
}
