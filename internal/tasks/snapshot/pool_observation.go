package snapshot

import "github.com/Beamfall/corvint/internal/tasks/wire"

const MaxPoolObservationBytes = 4096

type PoolObservation struct {
	AllocationID, DefinitionSha256  wire.Digest
	Kind, Revision, Tree, Class     string
	Passed, GroupClean              bool
	OutputSha256, EnvironmentSha256 wire.Digest
}

func (p PoolObservation) Encode() []byte {
	return wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-pool-observation/0")).Set("allocationId", wire.String(string(p.AllocationID))).Set("definitionSha256", wire.String(string(p.DefinitionSha256))).Set("kind", wire.String(p.Kind)).Set("revision", wire.String(p.Revision)).Set("tree", wire.String(p.Tree)).Set("class", wire.String(p.Class)).Set("passed", wire.Bool(p.Passed)).Set("groupClean", wire.Bool(p.GroupClean)).Set("outputSha256", wire.String(string(p.OutputSha256))).Set("environmentSha256", wire.String(string(p.EnvironmentSha256)))))
}
func DecodePoolObservation(raw []byte) (*PoolObservation, error) {
	if len(raw) > MaxPoolObservationBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "pool observation", "byte bound")
	}
	v, e := wire.Parse(raw)
	if e != nil {
		return nil, e
	}
	r := wire.NewReader(v, "pool observation")
	if e := r.Profile("taskman-pool-observation/0"); e != nil {
		return nil, e
	}
	r.Closed("profile", "allocationId", "definitionSha256", "kind", "revision", "tree", "class", "passed", "groupClean", "outputSha256", "environmentSha256")
	r.Field("profile").Exact("taskman-pool-observation/0")
	p := &PoolObservation{AllocationID: r.Field("allocationId").Digest(), DefinitionSha256: r.Field("definitionSha256").Digest(), Kind: r.Field("kind").Enum("health", "cleanup"), Revision: r.Field("revision").OID(), Tree: r.Field("tree").OID(), Class: r.Field("class").Enum("EXIT_ZERO", "EXIT_NONZERO", "SPAWN_FAILED", "TIMEOUT", "INTERRUPTED", "OUTPUT_LIMIT", "UNKNOWN", "SOURCE_CHANGED"), Passed: r.Field("passed").Bool(), GroupClean: r.Field("groupClean").Bool(), OutputSha256: r.Field("outputSha256").Digest(), EnvironmentSha256: r.Field("environmentSha256").Digest()}
	if p.Passed && (p.Class != "EXIT_ZERO" || !p.GroupClean) {
		r.Fail(wire.CodeMalformed, "passing observation lacks clean normal exit")
	}
	return p, r.Err()
}
