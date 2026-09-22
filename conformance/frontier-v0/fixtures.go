// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Fixtures are the adversarial half of this suite: one directory per row of the
// "Conformance and adversarial matrix" in docs/specs/change-frontier-v0.md.
//
// Each fixture is DATA. It declares the universe to construct in a closed
// vocabulary and the exact outcome the clause requires, including — for every
// operational failure — the byte-exact CF-V0-022 stderr envelope and its
// digest. Nothing here was observed from a runtime; every expectation is read
// off the clause.

// Fixture is one matrix row.
type Fixture struct {
	// Dir is the fixture directory, filled in by LoadFixtures.
	Dir string `json:"-"`

	ID         string   `json:"id"`
	MatrixRows []string `json:"matrixRows"`
	Clauses    []string `json:"clauses"`
	Intent     string   `json:"intent"`
	Cases      []Case   `json:"cases"`
}

// Case is one variant within a fixture. A row such as "each CEM unknown reason"
// is one fixture with three cases.
type Case struct {
	ID       string   `json:"id"`
	Note     string   `json:"note"`
	Declared Declared `json:"declared"`
	Expect   Expect   `json:"expect"`
	// Assertions are clause-anchored obligations the binding must check on the
	// produced output. They are prose because CF-V0-028 requires the
	// *expectation* to be authored from the clause; the binding turns each into
	// a check against whatever shape the implementation lands.
	Assertions []string `json:"assertions"`
	// ForbiddenSubstrings are CF-V0-024 canaries: none may appear in valid or
	// error output.
	ForbiddenSubstrings []string `json:"forbiddenSubstrings,omitempty"`
}

// Declared is the universe to construct, in a closed vocabulary. Every field is
// validated against its allowed set, so a fixture typo fails today rather than
// silently weakening a case.
type Declared struct {
	CEMProfile   string            `json:"cemProfile"`
	OCMProfile   string            `json:"ocmProfile"`
	ObjectFormat string            `json:"objectFormat"`
	ExpectedBase string            `json:"expectedBase"`
	Target       string            `json:"target"`
	Sidecar      string            `json:"sidecar"`
	DynamicTuple string            `json:"dynamicTuple"`
	LRF          string            `json:"lrf"`
	TCQ          string            `json:"tcq"`
	Injected     string            `json:"injected,omitempty"`
	Repeat       string            `json:"repeat,omitempty"`
	Hunks        []DeclaredHunk    `json:"hunks,omitempty"`
	Obligations  []DeclaredOblig   `json:"obligations,omitempty"`
	Counts       map[string]string `json:"counts,omitempty"`
	Canaries     []Canary          `json:"canaries,omitempty"`
}

// DeclaredHunk is one CEM hunk in the declared patch.
type DeclaredHunk struct {
	ID          string   `json:"id"`
	Disposition string   `json:"disposition"`
	Reason      string   `json:"reason,omitempty"`
	Bases       []string `json:"bases,omitempty"`
}

// DeclaredOblig is one OCM intent obligation.
type DeclaredOblig struct {
	ID          string   `json:"id"`
	Disposition string   `json:"disposition"`
	Reason      string   `json:"reason,omitempty"`
	HunkIDs     []string `json:"hunkIds,omitempty"`
	ClaimIDs    []string `json:"claimIds,omitempty"`
	TCQReasons  []string `json:"tcqReasons,omitempty"`
	TCQRelation string   `json:"tcqRelation,omitempty"`
	// ClaimSource is the language and grammar level of the document this
	// obligation's claims are extracted from. It is a property of the DECLARED
	// universe, not of the outcome: `python-post-3.9` names source the frozen
	// `python-ast/1` grammar (TCQ-V0-013) does not admit and the Go host's own
	// Python grammar accepts, which is the only way to place a Python edge that
	// the approximate grammar would otherwise qualify. `python-crlf` names the
	// opposite edge: frozen Python and CPython admit it while the approximation
	// rejects its CR bytes.
	ClaimSource string `json:"claimSource,omitempty"`
}

// Canary is one CF-V0-024 privacy canary: a distinctive string planted in a
// named body that MUST NOT reach any output.
type Canary struct {
	Where string `json:"where"`
	Value string `json:"value"`
}

// Expect is the required outcome.
type Expect struct {
	// Kind is "result" for a valid Frontier document and "error" for an
	// operational failure.
	Kind string `json:"kind"`
	// ExitCode: 0 valid empty, 1 valid open, 2 operational failure (CF-V0-004).
	ExitCode int `json:"exitCode"`
	// Stdout is "frontier-json" or "none".
	Stdout string `json:"stdout"`
	// FrontierState is "EMPTY" or "OPEN" for a result, empty for an error.
	FrontierState string `json:"frontierState,omitempty"`
	// ItemCount is the exact number of emitted items as a CF-V0-019 base-10
	// string. It is required for a result. Items below is a representative
	// sample, not necessarily the whole projection: a limit fixture at N=2048
	// declares the count and samples the first and last item rather than
	// spelling out 2,048 objects.
	ItemCount string `json:"itemCount,omitempty"`
	// ErrorCode is the CF-V0-022 Frontier code for an error.
	ErrorCode string `json:"errorCode,omitempty"`
	// Stderr is the byte-exact CF-V0-022 envelope including its single LF.
	Stderr string `json:"stderr,omitempty"`
	// StderrSha256 is SHA-256 over exactly those bytes.
	StderrSha256 string `json:"stderrSha256,omitempty"`
	// Items describes the required item projection for a result.
	Items []ExpectItem `json:"items,omitempty"`
}

// ExpectItem is the required shape of one emitted frontier item.
type ExpectItem struct {
	Kind            string   `json:"kind"`
	SubjectID       string   `json:"subjectId"`
	Reasons         []string `json:"reasons"`
	RelatedIDs      []string `json:"relatedIds"`
	AuthorityClass  string   `json:"authorityClass"`
	ResolutionClass string   `json:"resolutionClass"`
	NextAction      string   `json:"nextAction"`
}

// Closed vocabularies. A value outside its set is a fixture defect.
var (
	cemProfiles   = set("cem/0.2", "cem/0.1")
	ocmProfiles   = set("ocm/0.1-experimental", "ocm/0.1-bound-to-cem-0.1")
	objectFormats = set("sha1", "sha256")
	revisionState = set("matching", "missing", "mismatched")
	sidecarState  = set("canonical", "forged", "absent")
	tupleState    = set("absent", "complete", "command-only", "observation-only", "report-only",
		"command+observation", "command+report", "observation+report")
	lrfState = set("normal", "aggregate-relevance-bound-exceeded", "unsupported-lrf-context")
	tcqState = set("normal", "resource-exhausted", "forged-authority", "unknown-relation",
		"invalid-junit", "validation-error")
	injectedState = set("", "timeout", "memory-error", "interruption", "unknown-upstream-code",
		"uncatchable-termination")
	repeatState = set("", "reordered-caller-arrays", "two-fresh-processes")

	hunkDispositions  = set("supported", "unknown", "whitespace-only", "line-ending-only", "deletion")
	obligDispositions = set("linked", "unknown")
	claimSources      = set("", "go", "python-post-3.9", "python-crlf")

	// CF-V0-017 closed enumerations.
	authorityClasses  = set("NONE", "PRODUCER_DECLARED", "CALLER_REPORTED")
	resolutionClasses = set("ACTIONABLE", "AUTHORITY_REQUIRED", "PROFILE_REQUIRED")
	itemKinds         = set("HUNK_BASIS", "INTENT_CHANGE", "INTENT_TEST")

	// CF-V0-022 closed Frontier code set. An unknown future code cannot pass
	// through, so this set is exhaustive by construction.
	frontierCodes = set(
		"frontier-resource-exhausted",
		"frontier-interrupted",
		"invalid-frontier-input",
		"unsupported-frontier-context",
		"noncanonical-frontier",
		"frontier-internal-error",
	)

	// inheritedCodes is the closed allowlist CF-V0-022 demands: "The
	// implementation MUST maintain closed allowlists for the exact pinned
	// upstream profile versions; an unknown future code cannot pass through."
	// The CEM/OCM entries are the CEM-CB-021 stable set; the TCQ entries are the
	// TCQ-V0-042 set restricted to explicit public non-resource validation
	// errors, which is exactly what the CF-V0-022 row admits. Prefix matching is
	// deliberately not used: CF-V0-022 says translation "has no prefix or
	// message matching".
	inheritedCodes = set(
		// CEM-CB-021 stable WP2 codes and the retained existing codes.
		"invalid-excluded-path", "excluded-path-not-file", "excluded-artifact-mismatch",
		"expected-base-required", "target-required", "invalid-arguments", "unsupported-spec",
		"missing-field", "patch-digest-mismatch", "base-revision-mismatch",
		"unsupported-object-alternates", "repository-object-unavailable",
		"git-diff-failed", "git-diff-timeout",
		// TCQ-V0-042 public non-resource validation errors.
		"invalid-tcq-input", "unsupported-cem-profile", "unsupported-ocm-profile",
		"unsupported-claim-extractor", "invalid-command", "noncanonical-command",
		"command-target-mismatch", "command-cwd-unavailable", "invalid-observation",
		"noncanonical-observation", "observation-target-mismatch", "observation-command-mismatch",
		"report-digest-mismatch", "invalid-junit", "report-command-inconsistent", "invalid-tcq",
		"noncanonical-tcq", "invalid-target-revision", "target-mismatch",
		// LRF.
		"unsupported-lrf-context",
	)

	// canaryLocations are the exact bodies CF-V0-024 names.
	canaryLocations = set("source-body", "diff-body", "command-body", "report-body",
		"sibling-path", "exception-text")
)

func set(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// ErrorEnvelope is the exact CF-V0-022 stderr document for a code: one bounded
// object plus one LF. `code` sorts before `profile` under the CF-V0-019 raw-byte
// key order.
func ErrorEnvelope(code string) string {
	return `{"code":"` + code + `","profile":"frontier-error/0"}` + "\n"
}

// Validate checks a fixture against the closed vocabularies and the clause
// invariants that hold regardless of implementation.
func (f Fixture) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("fixture has no id")
	}
	if len(f.MatrixRows) == 0 {
		return fmt.Errorf("%s: declares no matrix row", f.ID)
	}
	if len(f.Clauses) == 0 {
		return fmt.Errorf("%s: cites no clause", f.ID)
	}
	if len(f.Cases) == 0 {
		return fmt.Errorf("%s: has no case", f.ID)
	}
	seen := map[string]bool{}
	for _, c := range f.Cases {
		if c.ID == "" {
			return fmt.Errorf("%s: a case has no id", f.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("%s: duplicate case id %q", f.ID, c.ID)
		}
		seen[c.ID] = true
		if len(c.Assertions) == 0 {
			return fmt.Errorf("%s/%s: no assertion", f.ID, c.ID)
		}
		if err := c.Declared.validate(f.ID + "/" + c.ID); err != nil {
			return err
		}
		if err := c.Expect.validate(f.ID + "/" + c.ID); err != nil {
			return err
		}
	}
	return nil
}

func (d Declared) validate(where string) error {
	checks := []struct {
		field string
		val   string
		allow map[string]bool
	}{
		{"cemProfile", d.CEMProfile, cemProfiles},
		{"ocmProfile", d.OCMProfile, ocmProfiles},
		{"objectFormat", d.ObjectFormat, objectFormats},
		{"expectedBase", d.ExpectedBase, revisionState},
		{"target", d.Target, revisionState},
		{"sidecar", d.Sidecar, sidecarState},
		{"dynamicTuple", d.DynamicTuple, tupleState},
		{"lrf", d.LRF, lrfState},
		{"tcq", d.TCQ, tcqState},
		{"injected", d.Injected, injectedState},
		{"repeat", d.Repeat, repeatState},
	}
	for _, c := range checks {
		if !c.allow[c.val] {
			return fmt.Errorf("%s: declared.%s = %q is outside its closed vocabulary", where, c.field, c.val)
		}
	}
	for _, h := range d.Hunks {
		if !hunkDispositions[h.Disposition] {
			return fmt.Errorf("%s: hunk %s disposition %q is outside its closed vocabulary", where, h.ID, h.Disposition)
		}
	}
	for _, o := range d.Obligations {
		if !obligDispositions[o.Disposition] {
			return fmt.Errorf("%s: obligation %s disposition %q is outside its closed vocabulary", where, o.ID, o.Disposition)
		}
		if !claimSources[o.ClaimSource] {
			return fmt.Errorf("%s: obligation %s claimSource %q is outside its closed vocabulary", where, o.ID, o.ClaimSource)
		}
	}
	for _, c := range d.Canaries {
		if !canaryLocations[c.Where] {
			return fmt.Errorf("%s: canary location %q is not a CF-V0-024 body", where, c.Where)
		}
		if c.Value == "" {
			return fmt.Errorf("%s: canary in %s has no value", where, c.Where)
		}
	}
	return nil
}

func (e Expect) validate(where string) error {
	switch e.Kind {
	case "result":
		// CF-V0-004: exit 0 means valid empty, exit 1 means valid open.
		if e.Stdout != "frontier-json" {
			return fmt.Errorf("%s: a result must write frontier JSON to stdout", where)
		}
		n, err := decimalCount(e.ItemCount)
		if err != nil {
			return fmt.Errorf("%s: itemCount: %w", where, err)
		}
		switch e.FrontierState {
		case "EMPTY":
			if e.ExitCode != 0 {
				return fmt.Errorf("%s: CF-V0-004 requires exit 0 for EMPTY", where)
			}
			if n != 0 || len(e.Items) != 0 {
				return fmt.Errorf("%s: CF-V0-004 requires exact items: [] for EMPTY", where)
			}
		case "OPEN":
			if e.ExitCode != 1 {
				return fmt.Errorf("%s: CF-V0-004 requires exit 1 for OPEN", where)
			}
			if n == 0 {
				return fmt.Errorf("%s: CF-V0-004 requires at least one item for OPEN", where)
			}
			if len(e.Items) > n {
				return fmt.Errorf("%s: %d sampled items exceed the declared itemCount %d", where, len(e.Items), n)
			}
			if n > LimitTotalItems {
				return fmt.Errorf("%s: itemCount %d exceeds the CF-V0-023 total-item limit, so this cannot be a complete result", where, n)
			}
		default:
			return fmt.Errorf("%s: frontierState must be EMPTY or OPEN, got %q", where, e.FrontierState)
		}
		if e.ErrorCode != "" || e.Stderr != "" {
			return fmt.Errorf("%s: a valid result declares no error code or stderr envelope", where)
		}
		for _, it := range e.Items {
			if err := it.validate(where); err != nil {
				return err
			}
		}
	case "error":
		// CF-V0-022: no stdout, exit 2, exactly one bounded stderr object.
		if e.ExitCode != 2 {
			return fmt.Errorf("%s: CF-V0-022 requires exit 2 for an operational failure", where)
		}
		if e.Stdout != "none" {
			return fmt.Errorf("%s: CF-V0-022 forbids stdout on operational failure", where)
		}
		if e.FrontierState != "" || len(e.Items) != 0 {
			return fmt.Errorf("%s: an operational failure emits no frontier result", where)
		}
		if e.ItemCount != "" {
			return fmt.Errorf("%s: an operational failure emits no item count", where)
		}
		if e.ErrorCode == "" {
			return fmt.Errorf("%s: no error code", where)
		}
		// `relevance-bound-exceeded` is an LRF *result issue*. CF-V0-022 says it
		// MUST NOT appear in a Frontier error envelope.
		if e.ErrorCode == "relevance-bound-exceeded" {
			return fmt.Errorf("%s: CF-V0-022 forbids relevance-bound-exceeded as an operational code", where)
		}
		if !frontierCodes[e.ErrorCode] && !inheritedCodes[e.ErrorCode] {
			return fmt.Errorf("%s: %q is neither a Frontier-owned code nor an inherited upstream code", where, e.ErrorCode)
		}
		want := ErrorEnvelope(e.ErrorCode)
		if e.Stderr != want {
			return fmt.Errorf("%s: stderr envelope is not the exact CF-V0-022 bytes\n  want %q\n  got  %q", where, want, e.Stderr)
		}
		if got := Sha256Hex([]byte(e.Stderr)); got != e.StderrSha256 {
			return fmt.Errorf("%s: stderrSha256 %s does not digest the declared envelope (%s)", where, e.StderrSha256, got)
		}
		ok, err := IsCompleteDocument([]byte(e.Stderr))
		if err != nil {
			return fmt.Errorf("%s: stderr envelope does not parse under the CF-V0-019 codec: %w", where, err)
		}
		if !ok {
			return fmt.Errorf("%s: stderr envelope is not codec bytes plus exactly one LF", where)
		}
	case "no-output-guarantee":
		// CF-V0-022's last sentence: "Process kill, uncatchable signal,
		// interpreter abort, or allocator/OS termination may prevent all output;
		// V0 makes no JSON, stderr, or exit-code guarantee when no handler can
		// execute." A fixture for that row asserts the ABSENCE of a partial or
		// misleading result, never a specific exit code.
		if e.Stdout != "none-or-unspecified" {
			return fmt.Errorf("%s: an uncatchable termination declares stdout none-or-unspecified", where)
		}
		if e.ErrorCode != "" || e.Stderr != "" || e.FrontierState != "" || e.ItemCount != "" {
			return fmt.Errorf("%s: no output is guaranteed, so no output may be declared", where)
		}
	default:
		return fmt.Errorf("%s: expect.kind must be result, error or no-output-guarantee, got %q", where, e.Kind)
	}
	return nil
}

// decimalCount parses a CF-V0-019 base-10 string: no sign, and no leading zero
// except the single digit "0".
func decimalCount(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("a result must declare an item count")
	}
	if s != "0" && s[0] == '0' {
		return 0, fmt.Errorf("%q has a leading zero, which CF-V0-019 forbids", s)
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%q is not a base-10 string", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func (i ExpectItem) validate(where string) error {
	if !itemKinds[i.Kind] {
		return fmt.Errorf("%s: item kind %q is outside CF-V0-007", where, i.Kind)
	}
	if !authorityClasses[i.AuthorityClass] {
		return fmt.Errorf("%s: authorityClass %q is outside CF-V0-017", where, i.AuthorityClass)
	}
	if !resolutionClasses[i.ResolutionClass] {
		return fmt.Errorf("%s: resolutionClass %q is outside CF-V0-017", where, i.ResolutionClass)
	}
	if len(i.Reasons) == 0 {
		return fmt.Errorf("%s: item has no reason", where)
	}
	// CF-V0-016: reasons are unique.
	seen := map[string]bool{}
	for _, r := range i.Reasons {
		if seen[r] {
			return fmt.Errorf("%s: duplicate reason %q", where, r)
		}
		seen[r] = true
	}
	// CF-V0-020: related IDs sort lexicographically and are deduplicated.
	if !sort.StringsAreSorted(i.RelatedIDs) {
		return fmt.Errorf("%s: relatedIds are not sorted", where)
	}
	for n := 1; n < len(i.RelatedIDs); n++ {
		if i.RelatedIDs[n] == i.RelatedIDs[n-1] {
			return fmt.Errorf("%s: duplicate relatedId %q", where, i.RelatedIDs[n])
		}
	}
	// CF-V0-023 per-item bounds.
	if len(i.RelatedIDs) > LimitRelatedIDsPerItem {
		return fmt.Errorf("%s: %d relatedIds exceeds the CF-V0-023 limit", where, len(i.RelatedIDs))
	}
	if len(i.Reasons) > LimitReasonsPerItem {
		return fmt.Errorf("%s: %d reasons exceeds the CF-V0-023 limit", where, len(i.Reasons))
	}
	return nil
}

// LoadFixtures reads every fixtures/<id>/case.json under dir.
func LoadFixtures(dir string) ([]Fixture, error) {
	root := filepath.Join(dir, "fixtures")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Fixture
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name(), "case.json")
		var f Fixture
		if err := loadJSON(path, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		f.Dir = filepath.Join(root, e.Name())
		if f.ID != e.Name() {
			return nil, fmt.Errorf("%s: fixture id %q does not match its directory %q", path, f.ID, e.Name())
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
