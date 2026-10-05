package transaction

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// --exclude-authors modes (CAL-V0-098): the ticket's most recent implement
// generation (owner decision D5), or every recorded implement generation.
const (
	ExcludeAuthorsLatest = "LATEST"
	ExcludeAuthorsAll    = "ALL"
)

// Author is one implement generation whose recorded pool member a review or
// integrate claim excludes. The label is a recorded member fact, not an
// authenticated identity (CAL-V0-098).
type Author struct {
	AttemptID        string
	Generation       wire.Size
	PoolID, MemberID string
}

// AuthorExclusion is what one CAL-V0-098 derivation found for a ticket:
// the implement-generation authors and the effective requested-pool
// exclusion set, the explicit members unioned with the authors' members in
// that pool (nil when empty).
type AuthorExclusion struct {
	Authors  []Author
	Excluded []string
}

// CheckExcludeAuthors validates only request facts, so replay does not depend
// on current state: a closed mode, an explicit pool, and a review or
// integrate stage.
func CheckExcludeAuthors(mode, pool, stage string) error {
	if mode == "" {
		return nil
	}
	if mode != ExcludeAuthorsLatest && mode != ExcludeAuthorsAll {
		return malformed("excludeAuthors is LATEST or ALL")
	}
	if pool == "" {
		return malformed("author exclusions require an explicit pool")
	}
	if stage != "review" && stage != "integrate" {
		return malformed("author exclusions require --stage review or integrate")
	}
	return nil
}

// endedGeneration is one generation of a ticket's attempt with its recorded
// history, nil when the stage and member are NOT_OBSERVED.
type endedGeneration struct {
	attemptID  string
	generation wire.Size
	history    *snapshot.GenerationHistory
}

// ticketGenerations lists every generation of the ticket's attempts, newest
// first. A prior generation carries its V1-0788 prior-generation history; an
// attempt's current generation is read as that history would record it once
// ended.
func ticketGenerations(attempts map[string]*snapshot.Attempt, ticketID string) []endedGeneration {
	out := []endedGeneration{}
	for _, a := range attempts {
		if a.TicketID.Raw != ticketID {
			continue
		}
		for _, p := range a.PriorGenerations {
			out = append(out, endedGeneration{attemptID: a.AttemptID, generation: p.Generation, history: p.History})
		}
		out = append(out, endedGeneration{attemptID: a.AttemptID, generation: a.Generation, history: endedHistory(a)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].generation.Uint64() > out[j].generation.Uint64() })
	return out
}

func (g endedGeneration) name() string {
	return "generation " + string(g.generation) + " of " + g.attemptID
}

// DeriveAuthors applies CAL-V0-098 to one ticket. Walking newest first, it
// skips recorded review and integrate generations; every other generation it
// reaches must be an implement generation with a recorded pool member, or
// the derivation is unverified and the detail says why. LATEST stops at the
// first implement generation; ALL reads them all. It never infers a member
// from receipts or any other history (no backfill of legacy member facts).
func DeriveAuthors(attempts map[string]*snapshot.Attempt, ticketID, mode, pool string, explicit []string) (*AuthorExclusion, string) {
	authors := []Author{}
	for _, g := range ticketGenerations(attempts, ticketID) {
		h := g.history
		if h == nil {
			return nil, g.name() + " records no stage or pool member (NOT_OBSERVED); its author cannot be excluded"
		}
		if h.Stage != nil && (*h.Stage == "review" || *h.Stage == "integrate") {
			continue
		}
		if h.Stage == nil {
			return nil, g.name() + " recorded no stage; it may have authored the ticket"
		}
		if h.MemberID == nil {
			return nil, "implement " + g.name() + " held no pool member"
		}
		authors = append(authors, Author{AttemptID: g.attemptID, Generation: g.generation, PoolID: *h.PoolID, MemberID: *h.MemberID})
		if mode == ExcludeAuthorsLatest {
			break
		}
	}
	if len(authors) == 0 {
		return nil, "no implement generation of " + ticketID + " is recorded"
	}
	set := map[string]bool{}
	for _, m := range explicit {
		set[m] = true
	}
	for _, a := range authors {
		if a.PoolID == pool {
			set[a.MemberID] = true
		}
	}
	var excluded []string
	for m := range set {
		excluded = append(excluded, m)
	}
	sort.Strings(excluded)
	return &AuthorExclusion{Authors: authors, Excluded: excluded}, ""
}

// AuthorsDetail names the excluded authors for a RESOURCE_COLLISION detail.
func (x *AuthorExclusion) AuthorsDetail() string {
	if x == nil {
		return ""
	}
	names := make([]string, 0, len(x.Authors))
	for _, a := range x.Authors {
		names = append(names, "member "+a.MemberID+" of pool "+a.PoolID+" (generation "+string(a.Generation)+")")
	}
	return "excluded implement authors: " + strings.Join(names, ", ")
}

// authorExclusion derives the claim's author exclusions for rec, or returns
// the INDEPENDENCE_UNVERIFIED refusal. It is nil without --exclude-authors.
func (c leaseContext) authorExclusion(ticketID string) (*AuthorExclusion, *leaseOutcome) {
	if c.l.ExcludeAuthors == "" {
		return nil, nil
	}
	x, why := DeriveAuthors(c.st.attempts, ticketID, c.l.ExcludeAuthors, c.l.Pool, c.l.ExcludeMembers)
	if x == nil {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeIndependenceUnverified, why)
		return nil, &out
	}
	return x, nil
}

// excluded is the effective member exclusion set of this claim: the
// explicit members, unioned with the derived authors when requested.
func (c leaseContext) excluded() []string {
	if c.authors != nil {
		return c.authors.Excluded
	}
	return c.l.ExcludeMembers
}

// authorsSuffix appends the excluded authors to a RESOURCE_COLLISION detail.
func (c leaseContext) authorsSuffix() string {
	if c.authors == nil {
		return ""
	}
	return "; " + c.authors.AuthorsDetail()
}
