package frontier

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/wp3codec"
)

// domainDigest is the frozen preimage shared by all three Frontier identities:
// the UTF-8 domain separator, one NUL byte, then the exact codec bytes
// (CF-V0-006, CF-V0-007, CF-V0-019). The NUL is what stops a longer domain
// from colliding with a shorter one plus a leading payload byte.
func domainDigest(domain string, body []byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(domain))
	hasher.Write([]byte{0x00})
	hasher.Write(body)
	return hex.EncodeToString(hasher.Sum(nil))
}

// universeValue builds the exact CF-V0-006 preimage object. Map, LRF, TCQ,
// command, observation, and report identities are deliberately absent: they
// are attempts to satisfy one stable obligation universe, so folding them in
// would give the same unresolved obligation a new identity every time evidence
// was repaired.
func universeValue(scope Scope) (wp3codec.Value, error) {
	start, err := wp3codec.Decimal(scope.IntentSpan.Start)
	if err != nil {
		return wp3codec.Value{}, err
	}
	end, err := wp3codec.Decimal(scope.IntentSpan.End)
	if err != nil {
		return wp3codec.Value{}, err
	}
	intent := wp3codec.Object(
		wp3codec.Member{Key: "blobOid", Value: wp3codec.String(scope.IntentBlobOID)},
		wp3codec.Member{Key: "path", Value: wp3codec.String(scope.IntentPath)},
		wp3codec.Member{Key: "span", Value: wp3codec.Object(
			wp3codec.Member{Key: "end", Value: end},
			wp3codec.Member{Key: "start", Value: start},
		)},
		wp3codec.Member{Key: "spanSha256", Value: wp3codec.String(scope.IntentSpanSHA256)},
	)
	return wp3codec.Object(
		wp3codec.Member{Key: "baseRevision", Value: wp3codec.String(scope.BaseRevision)},
		wp3codec.Member{Key: "excludedPath", Value: wp3codec.String(ExcludedPath)},
		wp3codec.Member{Key: "intent", Value: intent},
		wp3codec.Member{Key: "objectFormat", Value: wp3codec.String(scope.ObjectFormat)},
		wp3codec.Member{Key: "patchSha256", Value: wp3codec.String(scope.PatchSHA256)},
		wp3codec.Member{Key: "targetRevision", Value: wp3codec.String(scope.TargetRevision)},
	), nil
}

// UniverseID derives the CF-V0-006 identity. Identical Git content across
// clones intentionally yields an identical universe ID.
func UniverseID(scope Scope) (string, error) {
	value, err := universeValue(scope)
	if err != nil {
		return "", err
	}
	encoded, err := wp3codec.Encode(value)
	if err != nil {
		return "", err
	}
	return universeIDPrefix + domainDigest(universeDomain, encoded), nil
}

// ItemID derives the CF-V0-007 identity from exactly (kind, subjectId,
// universeId). Evidence repair may remove an item but cannot change the
// identity of an unresolved obligation in the same universe, which is what
// makes the queue diffable across runs.
func ItemID(kind, subjectID, universeID string) (string, error) {
	encoded, err := wp3codec.Encode(wp3codec.Object(
		wp3codec.Member{Key: "kind", Value: wp3codec.String(kind)},
		wp3codec.Member{Key: "subjectId", Value: wp3codec.String(subjectID)},
		wp3codec.Member{Key: "universeId", Value: wp3codec.String(universeID)},
	))
	if err != nil {
		return "", err
	}
	return itemIDPrefix + domainDigest(itemDomain, encoded), nil
}

// documentID derives the CF-V0-019 result identity over the document without
// its own `id` field, which is the only way a self-describing identity can be
// well defined.
func documentID(document Document) (string, error) {
	encoded, err := wp3codec.Encode(documentValue(document, false))
	if err != nil {
		return "", err
	}
	return documentIDPrefix + domainDigest(documentDomain, encoded), nil
}
