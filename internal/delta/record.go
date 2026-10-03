// Package delta joins immutable change evidence without emitting source prose.
package delta

import (
	json "encoding/json/v2"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"regexp"
	"sort"
	"strings"
)

const Schema = "corvint-delta/0"
const MaxObservations = 4096

type Record struct {
	Schema                  string       `json:"schema"`
	Base                    string       `json:"base"`
	Head                    string       `json:"head"`
	Tree                    string       `json:"tree"`
	Build                   string       `json:"build"`
	ChangedPathDigest       string       `json:"changedPathDigest"`
	ChangedPaths            []string     `json:"changedPaths"`
	InputDigests            []string     `json:"inputDigests"`
	Documentation           []Span       `json:"documentation"`
	Tests                   []Test       `json:"tests"`
	RunFullSuite            bool         `json:"runFullSuite"`
	MandatoryChecksRequired bool         `json:"mandatoryChecksRequired"`
	Gaps                    []Gap        `json:"gaps"`
	Denominators            Denominators `json:"denominators"`
	WorkKeys                []string     `json:"workKeys"`
	Unknowns                []Unknown    `json:"unknowns"`
	Decision                string       `json:"decision"`
}
type Span struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	State  string `json:"state"`
	Digest string `json:"digest"`
}
type Test struct {
	Path       string `json:"path"`
	Repository string `json:"repository"`
	Unit       string `json:"unit"`
	Origin     string `json:"origin"`
	Confidence string `json:"confidence"`
}
type Gap struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Denominator string `json:"denominator"`
}
type Denominators struct {
	LexicalFlows  int    `json:"lexicalFlows"`
	AffectedUnits int    `json:"affectedUnits"`
	Runtime       string `json:"runtime"`
	Complete      bool   `json:"complete"`
}
type Unknown struct {
	Code   string `json:"code"`
	Count  int    `json:"count"`
	Digest string `json:"digest"`
}

var unknownCodes = map[string]bool{
	"immutable-graph-unavailable": true, "affected-scope-incomplete": true, "documentation-baseline-unavailable": true, "documentation-baseline-invalid": true, "documentation-incomplete": true, "documentation-stale": true, "flow-assertion-witness-unavailable": true, "unit-assertion-witness-unavailable": true, "provider-coverage-missing": true, "provider-capture-unavailable": true, "provider-bound-exceeded": true, "external-coverage-incomplete": true, "external-projection-incomplete": true, "work-key-pattern-unconfigured": true, "work-key-metadata-unavailable": true, "work-key-invalid": true, "observation-bound-exceeded": true, "runtime-behaviour-unclassified": true,
}
var hexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var buildID = regexp.MustCompile(`^[A-Za-z0-9_.+\-]{1,128}$`)

func (r *Record) unknown(code string, count int, raw []byte) {
	if code == "documentation-incomplete" || code == "observation-bound-exceeded" {
		r.RunFullSuite = true
	}
	for i := range r.Unknowns {
		if r.Unknowns[i].Code == code {
			r.Unknowns[i].Count += count
			r.Unknowns[i].Digest = digest("uncertainty", append([]byte(r.Unknowns[i].Digest), raw...))
			return
		}
	}
	r.Unknowns = append(r.Unknowns, Unknown{code, count, digest("uncertainty", raw)})
}
func (r *Record) finalize() {
	sort.Strings(r.InputDigests)
	sort.Strings(r.WorkKeys)
	sort.Slice(r.Unknowns, func(i, j int) bool { return r.Unknowns[i].Code < r.Unknowns[j].Code })
	sort.Slice(r.Documentation, func(i, j int) bool {
		a, b := r.Documentation[i], r.Documentation[j]
		return a.ID+"\x00"+a.State < b.ID+"\x00"+b.State
	})
	sort.Slice(r.Tests, func(i, j int) bool {
		a, b := r.Tests[i], r.Tests[j]
		return a.Repository+"\x00"+a.Path+"\x00"+a.Unit < b.Repository+"\x00"+b.Path+"\x00"+b.Unit
	})
	sort.Slice(r.Gaps, func(i, j int) bool {
		a, b := r.Gaps[i], r.Gaps[j]
		return a.Denominator+"\x00"+a.ID < b.Denominator+"\x00"+b.ID
	})
	r.Decision = "no-op"
	if len(r.ChangedPaths) > 0 {
		r.Decision = "docs-only"
		for _, p := range r.ChangedPaths {
			if !isDocumentation(p) {
				r.Decision = "tests-needed"
			}
		}
		if r.RunFullSuite || len(r.Tests) > 0 {
			r.Decision = "tests-needed"
		}
	}
	for _, u := range r.Unknowns {
		if u.Code != "work-key-pattern-unconfigured" && u.Code != "runtime-behaviour-unclassified" {
			r.Decision = "findings"
		}
	}
	if len(r.Gaps) > 0 {
		r.Decision = "findings"
	}
	r.Denominators.Complete = len(r.Unknowns) == 0
}
func (r Record) Canonical() ([]byte, error) {
	bad := errors.New("delta-record-invalid")
	if r.Schema != Schema || !wire.IsGitOid(r.Base) || !wire.IsGitOid(r.Head) || !wire.IsGitOid(r.Tree) || !buildID.MatchString(r.Build) || !hexDigest.MatchString(r.ChangedPathDigest) {
		return nil, bad
	}
	if !oneOf(r.Decision, "no-op", "docs-only", "tests-needed", "findings") || r.Denominators.Runtime != "unknown" || r.Denominators.AffectedUnits < 0 || r.Denominators.LexicalFlows < 0 {
		return nil, bad
	}
	if len(r.Tests) > MaxObservations || len(r.Documentation) > MaxObservations || len(r.Gaps) > MaxObservations*2 {
		return nil, bad
	}
	for _, p := range r.ChangedPaths {
		if !validPath(p) {
			return nil, bad
		}
	}
	for _, d := range append(append([]string{}, r.InputDigests...), r.WorkKeys...) {
		if !hexDigest.MatchString(d) {
			return nil, bad
		}
	}
	for _, s := range r.Documentation {
		if !hexDigest.MatchString(s.ID) || !hexDigest.MatchString(s.Digest) || (s.Path != "" && !validPath(s.Path)) || !oneOf(s.State, "unchanged", "stale", "retired", "added", "unknown") {
			return nil, bad
		}
	}
	for _, t := range r.Tests {
		if !validPath(t.Path) || !hexDigest.MatchString(t.Repository) || !hexDigest.MatchString(t.Unit) || !oneOf(t.Origin, "in-repository", "external") || t.Confidence != "unscored" {
			return nil, bad
		}
	}
	for _, g := range r.Gaps {
		if !hexDigest.MatchString(g.ID) || !oneOf(g.Kind, "missing-flow-assertion", "missing-unit-assertion") || !oneOf(g.Denominator, "lexical-flows", "affected-units") {
			return nil, bad
		}
	}
	for _, u := range r.Unknowns {
		if !unknownCodes[u.Code] || u.Count < 1 || !hexDigest.MatchString(u.Digest) {
			return nil, bad
		}
	}
	return json.Marshal(r, json.Deterministic(true))
}
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

// Delta paths are canonical UTF-8 repository paths without control characters.
// Keep this admission identical to protocol/delta/schema.json's path grammar.
func validPath(p string) bool {
	return affected.ValidRelativePath(p) && strings.IndexFunc(p, func(c rune) bool { return c < 32 || c == 127 }) < 0
}
