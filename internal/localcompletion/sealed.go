// SPDX-License-Identifier: Apache-2.0
package localcompletion

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/dogfoodocm"
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

func (repo *repository) validateSealed(ctx context.Context, saved *state, snap snapshot) error {
	fail := errors.New("sealed-evidence-drift")
	continuation := saved.Sealed
	if _, err := os.Lstat(filepath.Join(repo.auth.Root, ".corvint/change.cem.json")); !os.IsNotExist(err) {
		return fail
	}
	if continuation == nil || saved.AggregateOutcome != nil || !snap.clean {
		return fail
	}
	binding, err := dogfoodflow.ResolveSealBinding(ctx, repo.auth.Root, snap.target)
	if err != nil || binding != continuation.Binding {
		return fail
	}
	if err = repo.requireBaseAncestor(ctx, saved.Plan.Base, binding.EvidenceTarget); err != nil {
		return err
	}
	prior := continuation.Prior
	if prior.Target != binding.EvidenceTarget || prior.PlanDigest != saved.PlanDigest || continuation.PriorReview != prior.Digest || len(prior.Bindings) != 3+2*len(saved.Plan.Intents) || len(prior.Reports) != 1+len(saved.Plan.Intents) || len(prior.Observations) != len(saved.Plan.Checks) {
		return fail
	}
	copy := prior
	copy.Digest = ""
	if valueDigest(copy) != prior.Digest {
		return fail
	}
	unsealed := *saved
	unsealed.Sealed = nil
	for index, name := range repo.bindingPaths(&unsealed) {
		item := prior.Bindings[index]
		if item.Path != name {
			return fail
		}
		if index == 1 {
			item.Path = filepath.Join(repo.auth.Root, binding.MapPath)
		}
		if !repo.artifactsCurrent([]artifact{item}) {
			return fail
		}
	}
	for index, item := range prior.Reports {
		if item.Path != repo.local(fmt.Sprintf("report-%s-%02d.md", prior.Target, index)) || !repo.artifactsCurrent([]artifact{item}) {
			return fail
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
			return fail
		}
		historical := *saved
		historical.Observations = []observation{observed}
		if !repo.checkQualifies(ctx, &historical, check, bound) {
			return fail
		}
	}
	if len(saved.Coordination) > 0 {
		expected := []string{filepath.Join(repo.auth.GitDir, "corvint/local-outcome.json"), filepath.Join(repo.auth.Root, ".corvint/change.ocm-status.json")}
		if len(saved.Coordination) != len(expected) {
			return fail
		}
		for index, name := range expected {
			if saved.Coordination[index].Path != name {
				return fail
			}
		}
		if !repo.artifactsCurrent(saved.Coordination) {
			return fail
		}
	}
	return repo.checkCoordinatorContext(ctx, saved, bound, true)
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
	if err := repo.validateSealed(ctx, saved, snap); err != nil {
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
	if !repo.reportCurrent(ctx, saved, snap) {
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
		return evaluation, errors.New("final-check-failed")
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
