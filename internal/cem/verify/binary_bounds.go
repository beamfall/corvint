package verify

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

// BinaryBlobSizes reads, in one bounded header batch, the size of every
// object a binary group names, and refuses repository-object-unavailable
// naming the path when either side exceeds gitauth.MaxBlobBytes
// (CEM-V1-985). Binary bodies are never read, so without this check an
// over-bound target blob would be bound silently. Prepare, `cem verify` and
// `verify-stable` all run it; a patch without binary groups reads nothing.
func BinaryBlobSizes(ctx context.Context, repository *gitauth.Repository, parsed *patch.Patch) (map[string]int, error) {
	if len(parsed.Binary) == 0 {
		return map[string]int{}, nil
	}
	var oids []string
	seen := map[string]bool{}
	for _, group := range parsed.Binary {
		for _, oid := range []string{group.OldOID, group.NewOID} {
			if oid != "" && !seen[oid] {
				seen[oid] = true
				oids = append(oids, oid)
			}
		}
	}
	sizes, err := repository.BlobHeaders(ctx, oids)
	if err != nil {
		return nil, err
	}
	for _, group := range parsed.Binary {
		for _, oid := range []string{group.OldOID, group.NewOID} {
			if oid != "" && sizes[oid] > gitauth.MaxBlobBytes {
				path := group.NewPath
				if path == nil {
					path = group.OldPath
				}
				return nil, cemcode.New(cemcode.RepositoryObjectUnavailable,
					"binary file %q blob %s is %d bytes, over the %d-byte blob bound (CEM-V1-985)",
					*path, oid, sizes[oid], gitauth.MaxBlobBytes)
			}
		}
	}
	return sizes, nil
}

// parseBinaryBounded parses the canonical patch and refuses an over-bound
// binary blob before base simulation (CEM-V1-985), for `cem verify` and the
// candidate check.
func parseBinaryBounded(ctx context.Context, repository *gitauth.Repository, patchBytes []byte) (*patch.Patch, error) {
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, err
	}
	if _, err := BinaryBlobSizes(ctx, repository, parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
