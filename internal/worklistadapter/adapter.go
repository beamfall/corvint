// Package worklistadapter is the repository-work-queue-adapter/0 producer for a
// committed JSON worklist. Corvint's self-dogfood adapter and `corvint work
// adapter` both run it; every authority it emits comes from the committed policy.
package worklistadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/workqueue"
	"github.com/Beamfall/corvint/internal/worksource"
)

const (
	PolicyPath      = ".corvint/work-queue-policy.json"
	WorklistProfile = "corvint-worklist/0"
	// RepositoryMapping is the mappingVersion of the adoptable repository worklist.
	RepositoryMapping = "repository-worklist-v0"
)

type mapping struct{ worklist, route string }

// mappings is the closed set of policy mappingVersion values this producer
// implements: where the committed worklist lives and which route its tickets need.
var mappings = map[string]mapping{
	"decision-0046-v0": {worklist: "docs/worklist.json", route: "codex"},
	RepositoryMapping:  {worklist: ".corvint/worklist.json", route: "agent"},
}

// WorklistPath returns the committed worklist path a mapping version reads.
func WorklistPath(mappingVersion string) (string, bool) {
	selected, known := mappings[mappingVersion]
	return selected.worklist, known
}

type Worklist struct {
	Profile string `json:"profile"`
	Tickets []Item `json:"tickets"`
}

type Item struct {
	Body       string   `json:"body"`
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	TouchPaths []string `json:"touchPaths"`
}

// Run executes one fixed operation against the repository containing directory
// and writes the canonical document to stdout.
func Run(ctx context.Context, program, directory string, arguments []string, stdout io.Writer) error {
	if len(arguments) != 1 {
		return fmt.Errorf("usage: %s snapshot|details|verify", program)
	}
	switch arguments[0] {
	case "snapshot", "details", "verify":
	default:
		return errors.New("unsupported operation")
	}
	storage, err := OwnedScratch()
	if err != nil {
		return err
	}
	root, err := worksource.ResolveRootWithScratch(ctx, directory, storage.Parent)
	if err != nil {
		return err
	}
	if storage.Parent != "" && storage.Root != root {
		return errors.New("source scratch root mismatch")
	}
	qualified, err := worksource.AcquireWithScratch(ctx, root, storage.Parent)
	if err != nil {
		return err
	}
	defer qualified.Close()
	if storage.Parent != "" && storage.Commit != qualified.Identity.Commit {
		return errors.New("source scratch commit mismatch")
	}
	policyRaw, err := SourceFile(qualified, PolicyPath)
	if err != nil {
		return err
	}
	policy, err := workqueue.ParsePolicy(policyRaw)
	if err != nil {
		return err
	}
	snapshot, details, checkpoint, err := DocumentsFromSource(qualified, policy)
	if err != nil {
		return err
	}
	var output []byte
	switch arguments[0] {
	case "snapshot":
		output = snapshot.Canonical()
	case "details":
		output = details.Canonical()
	case "verify":
		output = checkpoint.Canonical()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err = stdout.Write(output)
	return err
}

// Documents qualifies root and derives the three adapter documents from it.
func Documents(root string, policy *workqueue.Policy) (*workqueue.Snapshot, *workqueue.DetailsDocument, *workqueue.CheckpointDocument, error) {
	qualified, err := worksource.Acquire(context.Background(), root)
	if err != nil {
		return nil, nil, nil, err
	}
	defer qualified.Close()
	return DocumentsFromSource(qualified, policy)
}

func DocumentsFromSource(qualified *worksource.Source, policy *workqueue.Policy) (*workqueue.Snapshot, *workqueue.DetailsDocument, *workqueue.CheckpointDocument, error) {
	source := qualified.Identity
	selected, known := mappings[policy.MappingVersion]
	if !known {
		return nil, nil, nil, fmt.Errorf("unsupported mapping version: %s", policy.MappingVersion)
	}
	repositoryAuthority := policy.RepositoryAuthorityID
	queueAuthority := policy.QueueAuthorityID
	namespace := strings.TrimPrefix(queueAuthority, "queue:")
	capacityClass := "capacity:" + namespace + ":agent"
	route := workqueue.RouteAlternative{ID: "route:" + namespace + ":" + selected.route, Requires: []string{"capability:" + namespace + ":" + selected.route}}
	raw, err := SourceFile(qualified, selected.worklist)
	if err != nil {
		return nil, nil, nil, err
	}
	var list Worklist
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&list); err != nil || list.Profile != WorklistProfile {
		return nil, nil, nil, errors.New("invalid worklist")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, nil, nil, errors.New("invalid worklist")
	}
	tickets := make([]workqueue.TicketSummary, 0, len(list.Tickets))
	details := make([]workqueue.Detail, 0, len(list.Tickets))
	limit, err := workqueue.ParseCount(policy.DetailLimit)
	if err != nil {
		return nil, nil, nil, err
	}
	for index, item := range list.Tickets {
		itemRaw, _ := json.Marshal(item)
		payload := workqueue.DetailPayload{AcceptanceCriteria: []string{}, Body: &item.Body, EvidenceHandles: []string{}, Title: &item.Title}
		detailDigest := workqueue.DetailPayloadDigest(payload)
		ticket := workqueue.TicketSummary{
			AtomicRepositoryAuthorityIDs: []string{repositoryAuthority}, Authority: "COMPLETE",
			CapacityUses: []workqueue.CapacityUse{{ClassID: capacityClass, Units: 1}}, CollisionGroupIDs: []string{},
			DeclaredVersion: workqueue.SHA256Hex(itemRaw)[:16], DependencyTicketIDs: []string{}, DetailPayloadSHA256: &detailDigest,
			Lifecycle: "READY", QueueAuthorityID: queueAuthority, Rank: workqueue.Rank(index), RepositoryAuthorityID: repositoryAuthority,
			RouteAlternatives:   []workqueue.RouteAlternative{route},
			SelectionFacts:      workqueue.SelectionFacts{Approvals: "CLEAR", Dependencies: "SATISFIED", Holds: "CLEAR", Lease: "ABSENT"},
			TicketContentSHA256: workqueue.SHA256Hex(itemRaw), TicketID: "ticket:" + namespace + ":" + item.ID,
			TouchPaths: append([]string(nil), item.TouchPaths...),
		}
		sort.Strings(ticket.TouchPaths)
		workqueue.RefreshTicket(&ticket)
		tickets = append(tickets, ticket)
		if workqueue.Count(index) >= limit {
			continue
		}
		detail := workqueue.Detail{Payload: payload, RepositoryAuthorityID: repositoryAuthority, TicketID: ticket.TicketID, TicketVersionID: ticket.TicketVersionID}
		workqueue.RefreshDetail(&detail)
		details = append(details, detail)
	}
	snapshot := &workqueue.Snapshot{
		AccessContextID: policy.AccessContextID, CapacityClasses: []workqueue.CapacityClass{{AvailableUnits: 4, ID: capacityClass}},
		Checkpoint: workqueue.Checkpoint{ID: "checkpoint:" + namespace, Version: source.Tree}, Leases: []workqueue.LeaseSummary{},
		PolicyID: policy.ID, Profile: workqueue.SnapshotProfile, QueueAuthorityID: queueAuthority, RepositoryAuthorityID: repositoryAuthority,
		RepositorySource: source, Scope: workqueue.Scope{Complete: true, ID: policy.ScopeID, TicketCount: workqueue.Count(len(tickets))}, Tickets: tickets,
	}
	for index := range tickets {
		if workqueue.Count(index) >= limit {
			break
		}
		snapshot.DetailRequestTicketVersionIDs = append(snapshot.DetailRequestTicketVersionIDs, tickets[index].TicketVersionID)
	}
	sort.Strings(snapshot.DetailRequestTicketVersionIDs)
	workqueue.RefreshSnapshot(snapshot)
	detailDocument := &workqueue.DetailsDocument{Details: details, SnapshotID: snapshot.ID}
	workqueue.RefreshDetails(detailDocument)
	checkpoint := &workqueue.CheckpointDocument{Checkpoint: snapshot.Checkpoint, PolicyID: policy.ID, RepositorySource: source, SnapshotID: snapshot.ID}
	workqueue.RefreshCheckpoint(checkpoint)
	return snapshot, detailDocument, checkpoint, nil
}

// SourceFile returns the qualified bytes of a regular 100644 target-tree file.
func SourceFile(source *worksource.Source, path string) ([]byte, error) {
	for _, entry := range source.Entries {
		if entry.Path == path && entry.Mode == "100644" {
			return entry.Raw, nil
		}
	}
	return nil, fmt.Errorf("qualified source file unavailable: %s", path)
}
