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
}

// StatusTotals counts everything the report describes, dropped rows included.
type StatusTotals struct {
	Flows      int `json:"flows"`
	Variations int `json:"variations"`
	Tests      int `json:"tests"`
}

// FlowStatus reports each flow, variation, attached test and last result at the set's revision
// (AFU-V1-160). A non-empty base limits the flows to those FlowImpactAt names for it (AFU-V1-163).
// Test rows past MaxStatusTests are dropped in report order with Truncated set (AFU-V1-164).
func FlowStatus(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence, base string) ([]byte, error) {
	return FlowStatusFrom(ctx, root, set, evidence, base, nil)
}

// FlowStatusFrom is FlowStatus over evidence that includes the run evidence store's records, with
// what the store read gave (RVD-V1-007): the report carries the read, and a test with no result
// that a skipped record held names run-evidence-corrupt (RVD-V1-010).
func FlowStatusFrom(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence, base string, store *RunEvidenceRead) ([]byte, error) {
	report := StatusReport{Schema: StatusSchema, Flows: []StatusFlow{}}
	if store.present() {
		report.RunEvidence = store
	}
	flows := set.Flows
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
		for _, f := range impact.Flows {
			reached[f.FlowID] = true
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
	for _, intent := range flows {
		own := flowLinks(links, intent.FlowID)
		flow := StatusFlow{FlowID: intent.FlowID, Proposed: intent.Proposed, Variations: []StatusVariation{}}
		for _, v := range intent.Variations {
			variation := StatusVariation{VariationID: v.VariationID, Projects: nonNil(v.Projects), Tests: []StatusTest{}}
			projects := v.Projects
			if len(projects) == 0 {
				projects = []string{""}
			}
			for _, key := range declaredTargets(own, "test", v.VariationID) {
				for _, project := range projects {
					report.Totals.Tests++
					if report.Totals.Tests > MaxStatusTests {
						report.Truncated = true
						continue
					}
					result := lastResult(evidence, key, project, at)
					if result.Result == ResultNotObserved && store.corruptKey(key, project) {
						result.Reason = "run-evidence-corrupt"
					}
					variation.Tests = append(variation.Tests, result)
				}
			}
			if len(declaredTargets(own, "test", v.VariationID)) == 0 {
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

// lastResult picks the newest record that names the pair, the last in evidence order (the store's
// records in append order, then the --evidence files), whatever its tree (RVD-V1-007): a newer
// failing run is never hidden behind an older passing one. Freshness is reported separately: the
// result is stale, with its commit and tree, unless the record is bound to the evaluated commit and
// tree from a clean worktree. STATIC records never count (AFU-V1-014); with no record the result
// is NOT_OBSERVED (AFU-V1-161).
func lastResult(evidence []TestRunEvidence, key, project string, at head) StatusTest {
	t := StatusTest{TestKey: key, Project: project, Result: ResultNotObserved}
	var pick *TestRunEvidence
	for i := range evidence {
		r := &evidence[i]
		if r.Authority == AuthorityStatic || r.TestKey != key || (project != "" && r.Project != project) {
			continue
		}
		pick = r
	}
	if pick == nil {
		return t
	}
	stale := pick.Source != (RunSource{Commit: at.commit, Tree: at.tree, Clean: true})
	t.Result, t.Stale, t.SourceCommit, t.RunID, t.Authority = Classify(pick.Attempts), stale, pick.Source.Commit, pick.RunID, pick.Authority
	t.SourceTree = pick.Source.Tree
	return t
}
