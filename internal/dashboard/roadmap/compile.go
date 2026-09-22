package roadmap

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

// atmTicketSummary is the subset of one `atm roadmap` item this join reads.
type atmTicketSummary struct {
	TicketID    string `json:"ticketId"`
	Title       string `json:"title"`
	Milestone   string `json:"milestone"`
	Status      string `json:"status"`
	Eligibility string `json:"eligibility"`
	Owner       string `json:"owner"`
	Priority    string `json:"priority"`
}

// atmTicketDetail is the subset of one `atm ticket show` item this join
// reads: blockers and the record's requirementRefs.
type atmTicketDetail struct {
	Blockers []Blocker `json:"blockers"`
	Record   struct {
		RequirementRefs []string `json:"requirementRefs"`
	} `json:"record"`
}

// Compile joins `atm roadmap`, `atm ticket show` per ticket,
// docs/specs/REQUIREMENTS.tsv, the configured test-receipts directory, and
// the configured docs-state file into one roadmap Snapshot, grouped by
// milestone in `atm roadmap`'s own order. It only ever runs the `roadmap`
// and `ticket show` read verbs; it never mutates the planning store.
//
// A refused or unreachable `atm roadmap` call produces a whole-snapshot
// NOT_OBSERVED outcome with a reason — never an empty ticket list reported
// as success. A per-ticket `ticket show` failure does not abort the join;
// that ticket instead carries a synthetic NOT_OBSERVED blocker explaining
// what could not be observed, so one bad ticket cannot hide every other
// ticket's real evidence.
func Compile(ctx context.Context, options Options) (*Snapshot, []byte, error) {
	snapshot := &Snapshot{Schema: Schema, GeneratedAt: options.GeneratedAt}

	roadmapEnvelope, err := runAtm(ctx, options.AtmBinary, options.StoreRoot, options.Timeout, "roadmap")
	if err != nil {
		snapshot.Outcome = notObserved
		snapshot.Reason = "atm roadmap: " + err.Error()
		return finish(snapshot)
	}
	if roadmapEnvelope.Outcome != "OK" {
		snapshot.Outcome = notObserved
		snapshot.Reason = "atm roadmap refused: outcome " + roadmapEnvelope.Outcome
		return finish(snapshot)
	}

	summaries := make([]atmTicketSummary, 0, len(roadmapEnvelope.Items))
	for _, raw := range roadmapEnvelope.Items {
		var summary atmTicketSummary
		if err := json.Unmarshal(raw, &summary); err != nil {
			snapshot.Outcome = notObserved
			snapshot.Reason = "atm roadmap item did not parse: " + err.Error()
			return finish(snapshot)
		}
		summaries = append(summaries, summary)
	}

	// An unreadable table stays nil: buildTicket then marks every ref with
	// requirementsNotObserved instead of reporting it absent from a table
	// that was never observed (AGENTS.md invariant 2).
	requirements, _ := LoadRequirements(options.RequirementsTSV)

	// A dirty worktree means options.TreeDigest (the last committed tree)
	// no longer reflects everything on disk, so no receipt can be honestly
	// classified CURRENT or STALE against it (AGENTS.md invariant 2). A
	// boundary failure checking that is itself missing evidence and is
	// treated the same as dirty: WorktreeDirty already returns true
	// alongside its error for that reason.
	worktreeDirty, _ := WorktreeDirty(ctx, options.RepoRoot, options.Timeout)
	snapshot.WorktreeDirty = worktreeDirty

	milestoneIndex := map[string]int{}
	for _, summary := range summaries {
		ticket := buildTicket(ctx, options, summary, requirements, worktreeDirty)
		index, ok := milestoneIndex[ticket.Milestone]
		if !ok {
			index = len(snapshot.Milestones)
			milestoneIndex[ticket.Milestone] = index
			snapshot.Milestones = append(snapshot.Milestones, MilestoneGroup{Milestone: ticket.Milestone})
		}
		snapshot.Milestones[index].Tickets = append(snapshot.Milestones[index].Tickets, ticket)
	}
	for index := range snapshot.Milestones {
		tickets := snapshot.Milestones[index].Tickets
		sort.SliceStable(tickets, func(a, b int) bool { return tickets[a].LocalID < tickets[b].LocalID })
	}
	snapshot.Outcome = "OK"
	return finish(snapshot)
}

func buildTicket(ctx context.Context, options Options, summary atmTicketSummary, requirements map[string]Requirement, worktreeDirty bool) Ticket {
	local := localID(summary.TicketID)
	base := Ticket{
		TicketID: summary.TicketID, LocalID: local, Title: summary.Title,
		Milestone: summary.Milestone, Status: summary.Status, Eligibility: summary.Eligibility,
		Owner: summary.Owner, Priority: summary.Priority,
		Evidence: []EvidenceLink{}, TestReceipts: []TestReceipt{},
	}

	detailEnvelope, err := runAtm(ctx, options.AtmBinary, options.StoreRoot, options.Timeout, "ticket", "show", summary.TicketID)
	if err == nil && detailEnvelope.Outcome != "OK" {
		err = errors.New("ticket show refused: outcome " + detailEnvelope.Outcome)
	}
	if err == nil && len(detailEnvelope.Items) != 1 {
		err = errors.New("ticket show returned an unexpected item count")
	}
	var detail atmTicketDetail
	if err == nil {
		err = json.Unmarshal(detailEnvelope.Items[0], &detail)
	}
	if err != nil {
		base.Blockers = []Blocker{{Code: notObserved, Detail: "atm ticket show " + summary.TicketID + ": " + err.Error()}}
		base.DocState = DocState{State: notObserved, Reason: "ticket detail not observed"}
		return base
	}

	base.Blockers = append([]Blocker(nil), detail.Blockers...)
	if base.Blockers == nil {
		base.Blockers = []Blocker{}
	}
	for _, ref := range detail.Record.RequirementRefs {
		link := ResolveRequirement(requirements, ref)
		if requirements == nil {
			link.Reason = requirementsNotObserved
		}
		base.Evidence = append(base.Evidence, link)
		base.TestReceipts = append(base.TestReceipts, LoadReceipt(options.ReceiptsDir, ref, options.TreeDigest, worktreeDirty))
	}
	base.DocState = LoadDocState(options.DocsStatePath, local)
	return base
}

// localID strips the "ticket:corvint:planning:" style prefix `atm` reports,
// returning the trailing local id (e.g. "IPR-10").
func localID(ticketID string) string {
	for index := len(ticketID) - 1; index >= 0; index-- {
		if ticketID[index] == ':' {
			return ticketID[index+1:]
		}
	}
	return ticketID
}

func finish(snapshot *Snapshot) (*Snapshot, []byte, error) {
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	encoded = append(encoded, '\n')
	return snapshot, encoded, nil
}
