package tcq

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Frozen profile and vocabulary constants (TCQ-V0-023, TCQ-V0-031, TCQ-V0-036).
const (
	Profile          = "tcq/0"
	CommandSpec      = "test-command/0.1-experimental"
	ObservationSpec  = "test-observation/0.1-experimental"
	ReportFormat     = "junit-xml/corvint-v0"
	AuthorityClass   = "CALLER_REPORTED"
	MatchedRelation  = "test-report-matched-v0"
	OCMSpec          = "ocm/0.1-experimental"
	CEMCanonicalSpec = "cem/0.2"
	ClaimExtractor   = "corvint-test-claim/1"

	commandPrefix     = "test-command:sha256:"
	observationPrefix = "test-observation:sha256:"
	rowPrefix         = "test-row:sha256:"
	unitPrefix        = "test-unit:sha256:"
	tcqPrefix         = "tcq:sha256:"
)

// Domain separation strings. Each identity hashes its own domain so no preimage
// of one kind can ever be replayed as another (TCQ-V0-021/022/025/030/040).
const (
	domainExecutionKey = "corvint-test-execution-key/0"
	domainUnit         = "corvint-test-unit/0"
	domainCommand      = "corvint-test-command/0.1-experimental"
	domainObservation  = "corvint-test-observation/0.1-experimental"
	domainRow          = "corvint-junit-row/0"
	domainTCQ          = "corvint-tcq/0"
)

// canonicalValue is the shared `canonical-json-value` primitive: the frozen
// sorted-key, minimal, terminal-LF-free encoding every TCQ content address
// hashes over (TCQ-V0-040). It delegates to the one workspace implementation.
func canonicalValue(value wire.Value) []byte { return wire.CanonicalValue(value) }

// canonicalJSON is a complete artifact encoding: canonical value plus the one
// terminal LF that TCQ-V0-040 requires of a whole document.
func canonicalJSON(value wire.Value) []byte { return append(canonicalValue(value), '\n') }

// domainHash is SHA-256(UTF8(domain) || 0x00 || body), hex-encoded.
func domainHash(domain string, body []byte) string {
	digest := sha256.New()
	digest.Write([]byte(domain))
	digest.Write([]byte{0})
	digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// Builders keep document assembly declarative so the wire shapes in the spec
// read the same way here as they do in docs/specs/test-claim-qualification-v0.md.

func jsonNull() wire.Value              { return wire.Value{Kind: wire.KindNull} }
func jsonString(text string) wire.Value { return wire.Value{Kind: wire.KindString, Str: text} }
func jsonInt(value int64) wire.Value    { return wire.Value{Kind: wire.KindInt, Int: value} }
func jsonArray(items []wire.Value) wire.Value {
	if items == nil {
		items = []wire.Value{}
	}
	return wire.Value{Kind: wire.KindArray, Arr: items}
}

func jsonStrings(items []string) wire.Value {
	values := make([]wire.Value, 0, len(items))
	for _, item := range items {
		values = append(values, jsonString(item))
	}
	return jsonArray(values)
}

type member struct {
	key   string
	value wire.Value
}

func jsonObject(members ...member) wire.Value {
	object := &wire.Object{Keys: make([]string, 0, len(members)), Values: make(map[string]wire.Value, len(members))}
	for _, item := range members {
		object.Keys = append(object.Keys, item.key)
		object.Values[item.key] = item.value
	}
	return wire.Value{Kind: wire.KindObject, Obj: object}
}

// withoutMember returns a copy of an object minus one key. Command, observation,
// and TCQ identities all hash the document *without* its own `id` field.
func withoutMember(value wire.Value, drop string) wire.Value {
	object := &wire.Object{Values: make(map[string]wire.Value, len(value.Obj.Keys))}
	for _, key := range value.Obj.Keys {
		if key == drop {
			continue
		}
		object.Keys = append(object.Keys, key)
		object.Values[key] = value.Obj.Values[key]
	}
	return wire.Value{Kind: wire.KindObject, Obj: object}
}

// executionKey is the TCQ-V0-021 unit-derived execution identity. The same
// derivation keys JUnit rows (TCQ-V0-028), which is exactly why a row match
// states equality with a source-derived key and nothing more.
func executionKey(path, runtimeName string) string {
	preimage := jsonObject(
		member{"name", jsonString(runtimeName)},
		member{"path", jsonString(path)},
	)
	return domainHash(domainExecutionKey, canonicalValue(preimage))
}
