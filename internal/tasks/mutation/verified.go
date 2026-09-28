package mutation

import (
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CompleteVerified is the §7.3 reducer's pass on an OPEN ticket: completion
// kind VERIFIED naming the manifest, with the gate result digests as its
// evidence. It is not an envelope operation, so no role row applies; the
// caller has already reduced the manifest. acceptanceRevision is unchanged,
// because only COMPLETE_MANUAL forces the bump (§3.1).
func CompleteVerified(ctx Context, pre *ticket.Record, evidence []wire.Digest, manifest wire.Digest) (*ticket.Record, error) {
	work, err := clone(pre)
	if err != nil {
		return nil, err
	}
	if work.Status != ticket.StatusOpen {
		return nil, wire.Errorf(wire.CodeTicketState, "/status", "VERIFIED completion requires status OPEN (is %s)", work.Status)
	}
	m := manifest
	work.Completion = &ticket.Completion{Kind: "VERIFIED", Actor: ctx.Binding.ID, Evidence: append([]wire.Digest{}, evidence...), ManifestSha256: &m, RecordedAt: ctx.Now}
	work.Status = ticket.StatusCompleted
	if r := ctx.finalize(pre, work, false); r != nil && r.code != "" {
		return nil, wire.Errorf(r.code, "/", "%s", r.detail)
	} else if r != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/", "%s", r.detail)
	}
	return work, nil
}
