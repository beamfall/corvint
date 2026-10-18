package obligation

import (
	"encoding/json/v2"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Source-review finding kinds (TOL-V0-040). Preflight adds them only under
// the requireSourceReview policy.
const (
	FindingSourceMissing        = "SOURCE_REVIEW_MISSING"
	FindingSourceNoneUnsearched = "SOURCE_REVIEW_NONE_UNSEARCHED"
)

// Remedies of the source-review findings.
const (
	SourceMissingRemedy        = "record the legacy cases, legacy specs or manual cases reviewed for this obligation with ticket obligations set ... sources, or a none element with the query and the searched source ids"
	SourceNoneUnsearchedRemedy = "rerecord the none element with the query that was run and the non-empty list of source ids it searched"
)

// CheckReviewSources is the TOL-V0-040 source-review check of l's OPEN,
// DEFECT and BLOCKED obligations: each needs at least one review source,
// and every none element needs a non-blank query and a non-empty searched
// set, so a "none found" can be audited. WITNESSED and DEFERRED entries are
// not checked.
func CheckReviewSources(l *ticket.ObligationLedger) []Finding {
	var out []Finding
	for _, e := range l.Entries {
		if !ticket.ObligationPending(e.State) {
			continue
		}
		if len(e.Sources) == 0 {
			out = append(out, Finding{Kind: FindingSourceMissing, ID: e.ID,
				Detail: "obligation " + e.ID + " records no review sources", Remedy: SourceMissingRemedy})
			continue
		}
		for _, s := range e.Sources {
			if s.Kind == ticket.ObligationReviewNone && (blank(s.Query) || len(s.Searched) == 0) {
				out = append(out, Finding{Kind: FindingSourceNoneUnsearched, ID: e.ID,
					Detail: "obligation " + e.ID + " records a none review source without a query and a non-empty searched set", Remedy: SourceNoneUnsearchedRemedy})
				break
			}
		}
	}
	return out
}

// blank reports a string of Unicode whitespace only, the readNonblank rule.
func blank(s string) bool { return strings.TrimSpace(s) == "" }

// LegacyCasesSchema is the corvint-legacy-cases/1 record that `corvint docs
// corpus import-cases` emits (DCP-V1-070); MaxLegacyCasesBytes bounds it.
const (
	LegacyCasesSchema   = "corvint-legacy-cases/1"
	MaxLegacyCasesBytes = 32 << 20
)

var lowerHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ReadLegacyCaseDigests reads the case ID to case digest map of a
// corvint-legacy-cases/1 record (TOL-V0-043). It reads only schema,
// cases[].legacy.id and cases[].digest, and refuses a document that is not
// exactly one JSON value, a repeated object member, another schema, a
// missing or malformed digest, and a repeated case ID.
func ReadLegacyCaseDigests(raw []byte) (map[string]string, error) {
	if len(raw) > MaxLegacyCasesBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "--legacy-cases", "the legacy-cases record exceeds %d bytes", MaxLegacyCasesBytes)
	}
	var doc struct {
		Schema string `json:"schema"`
		Cases  []struct {
			Legacy struct {
				ID string `json:"id"`
			} `json:"legacy"`
			Digest string `json:"digest"`
		} `json:"cases"`
	}
	// json/v2 refuses duplicate member names and trailing data by default.
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "--legacy-cases", "the legacy-cases record is not one JSON document without repeated members")
	}
	if doc.Schema != LegacyCasesSchema {
		return nil, wire.Errorf(wire.CodeMalformed, "/schema", "the legacy-cases record is not %s", LegacyCasesSchema)
	}
	out := make(map[string]string, len(doc.Cases))
	for _, c := range doc.Cases {
		if c.Legacy.ID == "" || !lowerHex64.MatchString(c.Digest) {
			return nil, wire.Errorf(wire.CodeMalformed, "/cases", "a legacy case needs an id and a 64-hex digest")
		}
		if _, dup := out[c.Legacy.ID]; dup {
			return nil, wire.Errorf(wire.CodeMalformed, "/cases", "legacy case %s is repeated", c.Legacy.ID)
		}
		out[c.Legacy.ID] = c.Digest
	}
	return out, nil
}

// SourceReview is one review-source status (TOL-V0-042): a stale source
// whose current digest differs from the reviewed one, or an unresolved
// source whose current digest could not be read, with the reason.
type SourceReview struct {
	ID, State, Kind, Ref string
	Reviewed, Current    string
	Reason               string
}

// CheckSourceDigests compares every entry's legacy-case, legacy-spec and
// manual-case review sources with current (TOL-V0-042). current returns the
// source's current digest, or "" and the reason it is unknown. It reports
// and never changes state: a WITNESSED entry with a stale source stays
// WITNESSED. has is false when no entry records review sources.
func CheckSourceDigests(l *ticket.ObligationLedger, current func(kind, ref string) (string, string)) (stale, unresolved []SourceReview, has bool) {
	for _, e := range l.Entries {
		if e.Sources != nil {
			has = true
		}
		for _, s := range e.Sources {
			if s.Kind == ticket.ObligationReviewNone {
				continue
			}
			item := SourceReview{ID: e.ID, State: e.State, Kind: s.Kind, Ref: s.Ref, Reviewed: string(s.Digest)}
			cur, reason := current(s.Kind, s.Ref)
			switch {
			case cur == "":
				item.Reason = reason
				unresolved = append(unresolved, item)
			case cur != item.Reviewed:
				item.Current = cur
				stale = append(stale, item)
			}
		}
	}
	order := func(f []SourceReview) {
		sort.SliceStable(f, func(i, j int) bool {
			a, b := f[i], f[j]
			if a.ID != b.ID {
				return a.ID < b.ID
			}
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Ref < b.Ref
		})
	}
	order(stale)
	order(unresolved)
	return stale, unresolved, has
}
