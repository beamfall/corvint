package jstestprovider

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
// for the base external profile's bundled headless-shell tuple on live evidence
// (PWP-V0-009, proposed pending owner acceptance). It never admits a semver
// range, another profile, or the system-browser tuple.
const admittedBundledNodeVersion = "v24.11.1"

// ReceiptRuntimeTuple classifies an external receipt's runner/Node tuple with
// the same closed values the passing predicate uses. It never widens them.
func ReceiptRuntimeTuple(r Receipt) string {
	if !isExternalProfile(r.Profile) {
		return RuntimeTupleNotApplicable
	}
	switch r.Identity.RunnerVersion {
	case "1.60.0":
		if r.Profile == AttemptExternalProfile {
			return RuntimeTupleUnqualified
		}
		return RuntimeTupleCandidate
	case "1.63.0":
		if r.Identity.NodeVersion == "" {
			return RuntimeTupleUnobserved
		}
		if r.Identity.NodeVersion == qualifiedNodeVersion || (r.Profile == ExternalProfile && r.Identity.NodeVersion == admittedBundledNodeVersion) {
			return RuntimeTupleCandidate
		}
	}
	return RuntimeTupleUnqualified
}
