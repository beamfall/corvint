package obligation

// The reviewed-shortfall worklist (TOL-V0-086..088, GitHub #1182): a ranked,
// test-file-batched queue of obligations whose latest reviewed witness was
// HELD. It is pure: the caller folds the ledgers and supplies each entry's
// review time, test repository and staleness at HEAD.
import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Worklist classes and exclusion reasons.
const (
	WorklistItem      = "ITEM"
	WorklistExcluded  = "EXCLUDED"
	WorklistNeedsText = "NEEDS_OBLIGATION_TEXT"

	WorklistCandidateAwaitingReview = "CANDIDATE_AWAITING_REVIEW"
	WorklistBlocked                 = "BLOCKED"
	WorklistSubjectChanged          = "SUBJECT_CHANGED"
	WorklistSubjectUnverified       = "SUBJECT_UNVERIFIED"
	WorklistMirrorDecided           = "MIRROR_DECIDED_ELSEWHERE"

	WorklistRankPriority = "priority"
	WorklistRankNearest  = "nearest"
	WorklistRankAge      = "age"

	// MaxWorklistNext bounds --next.
	MaxWorklistNext = 100
)

// WorklistDefaultRank is the default ranking: priority, then nearest
// completion.
var WorklistDefaultRank = []string{WorklistRankPriority, WorklistRankNearest}

// WorklistInput is one obligation entry with the facts the fold does not
// carry: its ticket, the ticket's remaining pending core count, the
// reviewed witness's test repository and issue time, and whether the cited
// example changed at HEAD since review (Stale) or could not be read there
// (Unverified).
type WorklistInput struct {
	TicketID       string
	Priority       string
	Remaining      int64
	Entry          ticket.ObligationEntry
	TestRepository *string
	ReviewedAt     wire.Timestamp
	Stale          bool
	Unverified     bool
	// MirrorDecided marks a mirror entry decided on its owner ticket
	// (TOL-V0-122).
	MirrorDecided bool
}

// ClassifyWorklist places in: "" when it is not a reviewed shortfall (no
// HELD review, or WITNESSED or DEFERRED); NEEDS_OBLIGATION_TEXT when a
// reviewer found no falsifiable text; EXCLUDED with a reason for a mirror
// decided on its owner ticket (TOL-V0-122), a pending
// candidate, a BLOCKED entry, or a subject changed or unreadable at HEAD;
// otherwise ITEM.
func ClassifyWorklist(in WorklistInput) (class, reason string) {
	e := in.Entry
	if e.Review == nil || e.Review.Outcome != ticket.ObligationReviewHeld || !BoundaryPending(e) {
		return "", ""
	}
	if in.MirrorDecided {
		return WorklistExcluded, WorklistMirrorDecided
	}
	for _, v := range e.Review.Review.Verdicts {
		if v.Finding == ticket.VerdictFindingNoFalsifiableText {
			return WorklistNeedsText, ticket.VerdictFindingNoFalsifiableText
		}
	}
	switch {
	case e.Candidate != nil:
		return WorklistExcluded, WorklistCandidateAwaitingReview
	case e.State == ticket.ObligationBlocked:
		return WorklistExcluded, WorklistBlocked
	case in.Unverified:
		return WorklistExcluded, WorklistSubjectUnverified
	case in.Stale:
		return WorklistExcluded, WorklistSubjectChanged
	}
	return WorklistItem, ""
}

// WorklistVerdict is "partial" when any reviewer judged the witness
// partial, else "held".
func WorklistVerdict(e ticket.ObligationEntry) string {
	for _, v := range e.Review.Review.Verdicts {
		if v.Verdict == ticket.VerdictPartial {
			return ticket.VerdictPartial
		}
	}
	return "held"
}

// ParseWorklistRank reads a comma-separated ranking of priority, nearest
// and age, each at most once.
func ParseWorklistRank(s string) ([]string, error) {
	if s == "" {
		return WorklistDefaultRank, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range strings.Split(s, ",") {
		if k != WorklistRankPriority && k != WorklistRankNearest && k != WorklistRankAge {
			return nil, wire.Errorf(wire.CodeMalformed, "--rank", "unknown rank key %q: use priority, nearest or age", k)
		}
		if seen[k] {
			return nil, wire.Errorf(wire.CodeMalformed, "--rank", "rank key %s is repeated", k)
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

func worklistPriorityIndex(p string) int {
	for i, q := range ticket.Priorities {
		if q == p {
			return i
		}
	}
	return len(ticket.Priorities)
}

// RankWorklist orders items by keys in turn, then ticket id and obligation
// id, so the order is total and stable.
func RankWorklist(items []WorklistInput, keys []string) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		for _, k := range keys {
			switch k {
			case WorklistRankPriority:
				if x, y := worklistPriorityIndex(a.Priority), worklistPriorityIndex(b.Priority); x != y {
					return x < y
				}
			case WorklistRankNearest:
				if a.Remaining != b.Remaining {
					return a.Remaining < b.Remaining
				}
			case WorklistRankAge:
				if a.ReviewedAt != b.ReviewedAt {
					return a.ReviewedAt < b.ReviewedAt
				}
			}
		}
		if a.TicketID != b.TicketID {
			return a.TicketID < b.TicketID
		}
		return a.Entry.ID < b.Entry.ID
	})
}

// WorklistBatchKey is the test file an item cites: its test repository
// ("" for the store's own) and path.
func WorklistBatchKey(in WorklistInput) string {
	return BoundaryRepoKey(in.TestRepository) + "\x00" + in.Entry.Review.Review.Subject.Path
}

// WorklistNext returns the head of ranked items plus up to k later items
// citing the same test file, in rank order.
func WorklistNext(ranked []WorklistInput, k int) []WorklistInput {
	if len(ranked) == 0 {
		return nil
	}
	out := []WorklistInput{ranked[0]}
	key := WorklistBatchKey(ranked[0])
	for _, in := range ranked[1:] {
		if len(out) > k {
			break
		}
		if WorklistBatchKey(in) == key {
			out = append(out, in)
		}
	}
	return out
}

// WorklistAgeDays is the whole days from at to now, or -1 when at does not
// parse.
func WorklistAgeDays(at wire.Timestamp, now time.Time) int64 {
	t, err := time.Parse(time.RFC3339Nano, string(at))
	if err != nil {
		return -1
	}
	d := now.Sub(t)
	if d < 0 {
		return 0
	}
	return int64(d / (24 * time.Hour))
}

// WorklistItemValue renders one ranked item.
func WorklistItemValue(in WorklistInput, now time.Time) wire.Value {
	e := in.Entry
	rv := e.Review
	var missing, risks []wire.Value
	for _, v := range rv.Review.Verdicts {
		if v.StubRisk != ticket.StubRiskNone {
			risks = append(risks, wire.String(fmt.Sprintf("%s: %s", v.Reviewer, v.StubRisk)))
		}
		if v.Verdict != ticket.VerdictCorrect {
			missing = append(missing, wire.ObjectValue(wire.NewObject().Set("reviewer", wire.String(v.Reviewer)).Set("verdict", wire.String(v.Verdict)).Set("reason", wire.String(v.Reason))))
		}
	}
	age := wire.Null()
	if d := WorklistAgeDays(in.ReviewedAt, now); d >= 0 {
		age = wire.String(string(wire.CountOf(d)))
	}
	s := rv.Review.Subject
	return wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(in.TicketID)).Set("priority", wire.String(in.Priority)).
		Set("remainingCore", wire.String(string(wire.CountOf(in.Remaining)))).Set("id", wire.String(e.ID)).Set("title", wire.String(e.Title)).
		Set("verdict", wire.String(WorklistVerdict(e))).Set("missing", wire.Array(missing...)).Set("stubRisks", wire.Array(risks...)).
		Set("testRepository", wire.StringOrNull(in.TestRepository)).Set("path", wire.String(s.Path)).
		Set("startLine", wire.String(string(wire.CountOf(s.StartLine)))).Set("endLine", wire.String(string(wire.CountOf(s.EndLine)))).
		Set("reviewedAt", wire.String(string(in.ReviewedAt))).Set("reviewAgeDays", age))
}

// WorklistNeedsTextValue renders one needs-obligation-text listing.
func WorklistNeedsTextValue(in WorklistInput) wire.Value {
	var reviewers []wire.Value
	for _, v := range in.Entry.Review.Review.Verdicts {
		if v.Finding == ticket.VerdictFindingNoFalsifiableText {
			reviewers = append(reviewers, wire.ObjectValue(wire.NewObject().Set("reviewer", wire.String(v.Reviewer)).Set("reason", wire.String(v.Reason))))
		}
	}
	return wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(in.TicketID)).Set("id", wire.String(in.Entry.ID)).
		Set("title", wire.String(in.Entry.Title)).Set("findings", wire.Array(reviewers...)))
}
