package doccorpus

// This file duplicates the public S0E repository-envelope packet materializer
// of internal/cem/cli/stable_read_test.go: test helpers are not shared across
// packages, and the packet bytes, not the helper, are the fixture.

// stableReadPacket is the public S0E repository-envelope packet the Core read
// tests materialize: one ordinary sha1 repository, its artifacts and its sealed
// positive cem/1.0 map.
const stableReadPacket = "../../protocol/cem-1.0/stable/repository-envelope-packet"

const stableReadAcceptCase = "normal-exit-lingering-descendant-contained"

// stableReadTupleRoot holds one sealed packet per structural tuple admitted
// after the frozen packet's release (decision 0522, CEM-V1-050).
const stableReadTupleRoot = "../../protocol/cem-1.0/structural-tuples"

type stableReadFixture struct {
	repo, artifacts      string
	raw                  []byte // the sealed positive cem/1.0 map, committed at the target as its sidecar
	expectedBase, target string
	acceptGo             string // the Go release whose public result is ACCEPT
}
