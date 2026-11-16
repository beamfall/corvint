// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// The `flows status` document (AFU-V1-160 to AFU-V1-165, V1-5257): one trackable view of each flow,
// its variations, the tests attached to each and the last recorded result of each test.
const (
	StatusSchema      = "application-flow-status/0"
	ResultNotObserved = "NOT_OBSERVED"
	MaxStatusTests    = 8192
)

// StatusReport is `flows status`. With a base it holds exactly the flows `flows impact` names and
// the impact graph's scope, so an UNKNOWN scope (a possibly short list) is never silent (AFU-V1-163).
type StatusReport struct {
	Schema        string       `json:"schema"`
	Revision      string       `json:"revision"`
	Base          string       `json:"base,omitempty"`
	LimitedByBase bool         `json:"limited_by_base"`
	ImpactGraph   *ImpactGraph `json:"impact_graph,omitempty"`
	Flows         []StatusFlow `json:"flows"`
	Totals        StatusTotals `json:"totals"`
	Truncated     bool         `json:"truncated"`
	// RunEvidence is what the run evidence store gave the report; absent when the store was not read
	// (--no-store) or held nothing (RVD-V1-007, RVD-V1-010).
	RunEvidence *RunEvidenceRead `json:"run_evidence,omitempty"`
	// ResultSources lists every place the report looked for results, read or not, so a report
	// whose results are all NOT_OBSERVED names the sources it missed (AFL-V1-002).
	ResultSources []ResultSource `json:"result_sources"`
	// Inventory is the E2E inventory the unassigned count was taken from; absent without a
	// provider, and then Unassigned is null, never zero (AFL-V1-001, AFU-V1-150).
	Inventory  InventoryState `json:"inventory"`
	Unassigned *int           `json:"unassigned"`
}

// StatusFlow is one flow in intent order; the report computes no complete/incomplete status, which
// needs the run registry (`flows gaps`).
type StatusFlow struct {
	FlowID     string            `json:"flow_id"`
	Proposed   bool              `json:"proposed"`
	Variations []StatusVariation `json:"variations"`
}

// StatusVariation is one variation with its attached tests; Code is no-test when there is none.
type StatusVariation struct {
	VariationID string       `json:"variation_id"`
	Projects    []string     `json:"projects"`
	Code        string       `json:"code,omitempty"`
	Tests       []StatusTest `json:"tests"`
}

// StatusTest is one attached test and project pair with its last recorded result (AFU-V1-161,
// RVD-V1-007): NOT_OBSERVED with no record, else the AFU-V1-013 classification of the newest
// record, stale unless that record is bound to the evaluated commit and tree from a clean worktree.
type StatusTest struct {
	TestKey      string `json:"test_key"`
	Project      string `json:"project"`
	Result       string `json:"result"`
	Stale        bool   `json:"stale"`
	SourceCommit string `json:"source_commit,omitempty"`
	// SourceTree is the chosen record's tree; Stale says whether it differs from the evaluated one
	// (RVD-V1-007).
	SourceTree string `json:"source_tree,omitempty"`
	RunID      string `json:"run_id,omitempty"`
	Authority  string `json:"authority,omitempty"`
	// Reason is run-evidence-corrupt when the result is NOT_OBSERVED and a skipped store record held
	// the test (RVD-V1-010).
	Reason string `json:"reason,omitempty"`
	// Granularity is test when the result names this test, or file when it is the outcome of the
	// test's file or package that the runner reported only together; absent for NOT_OBSERVED
	// (AFL-V1-002).
	Granularity string `json:"granularity,omitempty"`
}

// StatusTotals counts everything the report describes, dropped rows included.
type StatusTotals struct {
	Flows      int `json:"flows"`
	Variations int `json:"variations"`
	Tests      int `json:"tests"`
}

// StatusOptions are a flows status report's inputs besides the intents: the base that limits it to
// the flows a change reaches, the results ReadResults read, and the E2E inventory provider whose
// tests no variation links are counted as unassigned.
type StatusOptions struct {
	Base      string
	Results   Results
	Inventory *InventoryProvider
}

// FlowStatus reports each flow, variation, attached test and last result at the set's revision
// (AFL-V1-001, AFU-V1-160). A non-empty base limits the report to exactly the flows and variations
// FlowImpactAt names for it (AFL-V1-004). Test rows past MaxStatusTests are dropped in report order
// with Truncated set (AFL-V1-005). The store read, if any, is carried, and a test with no result
// that a skipped store record held names run-evidence-corrupt (RVD-V1-010).
func FlowStatus(ctx context.Context, root string, set IntentSet, options StatusOptions) ([]byte, error) {
	evidence, store, base := options.Results.Records, options.Results.Store, options.Base
	report := StatusReport{Schema: StatusSchema, Flows: []StatusFlow{}, ResultSources: nonNilSources(options.Results.Sources), Inventory: absentInventory()}
	if store.present() {
		report.RunEvidence = store
	}
	flows := set.Flows
	var touched map[string]bool
	if base != "" {
		raw, err := FlowImpactAt(ctx, root, set, base)
		if err != nil {
			return nil, err
		}
		var impact ImpactReport
		if err := json.Unmarshal(raw, &impact); err != nil {
			return nil, errors.New("flows impact produced an unreadable report")
		}
		reached := map[string]bool{}
		touched = map[string]bool{}
		for _, f := range impact.Flows {
			reached[f.FlowID] = true
			for _, v := range f.Variations {
				touched[f.FlowID+"\x00"+v] = true
			}
		}
		flows = slices.DeleteFunc(slices.Clone(set.Flows), func(intent FlowIntent) bool { return !reached[intent.FlowID] })
		graph := impact.Graph
		report.Base, report.LimitedByBase, report.ImpactGraph = base, true, &graph
	}
	links, at, err := evaluateSet(ctx, root, set)
	if err != nil {
		return nil, err
	}
	report.Revision = at.commit
	if options.Inventory != nil {
		report.Inventory = options.Inventory.state(root, at)
		_, unassigned := unlinkedTests(set, links, options.Inventory.Provider.Inventory)
		report.Unassigned = &unassigned
	}
	for _, intent := range flows {
		own := flowLinks(links, intent.FlowID)
		flow := StatusFlow{FlowID: intent.FlowID, Proposed: intent.Proposed, Variations: []StatusVariation{}}
		for _, v := range intent.Variations {
			if touched != nil && !touched[intent.FlowID+"\x00"+v.VariationID] {
				continue
			}
			variation := StatusVariation{VariationID: v.VariationID, Projects: nonNil(v.Projects), Tests: []StatusTest{}}
			projects := v.Projects
			if len(projects) == 0 {
				projects = []string{""}
			}
			targets := declaredTestTargets(own, v.VariationID)
			for _, target := range targets {
				for _, project := range projects {
					report.Totals.Tests++
					if report.Totals.Tests > MaxStatusTests {
						report.Truncated = true
						continue
					}
					result := lastResultFor(evidence, target, project, at)
					if result.Result == ResultNotObserved && store.corruptKey(target.key, project) {
						result.Reason = "run-evidence-corrupt"
					}
					variation.Tests = append(variation.Tests, result)
				}
			}
			if len(targets) == 0 {
				variation.Code = GapNoTest
			}
			slices.SortFunc(variation.Tests, func(a, b StatusTest) int {
				if c := strings.Compare(a.TestKey, b.TestKey); c != 0 {
					return c
				}
				return strings.Compare(a.Project, b.Project)
			})
			flow.Variations = append(flow.Variations, variation)
			report.Totals.Variations++
		}
		report.Flows = append(report.Flows, flow)
		report.Totals.Flows++
	}
	return doccorpus.Encode(report)
}

// lastResult is lastResultFor a target known only by its key.
func lastResult(evidence []TestRunEvidence, key, project string, at head) StatusTest {
	return lastResultFor(evidence, testTarget{key: key}, project, at)
}

// lastResultFor picks the newest record that names the pair, the last in evidence order (the
// store's records in append order, then the named receipts and --evidence files), whatever its
// tree (RVD-V1-007): a newer failing run is never hidden behind an older passing one. A record
// names the pair when its key is the target's, or when it is a file-level receipt row for the
// target's test file; the result then carries granularity file and is never presented as a
// per-title result (AFL-V1-002). Freshness is reported separately: the result is stale, with its
// commit and tree, unless the record is bound to the evaluated commit and tree from a clean
// worktree. STATIC records never count (AFU-V1-014); with no record the result is NOT_OBSERVED.
func lastResultFor(evidence []TestRunEvidence, target testTarget, project string, at head) StatusTest {
	t := StatusTest{TestKey: target.key, Project: project, Result: ResultNotObserved}
	var pick *TestRunEvidence
	granularity := ""
	for i := range evidence {
		r := &evidence[i]
		if r.Authority == AuthorityStatic {
			continue
		}
		if g, ok := matchesTarget(r, target, project); ok {
			pick, granularity = r, g
		}
	}
	if pick == nil {
		return t
	}
	stale := pick.Source != (RunSource{Commit: at.commit, Tree: at.tree, Clean: true})
	t.Result, t.Stale, t.SourceCommit, t.RunID, t.Authority = Classify(pick.Attempts), stale, pick.Source.Commit, pick.RunID, pick.Authority
	t.SourceTree, t.Granularity = pick.Source.Tree, granularity
	return t
}

// nonNilSources keeps the result_sources member an array.
func nonNilSources(sources []ResultSource) []ResultSource {
	if sources == nil {
		return []ResultSource{}
	}
	return sources
}
