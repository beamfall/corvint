// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// qualificationCaseIDs are the ten fixed PMR-V2-006 qualification cases.
var qualificationCaseIDs = []string{
	"owned-launch-joins", "two-fresh-births", "cleanup-grandchild-interruption", "independent-absence",
	"legacy-samples-refused", "roles-ambiguous-refused", "graph-substitution-refused", "pid-reuse-refused",
	"schedule-order-preserved", "role-witness-order-invariant",
}

// qualificationRefusalsV2 hardcodes each negative case's native expected code;
// expected outcomes are never read from qualification JSON.
var qualificationRefusalsV2 = map[string]string{
	"legacy-samples-refused":     "process-legacy-sample-unsupported",
	"roles-ambiguous-refused":    "process-role-ambiguous",
	"graph-substitution-refused": "process-graph-substituted",
	"pid-reuse-refused":          "process-birth-changed",
}

// qualificationPairsV2 is the fixed graph/proof count of each case.
var qualificationPairsV2 = map[string]int{
	"two-fresh-births": 2, "schedule-order-preserved": 2, "role-witness-order-invariant": 2,
}

type qualifiedRunV2 struct {
	graphSHA string
	proofSHA string
	result   rawProcessResultV2
}

// verifyQualificationV2 checks the parent-pinned host qualification report
// and rechecks every frozen case invariant through verifyRawProcessV2, which
// never mints a token, so qualification does not call itself.
func verifyQualificationV2(ctx context.Context, admission ProcessAdmissionV2, policy ProcessPolicyV2, policySHA string,
	data []byte, store ArtifactReaderV2) error {
	var report ProcessQualificationV2
	if err := decodeWire(data, "ProcessQualificationV2", &report); err != nil {
		return rejected("process-qualification-invalid", err.Error())
	}
	if report.PolicySHA256 != policySHA || report.Implementation != admission.Implementation || report.Host != admission.HostTuple {
		return rejected("process-qualification-invalid", "qualification is for another policy, implementation or host tuple")
	}
	if len(report.Cases) != len(qualificationCaseIDs) {
		return rejected("process-qualification-invalid", "exactly one of each fixed qualification case is required")
	}
	seen := map[string]bool{}
	for _, c := range report.Cases {
		if seen[c.ID] {
			return rejected("process-qualification-invalid", "duplicate qualification case "+c.ID)
		}
		seen[c.ID] = true
		if err := verifyQualificationCaseV2(ctx, admission, policy, policySHA, c, store); err != nil {
			// A rejected case preimage invalidates the report; a blocked read
			// (unavailable, bounded or cancelled) keeps its own code.
			var refusal *ProcessErrorV2
			if errors.As(err, &refusal) && refusal.Outcome == OutcomeRejectedV2 && refusal.Code != "process-qualification-invalid" {
				return rejected("process-qualification-invalid", c.ID+": "+refusal.Code+": "+refusal.Detail)
			}
			return err
		}
	}
	return nil
}

func verifyQualificationCaseV2(ctx context.Context, admission ProcessAdmissionV2, policy ProcessPolicyV2, policySHA string,
	c QualificationCaseV2, store ArtifactReaderV2) error {
	invalid := func(detail string) error {
		return rejected("process-qualification-invalid", c.ID+": "+detail)
	}
	pairs := qualificationPairsV2[c.ID]
	if pairs == 0 {
		pairs = 1
	}
	if c.Policy != admission.Policy || len(c.GraphBindings) != pairs || len(c.Proofs) != pairs {
		return invalid("policy or graph/proof count differs from the fixed case")
	}
	reader := &verifierV2{ctx: ctx, store: store, policy: policy, documents: map[string][]byte{}, executables: map[string]bool{}}
	graphs := make([]GraphBindingV2, pairs)
	for i, ref := range c.GraphBindings {
		data, err := reader.readDocument(ref)
		if err != nil {
			return err
		}
		if err := decodeWire(data, "GraphBindingV2", &graphs[i]); err != nil {
			return invalid(err.Error())
		}
	}
	reader.graph = graphs[0]
	harness, err := reader.invocationV2(c.HarnessInvocation)
	if err != nil {
		return err
	}
	if harness.Purpose != "host-supervisor" || harness.Executable.SHA256 != admission.Implementation.SupervisorSHA256 {
		return invalid("harness was not executed by the admitted supervisor")
	}
	if _, err := reader.readDocument(c.HarnessStdout); err != nil {
		return err
	}
	runs := make([]qualifiedRunV2, pairs)
	var failures []error
	for i, ref := range c.Proofs {
		proof, err := reader.readDocument(ref)
		if err != nil {
			return err
		}
		result, err := verifyRawProcessV2(ctx, policy, policySHA, graphs[i], proof, store)
		runs[i] = qualifiedRunV2{graphSHA: result.graphSHA, proofSHA: ref.SHA256, result: result}
		failures = append(failures, err)
	}
	for _, err := range failures {
		if blockedReadV2(err) {
			return err
		}
	}
	if code, negative := qualificationRefusalsV2[c.ID]; negative {
		for _, err := range failures {
			var refusal *ProcessErrorV2
			if !errors.As(err, &refusal) || refusal.Code != code {
				return invalid("negative control did not refuse with " + code)
			}
		}
		return nil
	}
	for _, err := range failures {
		if err != nil {
			return invalid("positive control was refused: " + err.Error())
		}
	}
	if !qualificationInvariantV2(c.ID, runs) {
		return invalid("frozen case invariant does not hold")
	}
	return nil
}

// blockedReadV2 reports a refusal from reading evidence rather than from the
// evidence itself: the case cannot be judged, so it keeps its own code.
func blockedReadV2(err error) bool {
	var refusal *ProcessErrorV2
	if !errors.As(err, &refusal) || refusal.Outcome != OutcomeBlockedV2 {
		return false
	}
	switch refusal.Code {
	case "process-artifact-unavailable", "process-bound-exceeded", "process-verification-cancelled":
		return true
	}
	return false
}

func qualificationInvariantV2(id string, runs []qualifiedRunV2) bool {
	first := runs[0].result
	switch id {
	case "owned-launch-joins":
		return first.references > 0 && first.cleanups > 0
	case "independent-absence":
		return len(first.workload) > 0
	case "cleanup-grandchild-interruption":
		return interruptedGrandchildV2(first.logical)
	case "two-fresh-births":
		second := runs[1].result
		for _, key := range second.workload {
			if slices.Contains(first.workload, key) {
				return false
			}
		}
		return runs[0].graphSHA != runs[1].graphSHA && slices.EqualFunc(first.logical, second.logical, sameTopologyV2)
	case "schedule-order-preserved":
		return runs[0].graphSHA != runs[1].graphSHA && !slices.EqualFunc(first.logical, runs[1].result.logical, sameLogicalV2)
	case "role-witness-order-invariant":
		return runs[0].graphSHA == runs[1].graphSHA && runs[0].proofSHA != runs[1].proofSHA &&
			slices.EqualFunc(first.logical, runs[1].result.logical, sameLogicalV2)
	}
	return false
}

// interruptedGrandchildV2 requires a cancelled or timed-out workload node
// whose own parent is a workload node, so its retirement was owned below the
// supervisor's direct child and proved absent by the final sweep.
func interruptedGrandchildV2(logical []LogicalProcessV2) bool {
	roles := map[string]string{}
	for _, p := range logical {
		roles[p.Node] = p.Role
	}
	for _, p := range logical {
		outcome, _, _ := strings.Cut(p.State, "/")
		interrupted := slices.ContainsFunc(strings.Split(outcome, "+"), func(s string) bool { return s == "cancelled" || s == "timed-out" })
		if interrupted && p.Parent != nil && roles[*p.Parent] != "" && roles[*p.Parent] != "host-supervisor" {
			return true
		}
	}
	return false
}

func sameTopologyV2(a, b LogicalProcessV2) bool {
	return a.Node == b.Node && a.Role == b.Role && a.RunKind == b.RunKind && a.RunOrdinal == b.RunOrdinal &&
		equalParent(a.Parent, b.Parent) && slices.Equal(a.BirthDistinctFrom, b.BirthDistinctFrom)
}

func sameLogicalV2(a, b LogicalProcessV2) bool { return sameTopologyV2(a, b) && a.State == b.State }

func equalParent(a, b *string) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
