// SPDX-License-Identifier: Apache-2.0
package localcompletion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/trace"
)

// The original reviewed set is retained verbatim. The new set binds the seal's
// actual checks; it never changes what the parent-bound reports claimed.
type sealedContinuation struct {
	Binding     dogfoodflow.SealBinding `json:"binding"`
	Prior       reportSet               `json:"prior"`
	PriorReview string                  `json:"priorReview"`
}

func regularSharedMap(root string) bool {
	info, err := os.Lstat(filepath.Join(root, ".corvint/change.cem.json"))
	return err == nil && info.Mode().IsRegular()
}

func (repo *repository) bindSnapshot(ctx context.Context, target string) (snapshot, error) {
	tree, err := repo.git(ctx, "rev-parse", target+"^{tree}")
	if err != nil {
		return snapshot{}, err
	}
	listing, err := repo.git(ctx, "ls-tree", "-rz", target)
	if err != nil {
		return snapshot{}, err
	}
	var included []byte
	for _, row := range bytes.Split(listing, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return snapshot{}, errors.New("invalid-tree-listing")
		}
		if string(parts[1]) == ".corvint/change.cem.json" {
			continue
		}
		included = append(included, row...)
		included = append(included, 0)
	}
	return snapshot{target: target, tree: strings.TrimSpace(string(tree)), content: digest(included), clean: true}, nil
}

// sealedDrift is a condition refusal; artifactDrift adds both digests.
func sealedDrift(label, reason string) error {
	return &SealedDriftError{Artifact: label, Reason: reason}
}

func artifactDrift(label, reason string, pinned artifact) error {
	current := "absent"
	if now, err := artifactFor(pinned.Path); err == nil {
		current = "sha256:" + now.Digest
	} else if !os.IsNotExist(err) {
		current = "unreadable"
	}
	return &SealedDriftError{Artifact: label, Reason: reason, Pinned: "sha256:" + pinned.Digest, Current: current}
}

// coordinationLabels are the fixed public names of the two coordination pins.
var coordinationLabels = []string{"local-outcome.json", "change.ocm-status.json"}

func (repo *repository) coordinationPaths() []string {
	return []string{filepath.Join(repo.auth.GitDir, "corvint/local-outcome.json"), filepath.Join(repo.auth.Root, ".corvint/change.ocm-status.json")}
}

// sealArchiveEquivalent reports whether snap is the exact seal of the
// observation's target: the recorded sealed continuation, re-resolved now as
// the single-parent, byte- and mode-preserving move of the map and nothing
// else (LCP-V0-040), names the observed commit as its evidence target and
// that commit still has the observed tree. Only a check whose frozen plan
// declares allowSealArchiveReuse asks; any other difference reruns it
// (LCP-V0-920).
func (repo *repository) sealArchiveEquivalent(ctx context.Context, saved *state, observed *observation, snap snapshot) bool {
	if saved.Sealed == nil || !snap.clean {
		return false
	}
	binding, err := dogfoodflow.ResolveSealBinding(ctx, repo.auth.Root, snap.target)
	if err != nil || binding != saved.Sealed.Binding || binding.EvidenceTarget != observed.Target {
		return false
	}
	tree, err := repo.git(ctx, "rev-parse", observed.Target+"^{tree}")
	return err == nil && strings.TrimSpace(string(tree)) == observed.Tree
}

func (repo *repository) validateSealed(ctx context.Context, saved *state, snap snapshot) error {
	continuation := saved.Sealed
	if _, err := os.Lstat(filepath.Join(repo.auth.Root, ".corvint/change.cem.json")); !os.IsNotExist(err) {
		return sealedDrift("change.cem.json", "shared-map-present")
	}
	if continuation == nil || saved.AggregateOutcome != nil {
		return sealedDrift("state", "sealed-state-invalid")
	}
	if !snap.clean {
		return sealedDrift("worktree", "worktree-dirty")
	}
	binding, err := dogfoodflow.ResolveSealBinding(ctx, repo.auth.Root, snap.target)
	if err != nil || binding != continuation.Binding {
		return sealedDrift("seal-binding", "seal-binding-changed")
	}
	if err = repo.requireBaseAncestor(ctx, saved.Plan.Base, binding.EvidenceTarget); err != nil {
		return err
	}
	prior := continuation.Prior
	if prior.Target != binding.EvidenceTarget || prior.PlanDigest != saved.PlanDigest || continuation.PriorReview != prior.Digest || len(prior.Bindings) != 3+2*len(saved.Plan.Intents) || len(prior.Reports) != 1+len(saved.Plan.Intents) || len(prior.Observations) != len(saved.Plan.Checks) {
		return sealedDrift("prior-report-set", "prior-report-set-invalid")
	}
	copy := prior
	copy.Digest = ""
	if valueDigest(copy) != prior.Digest {
		return sealedDrift("prior-report-set", "prior-report-set-digest")
	}
	unsealed := *saved
	unsealed.Sealed = nil
	for index, name := range repo.bindingPaths(&unsealed) {
		item := prior.Bindings[index]
		label := fmt.Sprintf("prior-binding-%02d", index)
		if item.Path != name {
			return sealedDrift(label, "binding-path-changed")
		}
		if index == 1 {
			item.Path = filepath.Join(repo.auth.Root, binding.MapPath)
		}
		if !repo.artifactsCurrent([]artifact{item}) {
			return artifactDrift(label, "artifact-changed", item)
		}
	}
	for index, item := range prior.Reports {
		label := fmt.Sprintf("prior-report-%02d", index)
		if item.Path != repo.local(fmt.Sprintf("report-%s-%02d.md", prior.Target, index)) {
			return sealedDrift(label, "report-path-changed")
		}
		if !repo.artifactsCurrent([]artifact{item}) {
			return artifactDrift(label, "artifact-changed", item)
		}
	}
	bound, err := repo.bindSnapshot(ctx, binding.EvidenceTarget)
	if err != nil {
		return err
	}
	for index, check := range saved.Plan.Checks {
		observed := prior.Observations[index]
		found := false
		for _, retained := range saved.Observations {
			if valueDigest(retained) == valueDigest(observed) {
				found = true
				break
			}
		}
		if !found {
			return sealedDrift(fmt.Sprintf("prior-observation-%02d", index), "prior-observation-missing")
		}
		historical := *saved
		historical.Observations = []observation{observed}
		if !repo.checkQualifies(ctx, &historical, check, bound) {
			return sealedDrift(fmt.Sprintf("prior-observation-%02d", index), "prior-observation-unqualified")
		}
	}
	if len(saved.Coordination) > 0 {
		if !repo.coordinationShaped(saved) {
			return sealedDrift("coordination", "coordination-pins-invalid")
		}
		for index, item := range saved.Coordination {
			if !repo.artifactsCurrent([]artifact{item}) {
				return artifactDrift(coordinationLabels[index], "artifact-changed", item)
			}
		}
	}
	return repo.checkCoordinatorContext(ctx, saved, bound, true)
}

func (repo *repository) coordinationShaped(saved *state) bool {
	expected := repo.coordinationPaths()
	if len(saved.Coordination) != len(expected) {
		return false
	}
	for index, name := range expected {
		if saved.Coordination[index].Path != name {
			return false
		}
	}
	return true
}

// closeoutOutcome is the dogfood-record payload a replacement must prove.
type closeoutOutcome struct {
	OK              bool            `json:"ok"`
	Tool            string          `json:"tool"`
	State           string          `json:"state"`
	Base            string          `json:"base"`
	Target          string          `json:"target"`
	Tree            string          `json:"tree"`
	CandidateSHA256 string          `json:"candidate_sha256"`
	AdmittedSHA256  string          `json:"admitted_sha256"`
	Candidates      []string        `json:"candidates"`
	Admitted        []string        `json:"admitted"`
	TraceID         string          `json:"trace_id"`
	Trace           json.RawMessage `json:"trace"`
}

type closeoutTrace struct {
	Revision     string   `json:"revision"`
	TraceID      string   `json:"trace_id"`
	Task         string   `json:"task"`
	ChangedPaths []string `json:"changed_paths"`
	Outcome      string   `json:"outcome"`
}

// acceptCloseoutOutcome returns the new outcome pin when the pinned
// local-outcome.json was replaced by the lane closeout's own dogfood record of
// the sealed evidence target (LCP-V0-041), and nil when the pin is current. It
// only reads; the caller re-pins. Any other replacement, or a changed OCM
// aggregate, refuses sealed-evidence-drift naming the artifact (LCP-V0-042).
func (repo *repository) acceptCloseoutOutcome(ctx context.Context, saved *state) (*artifact, error) {
	if saved.Sealed == nil || len(saved.Coordination) == 0 || !repo.coordinationShaped(saved) {
		return nil, nil
	}
	pinned := saved.Coordination[0]
	if repo.artifactsCurrent([]artifact{pinned}) {
		return nil, nil
	}
	if status := saved.Coordination[1]; !repo.artifactsCurrent([]artifact{status}) {
		return nil, artifactDrift(coordinationLabels[1], "foreign-change", status)
	}
	reject := func(reason string) error { return artifactDrift(coordinationLabels[0], reason, pinned) }
	target := saved.Sealed.Binding.EvidenceTarget
	raw, err := readFile(pinned.Path, maxArtifactBytes)
	if err != nil {
		return nil, reject("replacement-unreadable")
	}
	var record closeoutOutcome
	var recorded closeoutTrace
	if _, err = wire.Parse(raw); err != nil || json.Unmarshal(raw, &record) != nil || json.Unmarshal(record.Trace, &recorded) != nil {
		return nil, reject("replacement-tampered")
	}
	tree, err := repo.git(ctx, "rev-parse", target+"^{tree}")
	if err != nil {
		return nil, err
	}
	if record.Base != saved.Plan.Base || record.Target != target || recorded.Revision != target || record.Tree != strings.TrimSpace(string(tree)) {
		return nil, reject("replacement-retargeted")
	}
	listing, err := repo.git(ctx, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", saved.Plan.Base, target, "--", ".")
	if err != nil {
		return nil, err
	}
	candidates := []string{}
	for _, name := range strings.Split(string(listing), "\x00") {
		if name != "" {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	if !record.OK || record.Tool != "dogfood-record" || record.State != "recorded" || recorded.Outcome != "passed" || record.TraceID == "" || recorded.TraceID != record.TraceID ||
		!slices.Equal(record.Candidates, candidates) || record.CandidateSHA256 != admissionDigest(candidates) || record.AdmittedSHA256 != admissionDigest(record.Admitted) ||
		!slices.Equal(recorded.ChangedPaths, record.Admitted) || !subset(record.Admitted, candidates) {
		return nil, reject("replacement-tampered")
	}
	if reason := repo.closeoutRecorded(saved, target, record); reason != "" {
		return nil, reject(reason)
	}
	if repo.checkCoordinatorContext(ctx, saved, snapshot{target: target}, false) != nil {
		return nil, reject("replacement-unreferenced")
	}
	return &artifact{Path: pinned.Path, Digest: digest(raw)}, nil
}

// closeoutRecorded proves the replacement is the newest trace row at the
// evidence target, appended after the enrollment's own coordination row, and
// that both rows admitted the same changed paths.
func (repo *repository) closeoutRecorded(saved *state, target string, record closeoutOutcome) string {
	raw, err := readFile(trace.StorePath(repo.auth.Root, target), maxArtifactBytes)
	if err != nil {
		return "replacement-unrecorded"
	}
	enrollment := "Local completion " + saved.PlanDigest
	var last []byte
	enrolled := false
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n")) {
		var row closeoutTrace
		if json.Unmarshal(line, &row) != nil {
			return "replacement-unrecorded"
		}
		if row.Task == enrollment && row.Revision == target {
			if !slices.Equal(row.ChangedPaths, record.Admitted) {
				return "admitted-set-mismatch"
			}
			enrolled = true
		}
		last = line
	}
	var newest closeoutTrace
	if !enrolled || json.Unmarshal(last, &newest) != nil || newest.TraceID != record.TraceID || newest.Task == enrollment || !sameJSON(last, record.Trace) {
		return "replacement-unrecorded"
	}
	return ""
}

// admissionDigest mirrors the recorder's path-admission digest.
func admissionDigest(paths []string) string {
	sum := sha256.New()
	sum.Write([]byte("corvint-record-path-admission/v0\x00"))
	for _, name := range paths {
		sum.Write([]byte(name))
		sum.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil))
}

func subset(items, of []string) bool {
	for _, item := range items {
		if !slices.Contains(of, item) {
			return false
		}
	}
	return true
}

func sameJSON(left, right []byte) bool {
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return false
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func (repo *repository) finishSealed(ctx context.Context, saved *state, snap snapshot, command PublicCommand, evaluation Evaluation) (Evaluation, error) {
	if saved.AggregateOutcome != nil {
		return evaluation, errors.New("sealed-aggregate-unsupported")
	}
	if saved.Sealed == nil {
		binding, err := dogfoodflow.ResolveSealBinding(ctx, repo.auth.Root, snap.target)
		if err != nil {
			return evaluation, err
		}
		if saved.ReportSet == nil {
			return evaluation, errors.New("sealed-reviewed-evidence-required")
		}
		saved.Sealed = &sealedContinuation{Binding: binding, Prior: *saved.ReportSet, PriorReview: saved.Review}
	}
	// The lane closeout's own record rewrites the coordinated outcome after
	// enrollment pinned it; re-pin only that proved rewrite, and require a new
	// review of a report set naming both digests (LCP-V0-041).
	replaced, err := repo.acceptCloseoutOutcome(ctx, saved)
	if err != nil {
		return evaluation, err
	}
	var replacedFrom string
	if replaced != nil {
		replacedFrom = saved.Coordination[0].Digest
		saved.Coordination[0] = *replaced
	}
	if err = repo.validateSealed(ctx, saved, snap); err != nil {
		return evaluation, err
	}
	binding := saved.Sealed.Binding
	// Never prepare or rewrite the parent's evidence while recovering a seal.
	aggregate, err := dogfoodocm.Status(ctx, dogfoodocm.Options{Root: repo.auth.Root, CEMPath: binding.MapPath, ExpectedBase: saved.Plan.Base, Target: binding.EvidenceTarget})
	if err != nil || !aggregate.OK {
		return evaluation, errors.New("ocm-bindings-required")
	}
	for _, row := range aggregate.Worklist {
		if row.Disposition == "unknown" && row.Reason != "unassessed" {
			return evaluation, errors.New("ocm-bindings-required")
		}
	}
	bootstrap := 0
	for _, intent := range saved.Plan.Intents {
		_, exists, e := repo.auth.LookupTreeEntry(ctx, saved.Plan.Base, intent)
		if e != nil {
			return evaluation, e
		}
		if !exists {
			bootstrap++
		}
	}
	args := repo.cemReadArguments("status", saved.Plan.Base, binding.EvidenceTarget, bootstrap)
	args[3] = binding.MapPath
	// Stable maps require the artifact root even though the shared path is absent.
	args = append(args, "--artifacts", repo.auth.Root)
	if _, err = repo.public(ctx, command, args); err != nil {
		return evaluation, err
	}
	if replaced != nil || !repo.reportCurrent(ctx, saved, snap) {
		set := &reportSet{PlanDigest: saved.PlanDigest, Target: snap.target, Bindings: []artifact{}, Reports: []artifact{}, Observations: []observation{}}
		for _, name := range repo.bindingPaths(saved) {
			item, e := artifactFor(name)
			if e != nil {
				return evaluation, e
			}
			set.Bindings = append(set.Bindings, item)
		}
		for index, prior := range saved.Sealed.Prior.Reports {
			raw, e := readFile(prior.Path, maxArtifactBytes)
			if e != nil {
				return evaluation, e
			}
			prefix := fmt.Sprintf("# Sealed completion continuation\n\nCompletion target: `%s`\nEvidence target: `%s`\nArchived map: `%s`\nPrior reviewed report set: `%s`\n\nThe report below remains bound to the evidence target. Current selected check observations are bound separately in this report set.\n\n", snap.target, binding.EvidenceTarget, binding.MapPath, saved.Sealed.Prior.Digest)
			if replaced != nil {
				prefix += fmt.Sprintf("The lane closeout's own dogfood record replaced the coordinated outcome `sha256:%s` with `sha256:%s`, proved against the evidence target, the trace store and the coordinator report.\n\n", replacedFrom, replaced.Digest)
			}
			name := repo.local(fmt.Sprintf("report-%s-%02d.md", snap.target, index))
			if e = writeFile(name, append([]byte(prefix), raw...)); e != nil {
				return evaluation, e
			}
			item, e := artifactFor(name)
			if e != nil {
				return evaluation, e
			}
			set.Reports = append(set.Reports, item)
		}
		for _, check := range saved.Plan.Checks {
			set.Observations = append(set.Observations, *repo.latest(saved, check.ID))
		}
		set.Digest = valueDigest(*set)
		saved.ReportSet = set
		saved.Review = ""
		saved.Terminal = nil
		saved.Lifecycle = "active"
		if err = repo.revalidate(ctx, saved, snap); err != nil {
			return evaluation, err
		}
		if err = repo.save(saved); err != nil {
			return evaluation, err
		}
		return repo.evaluate(ctx, saved)
	}
	if saved.Review != saved.ReportSet.Digest {
		return repo.evaluate(ctx, saved)
	}
	if err = repo.revalidate(ctx, saved, snap); err != nil {
		return evaluation, err
	}
	checked := repo.runFlow(ctx, command, func(ctx context.Context, steps dogfoodflow.Runner, stdout, stderr io.Writer) (int, error) {
		return dogfoodflow.CheckSealed(ctx, dogfoodflow.CheckOptions{Root: repo.auth.Root, Base: saved.Plan.Base, BaseVerifier: steps, TreeVerifier: steps}, stdout, stderr)
	})
	if err = repo.saveProcess("final-check", checked); err != nil {
		return evaluation, err
	}
	if !processPassed(checked) {
		return evaluation, repo.finalCheckError(ctx, "final-check", true)
	}
	if err = repo.revalidate(ctx, saved, snap); err != nil {
		return evaluation, err
	}
	artifacts, err := repo.terminalArtifacts()
	if err != nil {
		return evaluation, err
	}
	saved.Terminal = &terminal{ReportSet: saved.ReportSet.Digest, CheckExit: checked.code, Artifacts: artifacts}
	saved.Lifecycle = "satisfied"
	if err = repo.save(saved); err != nil {
		return evaluation, err
	}
	return repo.evaluate(ctx, saved)
}
