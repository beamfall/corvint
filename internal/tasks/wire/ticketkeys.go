package wire

// TicketRecordKeys is the closed taskman-ticket/0 record key set (SPEC §3.1),
// in record order. Corvint Core's read-only planner imports it instead of
// keeping a copy.
var TicketRecordKeys = []string{
	"profile", "ticketId", "revision", "acceptanceRevision", "previousRecordSha256", "status",
	"archivedFrom", "title", "body", "kind", "owner", "milestone", "priority", "order", "labels",
	"dependencies", "acceptanceCriteria", "requirementRefs", "source", "effects", "capabilities",
	"requiredGates", "holds", "executionClass", "approvals", "completion", "dueDate",
	"estimateMinutes", "supersedes", "supersededBy", "shadowOverlay", "createdAt", "updatedAt",
	"updatedBy",
}

// TicketRecordOptionalKeys are the taskman-ticket/0 record keys a record may
// omit. The native codec and Core's read-only planner both admit exactly
// these, so a new optional key cannot reach one reader and not the other.
var TicketRecordOptionalKeys = []string{"requiresPool", "requiredRoles", "escalations", "operatorNote", "externalReviews", "executionPrerequisites", "attachedEvidence", "knowHow"}

// EscalationMaxCurrentOpen bounds the OPEN questions of a ticket's current
// acceptance revision (ESC-V0-002). Stale OPEN questions do not count toward
// it. The native codec and Core's read-only planner both enforce it.
const EscalationMaxCurrentOpen = 16

// Attached-evidence bounds (TEA-V0-001): at most AttachedEvidenceMaxEntries
// entries per ticket, 1..AttachedEvidenceMaxDigests digests per entry, and a
// nonblank reason of at most AttachedEvidenceMaxReasonBytes. The native codec
// and Core's read-only planner both enforce them.
const (
	AttachedEvidenceMaxEntries     = 32
	AttachedEvidenceMaxDigests     = 16
	AttachedEvidenceMaxReasonBytes = 512
)

// Know-how note bounds (KHN-V0-002): at most KnowHowMaxEntries ledger entries
// per home ticket, nonblank text of at most KnowHowMaxTextBytes,
// 1..KnowHowMaxAnchors file anchors, at most KnowHowMaxRoutes route tokens of
// at most KnowHowMaxRouteBytes, and a reason of at most
// KnowHowMaxReasonBytes. The native codec and Core's read-only planner both
// enforce them.
const (
	KnowHowMaxEntries     = 32
	KnowHowMaxTextBytes   = 1024
	KnowHowMaxAnchors     = 4
	KnowHowMaxRoutes      = 4
	KnowHowMaxRouteBytes  = 64
	KnowHowMaxReasonBytes = 512
	KnowHowMaxSymbolBytes = 128
)

// ParseKnowHowSymbol validates a know-how symbol anchor name (KHN-V0-016):
// 1..KnowHowMaxSymbolBytes printable ASCII bytes other than space and '#',
// so `--symbol PATH#NAME` splits unambiguously at its last '#'. The native
// codec and Core's read-only planner both enforce it.
func ParseKnowHowSymbol(where, s string) (string, error) {
	if len(s) == 0 || len(s) > KnowHowMaxSymbolBytes {
		return "", Errorf(CodeMalformed, where, "symbol must be 1..%d bytes", KnowHowMaxSymbolBytes)
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c <= ' ' || c > '~' || c == '#' {
			return "", Errorf(CodeMalformed, where, "symbol %q must be printable ASCII without space or '#'", s)
		}
	}
	return s, nil
}

// KnowHowNotStale prefixes the refusal of a know-how RECONFIRM that would
// change no pin in a way that makes its note STALE (KHN-V0-019).
const KnowHowNotStale = "KNOWHOW_NOT_STALE"

// KnowHowPin is the freshness identity of one know-how anchor: its path, its
// symbol ("" for a file anchor) and the value freshness compares, which is
// the blob of a file anchor and the declaration digest of a symbol anchor.
type KnowHowPin struct{ Path, Symbol, Pin string }

// KnowHowReconfirmRefusal is "" when next re-pins exactly the anchors of
// prior (the same paths and symbols in the same order) and changes at least
// one Pin. Otherwise it is the reason a RECONFIRM is refused, prefixed
// KnowHowNotStale when no pin would change (KHN-V0-019). The native writer
// and codec and Core's read-only planner all apply this one rule.
func KnowHowReconfirmRefusal(prior, next []KnowHowPin) string {
	same := "a reconfirm re-pins exactly the note's anchors"
	if len(prior) != len(next) {
		return same
	}
	changed := false
	for i, a := range prior {
		if a.Path != next[i].Path || a.Symbol != next[i].Symbol {
			return same
		}
		changed = changed || a.Pin != next[i].Pin
	}
	if !changed {
		return KnowHowNotStale + ": no anchor's pin changed, so the note is not STALE at that commit"
	}
	return ""
}

// KnowHowRepositoryDetail prefixes the refusal of a repository-qualified
// know-how write or read whose repository is malformed, ambiguous or cannot be
// resolved (KHN-V0-024, KHN-V0-025, KHN-V0-027).
const KnowHowRepositoryDetail = "KNOWHOW_REPOSITORY"

// KnowHowMaxRepositoryBytes bounds a know-how repository alias.
const KnowHowMaxRepositoryBytes = 64

// ParseKnowHowRepository validates a know-how repository alias (KHN-V0-025):
// a token [A-Za-z0-9][A-Za-z0-9._-]* of at most KnowHowMaxRepositoryBytes, so
// it is always one path segment and never "." or "..".
func ParseKnowHowRepository(where, s string) (string, error) {
	if _, err := ParseToken(where, s, KnowHowMaxRepositoryBytes); err != nil {
		return "", Errorf(CodeMalformed, where, "%s: repository alias must be a token of at most %d bytes", KnowHowRepositoryDetail, KnowHowMaxRepositoryBytes)
	}
	return s, nil
}

// KnowHowRepositoryRefusal is "" when every anchor path of a note recorded in
// repository lies below "<repository>/" (KHN-V0-025), and otherwise the reason
// the note is refused. The native writer and codec and Core's read-only
// planner all apply this one rule.
func KnowHowRepositoryRefusal(repository string, paths []string) string {
	for _, p := range paths {
		if len(p) <= len(repository)+1 || p[:len(repository)+1] != repository+"/" {
			return KnowHowRepositoryDetail + ": every anchor path of a repository note starts with the repository alias and '/'"
		}
	}
	return ""
}
