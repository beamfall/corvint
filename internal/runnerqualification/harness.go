// SPDX-License-Identifier: Apache-2.0

package runnerqualification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func hexValue(s string, size int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == size && strings.ToLower(s) == s
}

func validText(s string) bool {
	return s != "" && len(s) <= 4096 && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}
func knownClass(c runnerselection.Class) bool {
	switch c {
	case runnerselection.BodyEdit, runnerselection.DeclarationEdit, runnerselection.DataEdit, runnerselection.Add, runnerselection.DeleteRename, runnerselection.ExecTarget, runnerselection.RunnerConfig, runnerselection.E2EServer:
		return true
	}
	return false
}
func validIdentity(i Identity) bool {
	for _, s := range []string{i.Runner.Plugin, i.Runner.PluginVersion, i.Runner.RunnerVersion, i.Execution.OS, i.Execution.Architecture, i.Execution.TracingBackend, i.Execution.TracingVersion, i.SelectorVersion} {
		if !validText(s) {
			return false
		}
	}
	return hexValue(i.Runner.ExecutableDigest, 32) && hexValue(i.Runner.ConfigurationDigest, 32) && hexValue(i.Execution.EnvironmentDigest, 32) && hexValue(i.NonSemanticDigest, 32) && hexValue(i.VerifierRevision, 20) && i.RecordFormat == testevidence.Format
}
func validate(d Declaration) error {
	if !validText(d.Runner) || !validText(d.Corpus.Repository) || !hexValue(d.Corpus.Commit, 20) || !validIdentity(d.Identity) {
		return fmt.Errorf("qualification-identity-invalid")
	}
	if d.Model != "traced" {
		return fmt.Errorf("execution-model-unsupported:%s", d.Model)
	}
	if len(d.Classes) == 0 || len(d.Classes) > 8 {
		return fmt.Errorf("qualification-classes-empty-or-capped")
	}
	seen := map[runnerselection.Class]bool{}
	for _, c := range d.Classes {
		if !knownClass(c) || seen[c] {
			return fmt.Errorf("qualification-class-invalid:%s", c)
		}
		seen[c] = true
	}
	pieces := map[string]bool{}
	for _, p := range d.Mutation {
		if !validText(p.Name) || p.Name == "core-file" || pieces[p.Name] || len(p.Paths) == 0 || len(p.Paths) > MaxPopulation {
			return fmt.Errorf("mutation-declaration-invalid")
		}
		pieces[p.Name] = true
	}
	return nil
}

func inspect(d Declaration, commit string, o Observation) (map[string]testrunner.RunnerResult, map[string]bool, error) {
	key := testrunner.InventoryKey{Checkout: commit, Runner: d.Runner, ConfigDigest: d.Identity.Runner.ConfigurationDigest, OS: d.Identity.Execution.OS, Arch: d.Identity.Execution.Architecture}
	inv, err := testrunner.CanonicalInventory(o.Inventory, key)
	if err != nil {
		return nil, nil, err
	}
	if len(inv.Tests) == 0 {
		return nil, nil, fmt.Errorf("inventory-empty")
	}
	r := testrunner.ReconcileInventory(inv, key, testrunner.RunSet{Full: true}, o.Results, false)
	if r.NeedsRequalification {
		return nil, nil, fmt.Errorf("full-results-incomplete:%s", strings.Join(r.Reasons, ","))
	}
	if !o.TimingsObserved || o.InventoryWall < 0 || o.CollectionWall < 0 {
		return nil, nil, fmt.Errorf("collection-or-inventory-time-unobserved")
	}
	known, selected := map[string]bool{}, map[string]bool{}
	for _, t := range inv.Tests {
		known[t.ID] = true
		if o.Plan.Full {
			selected[t.ID] = true
		}
	}
	if len(o.Plan.Tests) > testrunner.MaxTests {
		return nil, nil, fmt.Errorf("selected-set-capped")
	}
	seen := map[string]bool{}
	for _, id := range o.Plan.Tests {
		if !known[id] || seen[id] {
			return nil, nil, fmt.Errorf("selected-set-invalid:%s", id)
		}
		seen[id] = true
		selected[id] = true
	}
	results := map[string]testrunner.RunnerResult{}
	for _, v := range o.Results {
		results[v.ID] = v
	}
	return results, selected, nil
}

// Run draws independently and uniformly WITH replacement from the entire
// enumerated population. A repeated draw is a new experiment, not a deduped
// trial. Every failure and unobserved stage keeps its class unqualified.
func Run(ctx context.Context, req Request, driver Driver) (out Record) {
	start := time.Now()
	out.Declaration = req.Declaration
	out.Merges = append([]Merge{}, req.Merges...)
	out.Budget = req.Budget
	defer func() { out.Wall = time.Since(start) }()
	fail := func(err error) Record { out.Reasons = append(out.Reasons, err.Error()); return out }
	if err := validate(req.Declaration); err != nil {
		return fail(err)
	}
	if driver == nil || req.Checkout == "" || req.Samples <= 0 || req.Samples > MaxSamples || req.Budget <= 0 || len(req.Merges) == 0 || len(req.Merges) > MaxPopulation {
		return fail(fmt.Errorf("qualification-input-missing-or-capped"))
	}
	d := req.Declaration
	for _, m := range req.Merges {
		if !hexValue(m.Commit, 20) || !hexValue(m.Base, 20) || !hexValue(m.Head, 20) || m.Head != m.Commit || m.Base == m.Head || len(m.Classes) == 0 || len(m.Classes) > 8 {
			return fail(fmt.Errorf("merge-pin-invalid"))
		}
		seen := map[runnerselection.Class]bool{}
		for _, c := range m.Classes {
			if !knownClass(c) || seen[c] {
				return fail(fmt.Errorf("merge-class-invalid"))
			}
			seen[c] = true
		}
	}
	ctx, cancel := context.WithTimeout(ctx, req.Budget)
	defer cancel()
	if err := driver.Verify(ctx, d, req.Checkout, req.Merges); err != nil {
		return fail(fmt.Errorf("corpus-verification:%w", err))
	}
	capture := func(e Experiment, label string) (Observation, error) {
		if err := ctx.Err(); err != nil {
			return Observation{}, fmt.Errorf("qualification-budget-or-cancellation:%w", err)
		}
		pair, err := driver.Capture(ctx, d, req.Checkout, e)
		if err != nil {
			return Observation{}, fmt.Errorf("capture:%w", err)
		}
		if err = ctx.Err(); err != nil {
			return Observation{}, fmt.Errorf("qualification-budget-or-cancellation:%w", err)
		}
		a, _, err := inspect(d, e.Commit, pair.Traced)
		if err != nil {
			return Observation{}, err
		}
		b, _, err := inspect(d, e.Commit, pair.Untraced)
		if err != nil {
			return Observation{}, err
		}
		if !reflect.DeepEqual(a, b) {
			return Observation{}, fmt.Errorf("tracing-outcome-mismatch")
		}
		for model, o := range map[string]Observation{"traced": pair.Traced, "untraced": pair.Untraced} {
			out.Timings = append(out.Timings, Timing{label, model, o.InventoryWall, o.CollectionWall})
		}
		return pair.Traced, nil
	}
	baseline, err := capture(Experiment{Commit: d.Corpus.Commit}, "baseline")
	if err != nil {
		return fail(err)
	}
	baseResults, baseSelected, _ := inspect(d, d.Corpus.Commit, baseline)
	for _, r := range baseResults {
		if r.Outcome == "failed" {
			return fail(fmt.Errorf("baseline-failed:%s", r.ID))
		}
	}
	classes := append([]runnerselection.Class{}, d.Classes...)
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })
	for _, class := range classes {
		cr := ClassRecord{Class: class, Seed: req.Seed, Upper95: 1}
		block := func(err error) { cr.Reasons = append(cr.Reasons, err.Error()) }
		control, e := capture(Experiment{Commit: d.Corpus.Commit, Control: class}, "control:"+string(class))
		if e != nil {
			block(e)
		} else {
			results, selected, _ := inspect(d, d.Corpus.Commit, control)
			outcomesEqual := reflect.DeepEqual(baseResults, results)
			plansEqual := reflect.DeepEqual(baseSelected, selected)
			cr.ControlPassed = outcomesEqual && plansEqual
			if !outcomesEqual {
				block(fmt.Errorf("negative-control-outcome-differs"))
			}
			if !plansEqual {
				block(fmt.Errorf("negative-control-plan-differs"))
			}
		}
		replayed := 0
		replayOK := true
		for _, m := range req.Merges {
			has := false
			for _, c := range m.Classes {
				has = has || c == class
			}
			if !has {
				continue
			}
			replayed++
			a, e := capture(Experiment{Commit: m.Base}, "replay-base:"+m.Commit)
			if e != nil {
				block(e)
				replayOK = false
				continue
			}
			b, e := capture(Experiment{Base: m.Base, Commit: m.Head}, "replay-head:"+m.Commit)
			if e != nil {
				block(e)
				replayOK = false
				continue
			}
			ar, _, _ := inspect(d, m.Base, a)
			br, selected, _ := inspect(d, m.Head, b)
			var missed []string
			for id, r := range br {
				if old, ok := ar[id]; (!ok || old.Outcome != r.Outcome || old.Reason != r.Reason) && !selected[id] {
					missed = append(missed, id)
				}
			}
			if len(missed) > 0 {
				sort.Strings(missed)
				cr.Misses = append(cr.Misses, Miss{"replay:" + m.Commit, missed})
				replayOK = false
				block(fmt.Errorf("replay-miss"))
			}
			if class == runnerselection.RunnerConfig && !b.Plan.Full {
				replayOK = false
				block(fmt.Errorf("configuration-not-full"))
			}
		}
		cr.ReplayPassed = replayed > 0 && replayOK
		if replayed == 0 {
			block(fmt.Errorf("class-replay-missing"))
		}
		p, e := driver.Population(ctx, d, req.Checkout, class)
		if e == nil {
			e = ValidatePopulation(d, class, p)
		}
		if e != nil {
			block(e)
		} else {
			cr.PopulationSize = len(p.Mutants)
			// Canonicalize before the seeded draw; producer enumeration order must
			// not affect the population or choose a favourable sample.
			population := append([]Mutant{}, p.Mutants...)
			sort.Slice(population, func(i, j int) bool { return population[i].ID < population[j].ID })
			encoded, encodeErr := json.Marshal(population)
			if encodeErr != nil {
				block(fmt.Errorf("population-encoding:%w", encodeErr))
				out.Classes = append(out.Classes, cr)
				continue
			}
			digest := sha256.Sum256(encoded)
			cr.PopulationDigest = hex.EncodeToString(digest[:])
			rng := rand.New(rand.NewSource(req.Seed))
			trialMisses := 0
			for draw := 0; draw < req.Samples; draw++ {
				m := population[rng.Intn(len(population))]
				label := fmt.Sprintf("%s:%d:%s", class, draw, m.ID)
				o, e := capture(Experiment{Commit: d.Corpus.Commit, Mutant: &m}, label)
				if e != nil {
					block(e)
					break
				}
				cr.Sampled++
				cr.Draws = append(cr.Draws, m.ID)
				results, selected, _ := inspect(d, d.Corpus.Commit, o)
				cr.SelectedSizes = append(cr.SelectedSizes, len(selected))
				if class == runnerselection.RunnerConfig && !o.Plan.Full {
					block(fmt.Errorf("configuration-not-full"))
				}
				killed := false
				var missed []string
				for id, r := range results {
					if r.Outcome == "failed" {
						killed = true
						if !selected[id] {
							missed = append(missed, id)
						}
					}
				}
				if !killed {
					cr.Unkilled++
					continue
				}
				cr.Trials++
				if len(missed) > 0 {
					trialMisses++
					sort.Strings(missed)
					cr.Misses = append(cr.Misses, Miss{label, missed})
				}
			}
			cr.Upper95 = Upper95(trialMisses, cr.Trials)
			if trialMisses > 0 {
				block(fmt.Errorf("sampled-mutant-miss"))
			}
		}
		if cr.Trials < TrialFloor {
			block(fmt.Errorf("trial-floor:%d/%d", cr.Trials, TrialFloor))
		}
		cr.Passed = len(cr.Reasons) == 0 && cr.ControlPassed && cr.ReplayPassed && len(cr.Misses) == 0
		out.Classes = append(out.Classes, cr)
	}
	sort.Slice(out.Timings, func(i, j int) bool {
		a, b := out.Timings[i], out.Timings[j]
		if a.Experiment != b.Experiment {
			return a.Experiment < b.Experiment
		}
		return a.Model < b.Model
	})
	out.Complete = ctx.Err() == nil
	if !out.Complete {
		out.Reasons = append(out.Reasons, "qualification-budget-or-cancellation")
	}
	return out
}
