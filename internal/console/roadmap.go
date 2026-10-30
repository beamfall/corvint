package console

import (
	"context"
	"math"
	"strconv"
)

// roadmapPageSize bounds one roadmap page. More than this many tickets pages
// rather than growing one page without limit.
const roadmapPageSize = 50

// roadmapRefreshSeconds periodically repeats the same read-only roadmap
// request. Eligibility is derived by Corvint Tasks, so a blocker disappears
// only after the owning tool stops reporting it.
const roadmapRefreshSeconds = 30

// RoadmapRow is one ticket as the roadmap shows it: the fields `atm roadmap`
// reports, plus the blocker closure for a ticket the tool reports BLOCKED.
// Gate evidence (RequiredGates, GateResults) is rendered as the tool stated
// it, never upgraded to a pass/fail the tool did not report (IPR-02).
type RoadmapRow struct {
	TicketID      string
	Title         string
	Milestone     string
	Order         string
	Owner         string
	Priority      string
	Status        string
	Kind          string
	Eligibility   string
	NextAction    string
	RequiredGates []string
	GateResults   string
	Blockers      *Envelope
	BlockerSource Source
}

// RoadmapGroup is one milestone's tickets, in the order `atm roadmap`
// returned them. A ticket the tool assigned no milestone still renders, in
// its own group, rather than being dropped.
type RoadmapGroup struct {
	Milestone string
	Unmapped  bool
	Rows      []RoadmapRow
}

// Roadmap is the whole roadmap page: the grouped tickets, the read's own
// attribution, and the pagination `atm roadmap` reported.
type Roadmap struct {
	Groups    []RoadmapGroup
	Envelope  *Envelope
	Source    Source
	Total     int
	Untrusted []string
	Err       string

	Page       int
	PageSize   int
	PrevPage   int
	NextPage   int
	HasPrev    bool
	HasNext    bool
	TotalKnown bool
	TotalCount int
}

// ReadRoadmap reads one page of `atm roadmap`, grouped by the milestone the
// tool assigned. A ticket the tool reports BLOCKED gets its own blocker read;
// a ticket that is not blocked names no blocker read, so one roadmap page
// costs a handful of process invocations, not one per row.
func (t *Taskman) ReadRoadmap(ctx context.Context, page int) *Roadmap {
	if page < 1 {
		page = 1
	}
	// A page whose offset would not fit an int is read as page 1, like any
	// other unusable page number, rather than wrapping into a negative or
	// unrelated offset shown under the requested page number.
	if page > math.MaxInt/roadmapPageSize {
		page = 1
	}
	offset := (page - 1) * roadmapPageSize
	envelope, source := t.Run(ctx, "roadmap", "--offset", strconv.Itoa(offset), "--limit", strconv.Itoa(roadmapPageSize))
	roadmap := &Roadmap{
		Source: source, Envelope: envelope,
		Page: page, PageSize: roadmapPageSize, PrevPage: page - 1, NextPage: page + 1, HasPrev: page > 1,
	}
	if envelope == nil {
		roadmap.Err = source.Err
		return roadmap
	}
	roadmap.Untrusted = envelope.Untrusted
	if envelope.Refused() {
		return roadmap
	}
	var order []string
	byMilestone := map[string][]RoadmapRow{}
	for _, item := range envelope.Items {
		row := roadmapRowOf(item)
		if row.Eligibility == "BLOCKED" {
			row.Blockers, row.BlockerSource = t.Run(ctx, "ticket", "blockers", row.TicketID)
		}
		if _, seen := byMilestone[row.Milestone]; !seen {
			order = append(order, row.Milestone)
		}
		byMilestone[row.Milestone] = append(byMilestone[row.Milestone], row)
		roadmap.Total++
	}
	for _, key := range order {
		name, unmapped := key, false
		if name == "" {
			name, unmapped = "Unassigned", true
		}
		roadmap.Groups = append(roadmap.Groups, RoadmapGroup{Milestone: name, Unmapped: unmapped, Rows: byMilestone[key]})
	}
	roadmap.applyPage(envelope.Page)
	return roadmap
}

// applyPage reads the tool's own page report so "next" is offered only when
// more rows are actually known to remain. A total the tool did not state
// stays unknown here too, rather than being guessed.
func (r *Roadmap) applyPage(page map[string]any) {
	total, ok := page["total"].(float64)
	if !ok {
		r.HasNext = r.Total == r.PageSize
		return
	}
	r.TotalKnown = true
	r.TotalCount = int(total)
	r.HasNext = (r.Page-1)*r.PageSize+r.Total < r.TotalCount
}

func roadmapRowOf(item map[string]any) RoadmapRow {
	return RoadmapRow{
		TicketID:      stringOf(item["ticketId"]),
		Title:         stringOf(item["title"]),
		Milestone:     stringOf(item["milestone"]),
		Order:         stringOf(item["order"]),
		Owner:         stringOf(item["owner"]),
		Priority:      stringOf(item["priority"]),
		Status:        stringOf(item["status"]),
		Kind:          stringOf(item["kind"]),
		Eligibility:   stringOf(item["eligibility"]),
		NextAction:    stringOf(item["nextAction"]),
		RequiredGates: stringsOf(item["requiredGates"]),
		GateResults:   stringOf(item["gateResults"]),
	}
}
