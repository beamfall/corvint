package jstestprovider

import "slices"

// Receipt-level runtime tuple classification (PWP-V0-009, issue #665). The
// browser half of the tuple stays per test in qualifiedPlaywrightTuple; this
// classifies only the fields every retained external receipt carries, so a
// reader can say why no test can project passing execution.
const (
	// RuntimeTupleNotApplicable marks a receipt outside the external profiles.
	RuntimeTupleNotApplicable = ""
	// RuntimeTupleCandidate means the runner and Node versions match a
	// qualified tuple; each test's browser identity still decides.
	RuntimeTupleCandidate = "candidate"
	// RuntimeTupleUnqualified means the observed runner or Node version is not
	// a qualified tuple for this profile, so no test can pass.
	RuntimeTupleUnqualified = "unqualified"
	// RuntimeTupleUnobserved means the receipt did not observe its Node
	// version, so qualification cannot be decided.
	RuntimeTupleUnobserved = "unobserved"
)

const qualifiedNodeVersion = "v22.23.2"

// admittedBundledNodeVersion is the one additional exact Node release admitted
// for the bundled headless-shell tuple on live evidence: the base profile on
// 1.63.0 (PWP-V0-009), and the base and attempt-retaining profiles on 1.64.0
// (PWP-V0-036). It never admits a semver range, another profile, or the
// system-browser tuple.
const admittedBundledNodeVersion = "v24.11.1"

// freshnessNodeVersion is the exact Node release the per-test freshness
// profile was live-qualified on (PTF-V0, 2026-10-02). It is a row of the
// shared table, not a range, and admits no external profile.
const freshnessNodeVersion = "v22.23.3"

// Browser sources a runtime tuple row admits. The bundled source is the
// per-runner headless-shell tuple; the system source is the configured
// system-Chrome tuple.
const (
	BrowserSourceBundled = "playwright-bundled"
	BrowserSourceSystem  = "system"
)

// Tuple fields a mismatch names, checked in this order (PWP-V0-040).
const (
	TupleFieldRunner        = "runner"
	TupleFieldNode          = "node"
	TupleFieldBrowserSource = "browser-source"
)

// admittedRuntimeTuple is one exact admitted (profile, runner, Node, browser
// source) row. Every field is an exact value: decision 0448 admits Node patch
// releases one exact release at a time, never a major or minor range.
type admittedRuntimeTuple struct {
	profile, runner, node, source string
}

// admittedRuntimeTuples is the one table receipts and freshness both admit
// from (PWP-V0-039). A row exists only for a tuple a live run observed;
// 1.60.0 predates the Node and browser binding and is not a row. The
// per-test browser identity (bundled revision, system version) and the
// 1.64.0 system headless condition still decide per test.
var admittedRuntimeTuples = func() []admittedRuntimeTuple {
	var rows []admittedRuntimeTuple
	add := func(runner, node, source string, profiles ...string) {
		for _, p := range profiles {
			rows = append(rows, admittedRuntimeTuple{p, runner, node, source})
		}
	}
	// 1.63.0: issue 175 / PWP-V0-008 / PWP-V0-009.
	add("1.63.0", qualifiedNodeVersion, BrowserSourceBundled, ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile)
	add("1.63.0", qualifiedNodeVersion, BrowserSourceSystem, ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile)
	add("1.63.0", admittedBundledNodeVersion, BrowserSourceBundled, ExternalProfile)
	// 1.64.0: PWP-V0-033, PWP-V0-035, PWP-V0-036, PWP-V0-037.
	add("1.64.0", qualifiedNodeVersion, BrowserSourceBundled, ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile)
	add("1.64.0", qualifiedNodeVersion, BrowserSourceSystem, ExternalProfile)
	add("1.64.0", admittedBundledNodeVersion, BrowserSourceBundled, ExternalProfile, AttemptExternalProfile)
	// Per-test freshness (PTF-V0): 1.63.0 on Node v22.23.3, bundled only.
	add("1.63.0", freshnessNodeVersion, BrowserSourceBundled, FreshnessProfile)
	// PWP-V0-050: 1.64.0 on Node v22.23.2, bundled only, after its own live
	// freshness run, so one exact tuple admits external and freshness
	// receipts alike. Freshness on v24.11.1 or another 1.64.0 Node has no
	// live run and no row.
	add("1.64.0", qualifiedNodeVersion, BrowserSourceBundled, FreshnessProfile)
	return rows
}()

// freshnessReporterTuples lists the table's bundled freshness (runner, Node)
// rows in table order. The provider passes them to the reporter, so its
// freshness pre-filter reads this table instead of a copied literal
// (PWP-V0-052).
func freshnessReporterTuples() [][2]string {
	rows := [][2]string{}
	for _, row := range admittedRuntimeTuples {
		if row.profile == FreshnessProfile && row.source == BrowserSourceBundled {
			rows = append(rows, [2]string{row.runner, row.node})
		}
	}
	return rows
}

// runtimeTupleAdmitted reports whether the exact tuple is a row. An empty
// source matches any admitted browser source for the profile, runner and Node.
func runtimeTupleAdmitted(profile, runner, node, source string) bool {
	for _, row := range admittedRuntimeTuples {
		if row.profile == profile && row.runner == runner && row.node == node && (source == "" || row.source == source) {
			return true
		}
	}
	return false
}

// RuntimeTupleMismatch names the first tuple field that leaves a request or
// receipt outside the shared table, and the exact values the table admits
// for that field given the fields before it (PWP-V0-040).
type RuntimeTupleMismatch struct {
	Profile  string   `json:"profile"`
	Field    string   `json:"field"`
	Admitted []string `json:"admitted"`
}

// AdmittedRuntimeTupleMismatch checks runner, then Node, then browser source
// against the shared table for profile and returns nil when the tuple is a
// row. An empty Node or source is not checked: before a run the browser
// source is unobserved, and a caller may check the runner alone.
func AdmittedRuntimeTupleMismatch(profile, runner, node, source string) *RuntimeTupleMismatch {
	rows := admittedRuntimeTuples
	for _, step := range []struct {
		field, value string
		get          func(admittedRuntimeTuple) string
	}{
		{TupleFieldRunner, runner, func(r admittedRuntimeTuple) string { return r.runner }},
		{TupleFieldNode, node, func(r admittedRuntimeTuple) string { return r.node }},
		{TupleFieldBrowserSource, source, func(r admittedRuntimeTuple) string { return r.source }},
	} {
		if step.value == "" && step.field != TupleFieldRunner {
			continue
		}
		var admitted []string
		var kept []admittedRuntimeTuple
		for _, row := range rows {
			if row.profile != profile {
				continue
			}
			if !slices.Contains(admitted, step.get(row)) {
				admitted = append(admitted, step.get(row))
			}
			if step.get(row) == step.value {
				kept = append(kept, row)
			}
		}
		if len(kept) == 0 {
			slices.Sort(admitted)
			if admitted == nil {
				admitted = []string{}
			}
			return &RuntimeTupleMismatch{Profile: profile, Field: step.field, Admitted: admitted}
		}
		rows = kept
	}
	return nil
}

// ReceiptRuntimeTuple classifies an external receipt's runner/Node tuple with
// the same closed values the passing predicate uses. It never widens them.
func ReceiptRuntimeTuple(r Receipt) string {
	if !isExternalProfile(r.Profile) {
		return RuntimeTupleNotApplicable
	}
	if r.Identity.NodeVersion == "" {
		return RuntimeTupleUnobserved
	}
	if r.Identity.RunnerVersion == "1.60.0" {
		if r.Profile == AttemptExternalProfile {
			return RuntimeTupleUnqualified
		}
		return RuntimeTupleCandidate
	}
	if runtimeTupleAdmitted(r.Profile, r.Identity.RunnerVersion, r.Identity.NodeVersion, "") {
		return RuntimeTupleCandidate
	}
	return RuntimeTupleUnqualified
}
