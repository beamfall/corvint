package delta

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"regexp"
	"sort"
	"strings"
)

type Options struct {
	Base, Head, Build, PreviousGeneration, WorkKeyPattern string
	Providers                                             []string
	Checkouts                                             []extevidence.Checkout
}

var errArguments = errors.New("delta-invalid-arguments")
var opaqueKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/#-]{0,127}$`)

func Compile(ctx context.Context, root string, o Options) (Record, error) {
	if !wire.IsGitOid(o.Base) || !wire.IsGitOid(o.Head) || !buildID.MatchString(o.Build) || len(o.WorkKeyPattern) > 1024 || len(o.Checkouts) > 32 {
		return Record{}, errArguments
	}
	var pattern *regexp.Regexp
	if o.WorkKeyPattern != "" {
		var err error
		pattern, err = regexp.Compile(o.WorkKeyPattern)
		if err != nil {
			return Record{}, errArguments
		}
	}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return Record{}, errors.New("delta-repository-unavailable")
	}
	release := auth.BeginObjectSession()
	defer release()
	tree, err := auth.CommitTree(ctx, o.Head)
	if err != nil {
		return Record{}, errors.New("delta-head-unavailable")
	}
	changes, err := auth.DeltaPaths(ctx, o.Base, o.Head)
	if err != nil {
		return Record{}, errors.New("delta-change-set-unavailable")
	}
	raw, _ := json.Marshal(changes, json.Deterministic(true))
	r := Record{Schema: Schema, Base: o.Base, Head: o.Head, Tree: tree, Build: o.Build, ChangedPathDigest: digest("paths", raw), ChangedPaths: []string{}, InputDigests: []string{}, Documentation: []Span{}, Tests: []Test{}, Gaps: []Gap{}, WorkKeys: []string{}, Unknowns: []Unknown{}, MandatoryChecksRequired: true, Denominators: Denominators{Runtime: "unknown"}}
	for _, change := range changes {
		if !validPath(change.Path) {
			return Record{}, errors.New("delta-unrepresentable-path")
		}
		r.ChangedPaths = append(r.ChangedPaths, change.Path)
	}
	r.workKeys(ctx, auth, pattern)
	// An empty immutable change set has no change obligations, regardless of
	// provider availability. Optional supplied input still gets captured below.
	if len(changes) == 0 {
		if len(o.Providers) > 0 {
			r.external(ctx, root, o)
		}
		if o.PreviousGeneration != "" {
			r.documentation(ctx, root, o)
		}
		r.finalize()
		return r, nil
	}
	units := map[string]affected.Unit{}
	source, err := auth.RevisionFS(ctx, o.Head, affected.MaxSourceBytes)
	var graph *affected.Graph
	if err == nil {
		graph, err = affected.BuildFS(source, languages.All()...)
	}
	if err != nil {
		r.RunFullSuite = true
		r.unknown("immutable-graph-unavailable", 1, nil)
	} else {
		plan := affected.Select(graph, r.ChangedPaths)
		if plan.Scope != affected.ScopeBounded {
			r.RunFullSuite = true
			b, _ := json.Marshal(plan.Unknown, json.Deterministic(true))
			r.unknown("affected-scope-incomplete", max(1, len(plan.Unknown)), b)
		}
		addUnit := func(id string) {
			if u, ok := graph.Unit(id); ok {
				units[id] = u
			}
		}
		for _, id := range graph.ReachedUnitIDs(r.ChangedPaths) {
			addUnit(id)
		}
		for _, selected := range plan.Selected {
			addUnit(selected.UnitID)
			for _, id := range selected.Witness.Via {
				addUnit(id)
			}
			for _, p := range selected.Tests {
				r.addTest(Test{p, digest("repository", []byte("root")), digest("unit", []byte(selected.UnitID)), "in-repository", "unscored"})
			}
		}
	}
	r.Denominators.AffectedUnits = len(units)
	assertions := r.external(ctx, root, o)
	ids := make([]string, 0, len(units))
	for id := range units {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		unit := units[id]
		covered := true
		touched := false
		for _, p := range append(append([]string{}, unit.Sources...), unit.Tests...) {
			if contains(r.ChangedPaths, p) {
				touched = true
				if !assertions[p] {
					covered = false
				}
			}
		}
		if !covered || !touched {
			r.Gaps = append(r.Gaps, Gap{digest("unit", []byte(id)), "missing-unit-assertion", "affected-units"})
		}
	}
	if len(r.Gaps) > 0 {
		r.unknown("unit-assertion-witness-unavailable", len(r.Gaps), nil)
	}
	r.documentation(ctx, root, o)
	if len(r.Gaps) > MaxObservations*2 {
		r.unknown("observation-bound-exceeded", len(r.Gaps)-MaxObservations*2, nil)
		r.Gaps = r.Gaps[:MaxObservations*2]
		r.RunFullSuite = true
	}
	r.finalize()
	if _, err := r.Canonical(); err != nil {
		return Record{}, err
	}
	return r, nil
}
func contains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}
func (r *Record) addTest(t Test) {
	for _, old := range r.Tests {
		if old == t {
			return
		}
	}
	if len(r.Tests) >= MaxObservations {
		r.unknown("observation-bound-exceeded", 1, nil)
		r.RunFullSuite = true
		return
	}
	r.Tests = append(r.Tests, t)
}
func (r *Record) workKeys(ctx context.Context, auth *gitauth.Repository, pattern *regexp.Regexp) {
	if pattern == nil {
		r.unknown("work-key-pattern-unconfigured", 1, nil)
		return
	}
	message, err := auth.DeltaCommitMessage(ctx, r.Head)
	if err != nil {
		r.unknown("work-key-metadata-unavailable", 1, nil)
		return
	}
	matches := pattern.FindAll(message, MaxObservations+1)
	if len(matches) > MaxObservations {
		r.unknown("observation-bound-exceeded", 1, nil)
		matches = matches[:MaxObservations]
	}
	seen := map[string]bool{}
	for _, match := range matches {
		if !opaqueKey.Match(match) {
			r.unknown("work-key-invalid", 1, match)
			continue
		}
		key := digest("work-key", match)
		if !seen[key] {
			r.WorkKeys = append(r.WorkKeys, key)
			seen[key] = true
		}
	}
}
func (r *Record) external(ctx context.Context, root string, o Options) map[string]bool {
	assertions := map[string]bool{}
	captures := []extevidence.CapturedRecord{}
	total := 0
	if len(o.Providers) == 0 {
		r.RunFullSuite = true
		r.unknown("provider-coverage-missing", 1, nil)
		return assertions
	}
	providers := o.Providers
	if len(providers) > MaxProviders {
		r.RunFullSuite = true
		r.unknown("provider-bound-exceeded", len(providers)-MaxProviders, nil)
		providers = providers[:MaxProviders]
	}
	for _, name := range providers {
		data, err := capture(root, name, MaxProviderBytes)
		if err != nil || len(data) > MaxProviderTotalBytes-total {
			r.RunFullSuite = true
			r.unknown("provider-capture-unavailable", 1, nil)
			captures = append(captures, extevidence.FailedCapture())
			continue
		}
		total += len(data)
		c, err := extevidence.CaptureRecord(data)
		if err != nil {
			r.RunFullSuite = true
			r.unknown("provider-capture-unavailable", 1, nil)
			captures = append(captures, extevidence.FailedCapture())
			continue
		}
		r.InputDigests = append(r.InputDigests, c.Digest())
		captures = append(captures, c)
	}
	input := extevidence.SelectionInput{Changed: r.ChangedPaths, Profile: extevidence.ProfileStrict, Limit: MaxObservations, CheckoutStatus: func(ctx context.Context, dir string) ([]string, error) { return affected.DirtyPaths(ctx, "git", dir) }}
	if r.RunFullSuite {
		input.Incomplete = []string{"delta-incomplete"}
	}
	selection := extevidence.SelectionCaptured(ctx, root, o.Head, captures, o.Checkouts, input)
	result := selection.Selection
	if result["state"] != extevidence.SelectionNarrow {
		r.RunFullSuite = true
		b, _ := json.Marshal(result["blocking_reasons"], json.Deterministic(true))
		r.unknown("external-coverage-incomplete", 1, b)
	}
	if omitted, ok := result["omitted"].(map[string]any); ok {
		for _, v := range omitted {
			if n, ok := v.(int); ok && n > 0 {
				r.RunFullSuite = true
				r.unknown("external-projection-incomplete", n, nil)
			}
		}
	}
	if rows, ok := result["selected"].([]any); ok {
		for _, item := range rows {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			test, ok := row["test"].(map[string]any)
			if !ok {
				continue
			}
			p, _ := test["path"].(string)
			repo, _ := test["repository"].(string)
			if !validPath(p) || repo == "" {
				r.RunFullSuite = true
				r.unknown("external-projection-incomplete", 1, nil)
				continue
			}
			provider, _ := row["provider"].(string)
			r.addTest(Test{p, digest("repository", []byte(repo)), digest("external-unit", []byte(provider+"\x00"+p)), "external", "unscored"})
		}
	}
	for _, a := range selection.Assertions {
		if validPath(a.Subject) && validPath(a.Test) {
			assertions[a.Subject] = true
		}
	}
	return assertions
}

// isDocumentation is deliberately narrow and never overrides missing evidence.
func isDocumentation(path string) bool {
	return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".rst") || strings.HasSuffix(path, ".txt")
}
