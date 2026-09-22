package analyzerrust

import (
	"encoding/json"
	"strings"
)

// declaration records one manifest-derived scalar and the handle that declared
// it, so a second declaration of the same key can be compared rather than
// silently overwritten.
type declaration struct {
	handle string
	value  string
}

// factCollector charges every prospective output byte before a fact is
// retained, and holds the manifest declarations that crossCheck compares.
type factCollector struct {
	request      Request
	facts        []Fact
	outputSize   int
	digests      map[string]string
	declarations map[string]declaration
	seen         map[string]bool
}

// newFactCollector charges the complete empty success envelope by encoding it,
// so the running total is the real canonical byte count rather than a
// hand-maintained arithmetic model of it.
func newFactCollector(r Request) (*factCollector, string) {
	base, err := json.Marshal(success{Profile, Family, r.RequestID, "CANDIDATE",
		r.ScopeID, r.CompilationUnitID, r.Target, echoes(r), []Fact{}})
	if err != nil || len(base)+1 > MaxOutputBytes {
		return nil, "OUTPUT_LIMIT"
	}
	digests := make(map[string]string, len(r.Inputs))
	for _, in := range r.Inputs {
		digests[in.Handle] = in.SHA256
	}
	return &factCollector{
		request:      r,
		outputSize:   len(base) + 1,
		digests:      digests,
		declarations: map[string]declaration{},
		seen:         map[string]bool{},
	}, ""
}

// add charges, validates, and retains one fact. Every fact this candidate emits
// has exactly one input witness, so related_handle is always "-". The charge
// covers the fact's complete encoded bytes, including its evidence digest and
// the comma that joins it to the previous fact, before it is retained.
//
// A fact is one observation, so the same observation made twice in one input —
// two #[derive] attributes in a file, two cfg-gated items — is recorded once
// rather than emitted twice. This is what keeps the output strictly increasing
// at the source; the post-sort duplicate check in analyzeBound remains as the
// terminal invariant behind it.
func (c *factCollector) add(handle, kind, subject, predicate, value, instance string) string {
	fact := Fact{Kind: kind, InputHandle: handle, RelatedHandle: "-", Subject: subject,
		Predicate: predicate, Value: value, InstanceID: instance}
	key := kind + "\x00" + handle + "\x00" + subject + "\x00" + predicate + "\x00" + value + "\x00" + instance
	if c.seen[key] {
		return ""
	}
	if len(c.facts) >= MaxFacts {
		return "LIMIT_EXCEEDED"
	}
	if !factField(fact.Kind) || !factField(fact.InputHandle) || !factField(fact.RelatedHandle) ||
		!factField(fact.Subject) || !factField(fact.Predicate) || !factField(fact.Value) ||
		!factField(fact.InstanceID) {
		return "MALFORMED_INPUT"
	}
	fact.EvidenceSHA256 = evidence(c.request, c.digests[handle], fact)
	encoded, err := json.Marshal(fact)
	if err != nil {
		return "ANALYZER_FAILURE"
	}
	additional := len(encoded)
	if len(c.facts) > 0 {
		additional++
	}
	if c.outputSize > MaxOutputBytes-additional {
		return "OUTPUT_LIMIT"
	}
	c.outputSize += additional
	c.seen[key] = true
	c.facts = append(c.facts, fact)
	return ""
}

// declare records a manifest scalar under a logical key. A repeat of the same
// key with a different value is a conflict across the whole request, not a
// last-writer-wins overwrite.
func (c *factCollector) declare(key, handle, value string) string {
	if previous, seen := c.declarations[key]; seen && previous.value != value {
		return "CONFLICTING_VALUE"
	}
	c.declarations[key] = declaration{handle: handle, value: value}
	return ""
}

// crossCheck runs after every input has parsed. It compares declarations that
// only become comparable once the whole request is known.
func (c *factCollector) crossCheck(Request) string {
	manifest, hasManifest := c.declarations["package.rust-version"]
	toolchain, hasToolchain := c.declarations["toolchain.channel"]
	if hasManifest && hasToolchain && !toolchainSatisfies(toolchain.value, manifest.value) {
		return "CONFLICTING_VALUE"
	}
	return ""
}

// toolchainSatisfies compares a pinned toolchain channel with a declared
// minimum rust-version. Only an exact pinned three-component channel is
// comparable; a named channel is not a version and never conflicts.
func toolchainSatisfies(channel, rustVersion string) bool {
	if !exactVersion(channel) {
		return true
	}
	return compareVersions(channel, rustVersion) >= 0
}

// compareVersions orders two exact X.Y.Z versions numerically per component.
func compareVersions(left, right string) int {
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	for i := 0; i < 3; i++ {
		l, r := atoiBounded(leftParts[i]), atoiBounded(rightParts[i])
		if l != r {
			if l < r {
				return -1
			}
			return 1
		}
	}
	return 0
}

func atoiBounded(value string) int {
	n := 0
	for i := 0; i < len(value); i++ {
		n = n*10 + int(value[i]-'0')
	}
	return n
}

// factField is the common bound on every emitted fact string: printable ASCII,
// nonempty, and within the shared field ceiling.
func factField(value string) bool {
	if value == "" || len(value) > MaxFactFieldBytes {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7e {
			return false
		}
	}
	return true
}
