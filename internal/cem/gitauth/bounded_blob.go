package gitauth

import (
	"context"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// BlobBytesBounded admits a typed object header before allocating its body.
// It never falls back to an unbounded one-shot read or ambient worktree bytes.
// Unlike BlobBytes, this source-reader path deliberately does not reuse the
// shared memo: the consumer byte bound applies before every body allocation.
func (r *Repository) BlobBytesBounded(ctx context.Context, oid string, limit int) ([]byte, error) {
	return r.boundedObject(ctx, oid, "blob", limit)
}

func (r *Repository) boundedObject(ctx context.Context, oid, kind string, limit int) ([]byte, error) {
	if !wire.IsGitOid(oid) || limit < 1 || limit > MaxBlobBytes {
		return nil, cemcode.New(cemcode.InvalidArguments, "bounded blob needs full lowercase OID and supported byte bound")
	}
	perOp, err := r.budget.ReserveOperation(0)
	if err != nil {
		return nil, err
	}
	session := r.objects
	if session == nil {
		session = &gitrun.Session{}
		defer session.Close()
	}
	options := r.gitOptions(limit, nil)
	admit := func(_ string, fields []string, size int) bool {
		if len(fields) != 3 || fields[0] != oid || fields[1] != kind || size < 0 || size > limit {
			return false
		}
		return r.chargedOids[oid] || int64(size) <= MaxTotalBlobBytes-r.blobBytes
	}
	_, body, ok, err := session.Read(ctx, perOp, options, append(r.pinnedArgs(), "cat-file", "--batch"), oid, admit)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, unavailable("immutable source object header is unavailable or exceeds its admitted bound")
	}
	if err := requireObjectIdentity(ctx, kind, oid, body); err != nil {
		return nil, err
	}
	if !r.chargedOids[oid] {
		if r.chargedOids == nil {
			r.chargedOids = map[string]bool{}
		}
		r.chargedOids[oid] = true
		r.blobBytes += int64(len(body))
	}
	return body, nil
}
