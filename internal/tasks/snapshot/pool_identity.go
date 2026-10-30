package snapshot

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfilePoolIdentity is the PSR-V0-083 identity observation profile.
const ProfilePoolIdentity = "taskman-pool-identity/0"

// MaxPoolIdentityBytes bounds one encoded identity observation.
const MaxPoolIdentityBytes = 2048

// PoolIdentityReported is the class of an identity command that exited zero
// and printed one well-formed source pair; every other class leaves the pair
// empty and never proves the member's source (PSR-V0-084).
const PoolIdentityReported = "REPORTED"

// PoolIdentityClasses are the closed identity observation classes.
var PoolIdentityClasses = []string{PoolIdentityReported, "EXIT_NONZERO", "SPAWN_FAILED", "TIMEOUT", "INTERRUPTED", "OUTPUT_LIMIT", "UNKNOWN", "SIGNAL", "SOURCE_CHANGED", "MALFORMED_OUTPUT"}

// PoolIdentity is one run of a member's identity command, bound to the
// allocation it was observed for (PSR-V0-083). ObservedAt is the runner's
// wall clock when the command ended; ObservedSeq is the receipt sequence that
// recorded it, "0" in the unrecorded observation a caller supplies.
type PoolIdentity struct {
	PoolID, MemberID               string
	AllocationID, DefinitionSha256 wire.Digest
	Class                          string
	Source                         *intent.PoolSource
	OutputSha256                   wire.Digest
	ObservedAt                     wire.Timestamp
	ObservedSeq                    wire.Size
}

func (p PoolIdentity) Value() wire.Value {
	o := wire.NewObject().Set("profile", wire.String(ProfilePoolIdentity)).Set("poolId", wire.String(p.PoolID)).Set("memberId", wire.String(p.MemberID)).Set("allocationId", wire.String(string(p.AllocationID))).Set("definitionSha256", wire.String(string(p.DefinitionSha256))).Set("class", wire.String(p.Class)).Set("source", PoolSourceValue(p.Source)).Set("outputSha256", wire.String(string(p.OutputSha256))).Set("observedAt", wire.String(string(p.ObservedAt))).Set("observedSeq", wire.String(string(p.ObservedSeq)))
	return wire.ObjectValue(o)
}

func (p PoolIdentity) Encode() []byte { return wire.EncodeFile(p.Value()) }

// PoolSourceValue is a source pair object, or null.
func PoolSourceValue(s *intent.PoolSource) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.ObjectValue(wire.NewObject().Set("revision", wire.String(s.Revision)).Set("tree", wire.String(s.Tree)))
}

func readPoolSourceOrNull(r *wire.Reader) *intent.PoolSource {
	if r.IsNull() {
		return nil
	}
	r.Closed("revision", "tree")
	return &intent.PoolSource{Revision: r.Field("revision").OID(), Tree: r.Field("tree").OID()}
}

func readPoolIdentity(r *wire.Reader) PoolIdentity {
	// CAL-V0-131: a newer identity record is UNSUPPORTED_VERSION; the nested
	// pools.json reader relies on the first recorded error sticking.
	if r.Profile(ProfilePoolIdentity) != nil {
		return PoolIdentity{}
	}
	r.Closed("profile", "poolId", "memberId", "allocationId", "definitionSha256", "class", "source", "outputSha256", "observedAt", "observedSeq")
	r.Field("profile").Exact(ProfilePoolIdentity)
	p := PoolIdentity{PoolID: r.Field("poolId").Label(), MemberID: r.Field("memberId").Label(), AllocationID: r.Field("allocationId").Digest(), DefinitionSha256: r.Field("definitionSha256").Digest(), Class: r.Field("class").Enum(PoolIdentityClasses...), Source: readPoolSourceOrNull(r.Field("source")), OutputSha256: r.Field("outputSha256").Digest(), ObservedAt: r.Field("observedAt").Timestamp(), ObservedSeq: r.Field("observedSeq").Size()}
	if r.Err() == nil && (p.Class == PoolIdentityReported) != (p.Source != nil) {
		r.Fail(wire.CodeMalformed, "an identity reports a source pair exactly when its class is REPORTED")
	}
	return p
}

// DecodePoolIdentity reads one canonical identity observation.
func DecodePoolIdentity(raw []byte) (*PoolIdentity, error) {
	if len(raw) > MaxPoolIdentityBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "pool identity", "byte bound")
	}
	v, e := wire.Parse(raw)
	if e != nil {
		return nil, e
	}
	r := wire.NewReader(v, "pool identity")
	p := readPoolIdentity(r)
	if r.Err() != nil {
		return nil, r.Err()
	}
	if string(p.Encode()) != string(raw) {
		return nil, wire.Errorf(wire.CodeMalformed, "pool identity", "non-canonical encoding")
	}
	return &p, nil
}
