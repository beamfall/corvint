package workflow

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
)

// binaryRows renders each binary group of the prepared patch, in patch order,
// as an excluded row bound by its blob identity (CEM-V1-983). The sizes are
// Git's object headers for the bound object IDs; the absent side of a
// creation or deletion is null. An over-bound blob on either side refuses
// (CEM-V1-985). It returns nil when the patch has no binary group, so a
// text-only prepare result is unchanged.
func (s *Session) binaryRows(ctx context.Context, parsed *patch.Patch) ([]any, error) {
	if len(parsed.Binary) == 0 {
		return nil, nil
	}
	sizes, err := verify.BinaryBlobSizes(ctx, s.repository, parsed)
	if err != nil {
		return nil, err
	}
	side := func(oid string) (any, any) {
		if oid == "" {
			return nil, nil
		}
		return oid, sizes[oid]
	}
	rows := make([]any, 0, len(parsed.Binary))
	for _, group := range parsed.Binary {
		baseOID, baseSize := side(group.OldOID)
		targetOID, targetSize := side(group.NewOID)
		rows = append(rows, map[string]any{
			"path": binaryPath(group), "change": binaryChange(group), "mode": group.Mode,
			"baseBlobOid": baseOID, "targetBlobOid": targetOID,
			"baseSize": baseSize, "targetSize": targetSize,
			"disposition": "excluded",
		})
	}
	return rows, nil
}

func binaryPath(group *patch.Group) string {
	if group.NewPath != nil {
		return *group.NewPath
	}
	return *group.OldPath
}

func binaryChange(group *patch.Group) string {
	switch group.Kind {
	case patch.KindCreate:
		return "create"
	case patch.KindDelete:
		return "delete"
	}
	return "modify"
}

// binaryCiteRefusal refuses an evidence span in a path the patch changes as a
// binary group: binary content has no line classification, and an empty hunk
// list would otherwise read the span as stable (CEM-V1-984).
func binaryCiteRefusal(evidencePath string) error {
	return cemcode.New(cemcode.CiteSpanNotStable,
		"evidence span in %q cannot be classified: its CEM patch group is a binary change bound by blob identity, and binary content has no line classification (CEM-V1-984); cite a span in a text file",
		evidencePath)
}
