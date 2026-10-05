// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"slices"
	"strconv"
	"strings"
)

// nativeLocator is one actual process-bearing native field at a finite
// locator: provider freshness/app-instance/descendant rows and report cleanup
// descendant rows. Nothing else is a process reference target.
type nativeLocator struct {
	kind    string
	context int
	node    *jsonNode
	// server groups the fresh-process and app-instance locators of one
	// provider artifact, which must all join the same server birth.
	server string
}

type cleanupTarget struct {
	context  []int // admissible producer contexts
	facts    CleanupFactsV2
	provider bool
}

type nativeSourcesV2 struct {
	locators map[string]nativeLocator // artifact ID + NUL + pointer
	cleanups map[string]cleanupTarget // report pointer
	// providerCleanup holds provider descendant observations, which agree with
	// absence proofs only when they are themselves observed absent.
	providerCleanup []CleanupFactsV2
}

func locatorKey(artifactID, pointer string) string { return artifactID + "\x00" + pointer }

func compareSourceV2(a, b SourceBindingV2) int {
	if a.Kind != b.Kind {
		return strings.Compare(a.Kind, b.Kind)
	}
	return strings.Compare(a.Artifact.ID, b.Artifact.ID)
}

func member(n *jsonNode, key string) (*jsonNode, bool) {
	if n == nil || n.kind != 'o' {
		return nil, false
	}
	child, ok := n.obj[key]
	return child, ok
}

// loadNativeSourcesV2 reads the hash-verified report and provider receipts of
// the expected graph and enumerates their finite process locators and outer
// cleanup objects. It checks the producer contexts against the raw report.
func (v *verifierV2) loadNativeSourcesV2() (*nativeSourcesV2, error) {
	graph := v.graph
	reportBytes, err := v.readDocument(graph.Report)
	if err != nil {
		return nil, err
	}
	report, err := parseJSONTree(reportBytes, false)
	if err != nil || report.kind != 'o' {
		return nil, rejected("process-native-reference-invalid", "report is not a JSON object")
	}
	runs, okRuns := member(report, "runs")
	controls, okControls := member(report, "controls")
	if !okRuns || !okControls || runs.kind != 'a' || controls.kind != 'a' {
		return nil, rejected("process-native-reference-invalid", "report runs/controls arrays are required")
	}
	runContext, controlContexts, err := checkContextsV2(graph.Contexts, runs.arr, len(controls.arr))
	if err != nil {
		return nil, err
	}
	out := &nativeSourcesV2{locators: map[string]nativeLocator{}, cleanups: map[string]cleanupTarget{}}
	receiptRun := map[string][]int{}
	addCleanup := func(owner *jsonNode, pointer string, contexts []int) error {
		cleanup, ok := member(owner, "cleanup")
		if !ok {
			return rejected("process-native-reference-invalid", pointer+" is missing")
		}
		facts, descendants, err := cleanupFactsV2(cleanup)
		if err != nil {
			return rejected("process-native-reference-invalid", pointer+": "+err.Error())
		}
		out.cleanups[pointer] = cleanupTarget{context: contexts, facts: facts}
		if descendants != nil {
			for j, row := range descendants {
				out.locators[locatorKey(graph.Report.ID, pointer+"/descendants/processes/"+strconv.Itoa(j))] = nativeLocator{kind: "descendant", context: contexts[0], node: row}
			}
		}
		return nil
	}
	for i, run := range runs.arr {
		pointer := "/runs/" + strconv.Itoa(i) + "/cleanup"
		if err := addCleanup(run, pointer, []int{runContext[i]}); err != nil {
			return nil, err
		}
		if receipt, ok := member(run, "receipt_sha256"); ok {
			if receipt.kind != 's' {
				return nil, rejected("process-native-reference-invalid", "run receipt_sha256 is not a string")
			}
			receiptRun[receipt.str] = append(receiptRun[receipt.str], i)
		}
	}
	for c, control := range controls.arr {
		if err := addCleanup(control, "/controls/"+strconv.Itoa(c)+"/cleanup", controlContexts[c]); err != nil {
			return nil, err
		}
	}
	ids := map[string]bool{}
	for i, source := range graph.Sources {
		if i > 0 && compareSourceV2(graph.Sources[i-1], source) >= 0 {
			return nil, blocked("process-graph-invalid", "sources are not strictly sorted by kind and artifact ID")
		}
		if ids[source.Artifact.ID] {
			return nil, blocked("process-graph-invalid", "duplicate source artifact ID")
		}
		ids[source.Artifact.ID] = true
		switch source.Kind {
		case "request", "report":
			if (source.Kind == "request" && source.Artifact != graph.Request) || (source.Kind == "report" && source.Artifact != graph.Report) {
				return nil, blocked("process-graph-invalid", source.Kind+" source differs from the graph "+source.Kind)
			}
			continue
		case "provider":
		default:
			// Hook, control, attestation-output and producer-job locators are
			// not enumerated yet, so their process fields cannot be joined.
			return nil, blocked("process-native-source-unsupported", source.Kind+" process locators are not implemented")
		}
		owners := receiptRun["sha256:"+source.Artifact.SHA256]
		if len(owners) != 1 {
			return nil, blocked("process-graph-invalid", "provider receipt does not map to exactly one report run")
		}
		data, err := v.readDocument(source.Artifact)
		if err != nil {
			return nil, err
		}
		provider, err := parseJSONTree(data, false)
		if err != nil || provider.kind != 'o' {
			return nil, rejected("process-native-reference-invalid", "provider receipt is not a JSON object")
		}
		if err := out.addProviderLocators(source.Artifact.ID, provider, runContext[owners[0]]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *nativeSourcesV2) addProviderLocators(id string, provider *jsonNode, context int) error {
	add := func(pointer, kind string, node *jsonNode) {
		locator := nativeLocator{kind: kind, context: context, node: node}
		if kind != "descendant" {
			locator.server = id
		}
		s.locators[locatorKey(id, pointer)] = locator
	}
	observation := func(pointer string, node *jsonNode) error {
		if node.kind != 'o' {
			return rejected("process-native-reference-invalid", pointer+" is not an object")
		}
		facts, rows, err := descendantFactsV2(node)
		if err != nil {
			return rejected("process-native-reference-invalid", pointer+": "+err.Error())
		}
		s.providerCleanup = append(s.providerCleanup, facts)
		for j, row := range rows {
			add(pointer+"/processes/"+strconv.Itoa(j), "descendant", row)
		}
		return nil
	}
	if freshness, ok := member(provider, "freshness"); ok {
		if freshness.kind != 'o' {
			return rejected("process-native-reference-invalid", "/freshness is not an object")
		}
		if leader, ok := member(freshness, "leader"); ok {
			add("/freshness/leader", "fresh-process", leader)
		}
		for _, served := range []string{"servedBefore", "servedAfter"} {
			if observation, ok := member(freshness, served); ok {
				process, ok := member(observation, "process")
				if !ok {
					return rejected("process-native-reference-invalid", "/freshness/"+served+"/process is missing")
				}
				add("/freshness/"+served+"/process", "fresh-process", process)
			}
		}
		if descendants, ok := member(freshness, "serverDescendants"); ok {
			if err := observation("/freshness/serverDescendants", descendants); err != nil {
				return err
			}
		}
	}
	if descendants, ok := member(provider, "descendantObservation"); ok {
		if err := observation("/descendantObservation", descendants); err != nil {
			return err
		}
	}
	if attestation, ok := member(provider, "applicationAttestation"); ok {
		for _, side := range []string{"before", "after"} {
			if observed, ok := member(attestation, side); ok {
				inner, _ := member(observed, "attestation")
				instance, ok := member(inner, "instance")
				if !ok {
					return rejected("process-native-reference-invalid", "/applicationAttestation/"+side+"/attestation/instance is missing")
				}
				add("/applicationAttestation/"+side+"/attestation/instance", "app-instance", instance)
			}
		}
	}
	return nil
}

// checkContextsV2 checks the producer-derived contexts against the raw report:
// context 0 is the workflow, then report runs in original order, then control
// contexts in original control order.
func checkContextsV2(contexts []RunContextV2, runs []*jsonNode, controls int) ([]int, [][]int, error) {
	invalid := func(detail string) ([]int, [][]int, error) {
		return nil, nil, blocked("process-graph-invalid", detail)
	}
	if len(contexts) == 0 {
		return invalid("context 0 is required")
	}
	first := contexts[0]
	if first.RunKind != "workflow" || first.RunIndex != nil || first.ControlIndex != nil || first.ControlOrdinal != nil || first.AttemptOrdinal != nil || first.RunOrdinal != 0 {
		return invalid("context 0 must be the workflow context")
	}
	runContext := make([]int, 0, len(runs))
	controlContexts := make([][]int, controls)
	lastControl := -1
	for i := 1; i < len(contexts); i++ {
		c := contexts[i]
		switch {
		case c.RunIndex != nil && c.ControlIndex == nil && c.ControlOrdinal == nil && c.AttemptOrdinal == nil && lastControl < 0:
			if *c.RunIndex != len(runContext) || *c.RunIndex >= len(runs) {
				return invalid("run contexts must follow report run order")
			}
			run := runs[*c.RunIndex]
			kind, okKind := member(run, "kind")
			ordinal, okOrdinal := member(run, "ordinal")
			value, okValue := ordinal.integer()
			if !okKind || !okOrdinal || !okValue || kind.kind != 's' || c.RunKind != kind.str || int64(c.RunOrdinal) != value || !slices.Contains([]string{"repeat", "probe-original", "probe-reversed", "probe-isolated"}, c.RunKind) {
				return invalid("run context does not match report run kind/ordinal")
			}
			runContext = append(runContext, i)
		case c.RunIndex == nil && c.ControlIndex != nil && c.RunKind == "control":
			if *c.ControlIndex < lastControl || *c.ControlIndex >= controls {
				return invalid("control contexts must follow report control order")
			}
			lastControl = *c.ControlIndex
			controlContexts[lastControl] = append(controlContexts[lastControl], i)
		default:
			return invalid("unrecognized context coordinates")
		}
	}
	if len(runContext) != len(runs) {
		return invalid("every report run needs exactly one context")
	}
	for _, contexts := range controlContexts {
		if len(contexts) == 0 {
			return invalid("every report control needs a context")
		}
	}
	return runContext, controlContexts, nil
}

var cleanupMembers = []string{"owned_group", "status", "qualification", "cancelled", "timed_out"}

// cleanupFactsV2 extracts testacceptance Cleanup exactly and recomputes the
// native cleanupState; descendants presence stays distinct from absence.
func cleanupFactsV2(n *jsonNode) (CleanupFactsV2, []*jsonNode, error) {
	var facts CleanupFactsV2
	if n.kind != 'o' {
		return facts, nil, errorString("cleanup is not an object")
	}
	descendants, present := n.obj["descendants"]
	if len(n.obj) != len(cleanupMembers)+map[bool]int{true: 1, false: 0}[present] {
		return facts, nil, errorString("cleanup members are not closed")
	}
	for _, key := range cleanupMembers {
		if _, ok := n.obj[key]; !ok {
			return facts, nil, errorString("cleanup member " + key + " is missing")
		}
	}
	values := []struct {
		key string
		out *bool
	}{{"owned_group", &facts.OwnedGroup}, {"cancelled", &facts.Cancelled}, {"timed_out", &facts.TimedOut}}
	for _, value := range values {
		if node := n.obj[value.key]; node.kind == 'b' {
			*value.out = node.b
		} else {
			return facts, nil, errorString("cleanup " + value.key + " is not a boolean")
		}
	}
	status, qualification := n.obj["status"], n.obj["qualification"]
	if status.kind != 's' || qualification.kind != 's' || !slices.Contains([]string{"owned-process-group", "descendant-cleanup-unsupported", "ancestor-process-group"}, status.str) {
		return facts, nil, errorString("cleanup status/qualification is unsupported")
	}
	facts.Status, facts.Qualification = status.str, qualification.str
	facts.Failures, facts.Limitations = []string{}, []string{}
	var rows []*jsonNode
	if present {
		observed, observedRows, err := descendantFactsV2(descendants)
		if err != nil {
			return facts, nil, err
		}
		facts.DescendantsPresent = true
		facts.Absent, facts.Scope, facts.IntervalMS = observed.Absent, observed.Scope, observed.IntervalMS
		facts.Failures, facts.Limitations = observed.Failures, observed.Limitations
		rows = observedRows
	}
	facts.DerivedState = cleanupStateV2(facts)
	return facts, rows, nil
}

// descendantFactsV2 reads one procgroup DescendantObservation object.
func descendantFactsV2(n *jsonNode) (CleanupFactsV2, []*jsonNode, error) {
	var facts CleanupFactsV2
	if n == nil || n.kind != 'o' || len(n.obj) != 6 {
		return facts, nil, errorString("descendant observation members are not closed")
	}
	scope, interval, processes, absent := n.obj["scope"], n.obj["interval_ms"], n.obj["processes"], n.obj["absent"]
	failures, limitations := n.obj["failures"], n.obj["limitations"]
	value, okInterval := interval.integer()
	if scope == nil || scope.kind != 's' || !okInterval || value < 0 || processes == nil || processes.kind != 'a' || absent == nil || absent.kind != 'b' {
		return facts, nil, errorString("descendant observation is malformed")
	}
	var err error
	if facts.Failures, err = stringArray(failures); err != nil {
		return facts, nil, err
	}
	if facts.Limitations, err = stringArray(limitations); err != nil {
		return facts, nil, err
	}
	facts.DescendantsPresent, facts.Absent, facts.Scope, facts.IntervalMS = true, absent.b, scope.str, int(value)
	facts.OwnedGroup = true
	facts.DerivedState = cleanupStateV2(facts)
	return facts, processes.arr, nil
}

func stringArray(n *jsonNode) ([]string, error) {
	if n == nil || n.kind != 'a' {
		return nil, errorString("string array is required")
	}
	out := make([]string, 0, len(n.arr))
	for _, child := range n.arr {
		if child.kind != 's' {
			return nil, errorString("string array is required")
		}
		out = append(out, child.str)
	}
	return out, nil
}

// cleanupStateV2 is testacceptance cleanupState over extracted facts.
func cleanupStateV2(f CleanupFactsV2) string {
	if f.DescendantsPresent && !f.Absent && len(f.Failures) == 0 {
		return "survivors"
	}
	if !f.OwnedGroup || !f.DescendantsPresent || !f.Absent || len(f.Failures) > 0 {
		return "unknown"
	}
	return "observed-absent"
}

type errorString string

func (e errorString) Error() string { return string(e) }

// Closed native process fragment shapes.
func freshProcessFragment(n *jsonNode) (pid int64, start string, ok bool) {
	if n == nil || n.kind != 'o' || len(n.obj) != 2 {
		return 0, "", false
	}
	pidNode, startNode := n.obj["pid"], n.obj["start"]
	pid, ok = pidNode.integer()
	if !ok || startNode == nil || startNode.kind != 's' {
		return 0, "", false
	}
	return pid, startNode.str, true
}

func appInstanceFragment(n *jsonNode) (kind, id, generation string, ok bool) {
	if n == nil || n.kind != 'o' || len(n.obj) != 3 {
		return "", "", "", false
	}
	k, i, g := n.obj["kind"], n.obj["id"], n.obj["startGeneration"]
	if k == nil || i == nil || g == nil || k.kind != 's' || i.kind != 's' || g.kind != 's' {
		return "", "", "", false
	}
	return k.str, i.str, g.str, true
}

func descendantFragment(n *jsonNode) (pid, ppid int64, start, state string, ok bool) {
	if n == nil || n.kind != 'o' || len(n.obj) != 4 {
		return 0, 0, "", "", false
	}
	pidNode, parentNode, startNode, stateNode := n.obj["pid"], n.obj["parent_pid"], n.obj["start"], n.obj["state"]
	pid, okPID := pidNode.integer()
	ppid, okParent := parentNode.integer()
	if !okPID || !okParent || startNode == nil || stateNode == nil || startNode.kind != 's' || stateNode.kind != 's' || stateNode.str == "" {
		return 0, 0, "", "", false
	}
	return pid, ppid, startNode.str, stateNode.str, true
}
