// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"maps"
	"slices"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

type canonicalLinkReport struct {
	rows       map[string][]StatusTest
	inventory  InventoryState
	gaps       []Gap
	unassigned int
}

// linkStatus reads canonical test IDs and immutable bindings for the report's commit.
// The version 0 report continues to use its provider and reviewed intent links.
func linkStatus(ctx context.Context, root string, set IntentSet, at head) (*canonicalLinkReport, error) {
	entry, err := git(ctx, root, "ls-tree", at.commit, "--", FlowsDeclarationPath)
	if err != nil {
		return nil, err
	}
	if len(entry) == 0 {
		return nil, nil
	}
	rows := map[string][]StatusTest{}
	tree, err := testinventory.NewCommitTree(ctx, testinventory.Git{Root: root, Executable: gitstatus.Executable()}, at.commit)
	if err != nil {
		return nil, err
	}
	raw, err := tree.Read(FlowsDeclarationPath, maxFlowsDeclarationBytes)
	if err != nil {
		return nil, err
	}
	d, err := decodeFlowsDeclaration(raw)
	if err != nil {
		return nil, err
	}
	if d.Version != 1 || d.Dir != set.Dir {
		return nil, nil
	}
	files, _ := tree.Files()
	variants := linkVariations(set)
	known := map[string]bool{}
	inventory := InventoryState{Source: testinventory.Path, State: InventoryUnverified, BoundTo: at.commit, Reason: "canonical inventory is unavailable"}
	data, err := tree.Read(testinventory.Path, testinventory.MaxBytes)
	if err == nil {
		inv, e := testinventory.Decode(data)
		if e == nil {
			verdict := testinventory.Judge(tree)
			inventory.State, inventory.Reason = InventoryComplete, "canonical inventory at the report commit"
			listed := testinventory.Listed(data, verdict)
			for _, iv := range verdict.Instances {
				if iv.Runner == testid.Playwright || iv.Runner == testid.Pytest || iv.Runner == testid.RSpec {
					if iv.Full || len(iv.Unassigned) > 0 {
						inventory.State, inventory.Reason = InventoryPartial, "browser inventory has stale or unassigned files"
					}
				}
			}
			for _, in := range inv.Instances {
				for _, t := range in.Tests {
					if browserTest(in.Runner, t) && slices.Contains(listed[in.Key][t.File], t.ID) {
						known[t.ID] = true
					}
				}
			}
		}
	}
	assigned := map[string]bool{}
	reasons := map[string]string{}
	invalid := invalidLinkPins(ctx, tree.Git, at.commit, d)
	for _, l := range d.Links {
		id, _ := testid.Parse(l.TestID)
		v, ok := variants[l.VariationID]
		if !ok {
			reasons[l.TestID] = linkDangling
			inventory.Reason += "; " + linkDangling + " " + l.VariationID + " " + l.TestID
			continue
		}
		project := ""
		if id.Runner == testid.Playwright && len(id.Segments) > 0 {
			project = id.Segments[0]
		}
		row := StatusTest{TestKey: l.TestID, Project: project, Result: ResultNotObserved}
		switch {
		case !l.HasAcceptance():
			row.Reason = "unaccepted"
		case !known[l.TestID] && inventory.State == InventoryComplete:
			row.Reason = linkDangling
		case invalid[l.TestID+"\x00"+l.VariationID] || l.TestContentHash != files[id.File] || l.VariationDigest != v.digest:
			row.Stale, row.Reason = true, linkStale
		case !known[l.TestID] || len(v.sources) == 0:
			row.Reason = linkUnknown
		default:
			assigned[l.TestID] = true
		}
		if row.Reason != "" {
			reasons[l.TestID] = row.Reason
		}
		rows[v.id] = append(rows[v.id], row)
	}
	inventory.Tests = len(known)
	report := &canonicalLinkReport{rows: rows, inventory: inventory, gaps: []Gap{}}
	for _, key := range slices.Sorted(maps.Keys(known)) {
		if assigned[key] && reasons[key] == "" {
			continue
		}
		report.unassigned++
		if len(report.gaps) >= MaxUnlinkedTests {
			continue
		}
		id, _ := testid.Parse(key)
		reason := reasons[key]
		if reason == "" {
			reason = testinventory.ReasonUnassigned
		}
		gap := Gap{Code: GapUnlinkedTest, TestKey: key, Detail: reason + "; file " + id.File}
		if id.Runner == testid.Playwright && len(id.Segments) > 0 {
			gap.Project = id.Segments[0]
		}
		report.gaps = append(report.gaps, gap)
	}
	return report, nil
}
