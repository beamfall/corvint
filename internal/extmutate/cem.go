package extmutate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// joinCEM reads a cem/1.0 map whose base is base and whose patch digest is
// the canonical base..head patch, and records on each hunk the ID of the CEM
// hunk of the same path whose new-side range contains it (CCE-V0-238). A
// hunk no CEM hunk contains keeps an empty ID and says so; the join never
// guesses by nearness.
func joinCEM(ctx context.Context, root, mapPath, base, head string, hunks []Hunk) (*CEMMapPin, error) {
	raw, err := readMap(mapPath)
	if err != nil {
		return nil, refuse("discriminate-map-invalid", "%s: %v", mapPath, err)
	}
	stable, err := wire.ParseStable(raw)
	if err != nil {
		return nil, refuse("discriminate-map-invalid", "%s: %v", mapPath, err)
	}
	mechanics := wire.Map(stable.Change)
	if mechanics.BaseRevision != base {
		return nil, refuse("discriminate-map-mismatch", "the map base %s is not --base %s", mechanics.BaseRevision, base)
	}
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, refuse("discriminate-map-mismatch", "%v", err)
	}
	patch, err := repository.CanonicalDiff(ctx, base, head)
	if err != nil {
		return nil, refuse("discriminate-map-mismatch", "canonical patch: %v", err)
	}
	if sum := sha256.Sum256(patch); hex.EncodeToString(sum[:]) != mechanics.PatchSha256 {
		return nil, refuse("discriminate-map-mismatch", "the canonical patch from --base to --head does not match the map patch digest")
	}
	for i := range hunks {
		h := &hunks[i]
		h.CEMJoin = CEMUnjoined
		start, end := int64(h.NewStart), int64(h.NewStart+h.NewCount)
		for _, c := range mechanics.Hunks {
			if c.Path == h.Path && c.NewRange.Start <= start && end <= c.NewRange.Start+c.NewRange.Count {
				h.CEMHunk, h.CEMJoin = c.ID, CEMJoined
				break
			}
		}
	}
	sum := sha256.Sum256(raw)
	return &CEMMapPin{Sha256: hex.EncodeToString(sum[:]), BaseRevision: mechanics.BaseRevision, PatchSha256: mechanics.PatchSha256}, nil
}

func readMap(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, wire.MaxMapBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > wire.MaxMapBytes {
		return nil, errors.New("exceeds the map bound")
	}
	return raw, nil
}
