package testacceptance

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

func validateFreshnessOptions(r Request) error {
	f := r.Freshness
	c := f.Provider
	if r.AppBuildDir != "" || r.RunnerVersion != "1.63.0" || c.Mutation != nil || c.ProductDir != r.Product.Root || c.ProductExpected.Revision != r.Product.Commit || c.ProductExpected.Tree != r.Product.Tree || c.TestExpected.Revision != r.TestRepository.Commit || c.TestExpected.Tree != r.TestRepository.Tree || c.ProductExpected.DirtyState != "clean" || c.TestExpected.DirtyState != "clean" || c.ProductExpected.DirtyDigest != "" || c.TestExpected.DirtyDigest != "" || !reflect.DeepEqual(c.Runner.Argv, r.Runner.Argv) || !reflect.DeepEqual(c.Server.Argv, r.Server.Argv) || "sha256:"+c.Runner.ExecutableDigest != r.Runner.ExecutableSHA256 || "sha256:"+c.Runner.EntrypointDigest != r.Runner.EntrypointSHA256 || "sha256:"+c.Server.ExecutableDigest != r.Server.ExecutableSHA256 || "sha256:"+c.Server.EntrypointDigest != r.Server.EntrypointSHA256 || c.DocumentURL != r.ReadyURL {
		return errors.New("freshness-request-binding-invalid")
	}
	for _, root := range []struct{ root, expected string }{{r.Product.Root, c.ProductExpected.RootCommit}, {r.TestRepository.Root, c.TestExpected.RootCommit}} {
		actual, e := git(root.root, "rev-list", "--max-parents=0", "HEAD")
		if e != nil || actual != root.expected {
			return errors.New("freshness-root-commit-invalid")
		}
	}
	for _, file := range []File{{Path: filepath.Join(r.Product.Root, c.ArtifactPath), SHA256: "sha256:" + c.SourceDigest}, {Path: c.ObserverConfigFile, SHA256: "sha256:" + c.ObserverConfigDigest}, f.AttestationConfiguration} {
		if regular(file.Path) != nil {
			return errors.New("freshness-input-unavailable")
		}
		b, e := os.ReadFile(file.Path)
		if e != nil || Hash(b) != file.SHA256 {
			return errors.New("freshness-input-drift")
		}
	}
	for _, command := range []jstestprovider.FreshCommandIdentity{c.Runner, c.Server, c.Observer} {
		if len(command.Argv) < 2 || len(command.Argv) > 16 {
			return errors.New("freshness-command-invalid")
		}
		for _, pin := range []File{{command.Argv[0], "sha256:" + command.ExecutableDigest}, {command.Argv[1], "sha256:" + command.EntrypointDigest}} {
			if regular(pin.Path) != nil {
				return errors.New("freshness-command-unavailable")
			}
			b, e := os.ReadFile(pin.Path)
			if e != nil || Hash(b) != pin.SHA256 {
				return errors.New("freshness-command-drift")
			}
		}
	}
	if len(f.Attestation.Argv) == 0 || len(f.Attestation.Argv) > 16 || regular(f.Attestation.Argv[0]) != nil {
		return errors.New("freshness-attestation-command-invalid")
	}
	b, e := os.ReadFile(f.Attestation.Argv[0])
	if e != nil || Hash(b) != f.Attestation.ExecutableSHA256 {
		return errors.New("freshness-attestation-executable-drift")
	}
	if len(f.Attestation.Argv) > 1 && filepath.IsAbs(f.Attestation.Argv[1]) {
		entry, e := os.ReadFile(f.Attestation.Argv[1])
		if e != nil || Hash(entry) != f.Attestation.EntrypointSHA256 {
			return errors.New("freshness-attestation-entrypoint-drift")
		}
	}
	if c.Dependencies == nil || c.DependencyDigest != jstestprovider.FreshDependencyDigest(c.Dependencies) || !jstestprovider.FreshDependenciesMatch(c.Dependencies, jstestprovider.ObserveFreshDependencies(c.Dependencies)) {
		return errors.New("freshness-dependency-closure-unqualified")
	}
	return nil
}

// Bind independently decoded observations to the approved request rather than
// accepting the expected target asserted by the receipt itself.
func freshRequestBinding(n jstestprovider.Receipt, r Request) bool {
	if r.Freshness == nil || n.Freshness == nil || n.ApplicationAttestation == nil {
		return false
	}
	f, c := n.Freshness, r.Freshness.Provider
	if !reflect.DeepEqual(f.Dependencies, c.Dependencies) || f.DependencyDigest != c.DependencyDigest || f.ProductExpected != c.ProductExpected || f.TestExpected != c.TestExpected || f.ArtifactPath != c.ArtifactPath || f.SourceDigest != c.SourceDigest || f.DocumentURL != c.DocumentURL || !reflect.DeepEqual(f.Runner.Expected, c.Runner) || !reflect.DeepEqual(f.Server.Expected, c.Server) || !reflect.DeepEqual(f.Observer.Expected, c.Observer) || f.ObserverConfigBefore != c.ObserverConfigDigest || f.ObserverConfigAfter != c.ObserverConfigDigest {
		return false
	}
	if n.Identity.ConfigFile != r.Config || n.Identity.RunnerVersion != r.RunnerVersion {
		return false
	}
	inputs := map[string]string{}
	for _, pin := range r.Inputs {
		inputs[pin.Path] = strings.TrimPrefix(pin.SHA256, "sha256:")
	}
	if n.Identity.ConfigDigest != inputs[r.Config] || n.Identity.PackageDigest != strings.TrimPrefix(Hash([]byte(r.Package+"="+inputs[r.Package]+"\n"+r.Lockfile+"="+inputs[r.Lockfile]+"\n")), "sha256:") {
		return false
	}
	for path, digest := range n.Identity.ConfigInputDigests {
		if inputs[path] != digest && (c.Dependencies == nil || c.Dependencies.Files[path] != digest) {
			return false
		}
	}
	for _, test := range r.Tests {
		if n.Identity.TestFileDigests[test.File] != inputs[test.File] {
			return false
		}
	}
	a := n.ApplicationAttestation.Provider
	return reflect.DeepEqual(a.Argv, r.Freshness.Attestation.Argv) && a.ExecutableDigest == strings.TrimPrefix(r.Freshness.Attestation.ExecutableSHA256, "sha256:") && a.ConfigDigest == strings.TrimPrefix(r.Freshness.AttestationConfiguration.SHA256, "sha256:")
}

func freshSemanticMatch(t Test, n jstestprovider.TestOutcome) bool {
	return n.Anchor != nil && n.Anchor.File == t.File && n.Anchor.Line == t.Line && n.Name == t.Title && n.Project != nil && n.Project.Name == t.Project && n.ID != ""
}

func freshNativeRow(r jstestprovider.Receipt, t Test) (jstestprovider.TestOutcome, bool) {
	var row jstestprovider.TestOutcome
	count := 0
	for _, n := range r.Tests {
		if freshSemanticMatch(t, n) {
			row = n
			count++
		}
	}
	return row, count == 1
}

// normalizedFreshArgv removes only independently recognized provider transport
// and the exact requested file selectors. Unknown argv differences remain.
func normalizedFreshArgv(n jstestprovider.Receipt, r Request) []string {
	out := []string{}
	for _, arg := range n.Identity.Argv {
		if strings.HasPrefix(arg, "--config=") {
			out = append(out, "--config="+n.Identity.ConfigFile)
			continue
		}
		selector := false
		for _, t := range r.Tests {
			if arg == regexp.QuoteMeta(t.File) || arg == "--grep="+isolationPattern(t.Title) {
				selector = true
			}
		}
		if !selector {
			out = append(out, arg)
		}
	}
	return out
}

func responseDefinition(control behaviorfalsify.PlannedControl) (jstestprovider.ResponseMutationDefinition, bool) {
	d := control.Definition
	if control.Kind != behaviorfalsify.ChangedFixtureValue || control.Disposition != "run" || len(d) != 5 {
		return jstestprovider.ResponseMutationDefinition{}, false
	}
	for _, key := range []string{"artifactPath", "sourceSha256", "from", "to", "mutantSha256"} {
		if _, ok := d[key]; !ok {
			return jstestprovider.ResponseMutationDefinition{}, false
		}
	}
	return jstestprovider.ResponseMutationDefinition{ArtifactPath: d["artifactPath"], SourceSHA256: d["sourceSha256"], From: d["from"], To: d["to"], MutantSHA256: d["mutantSha256"]}, true
}

// sameFreshClosure allows exactly the derived response bytes and an observed
// new server generation to differ. Source, test, tools, config and environment
// remain exact; no generic verified kill can substitute for this join.
func sameFreshClosure(base, mutant jstestprovider.Receipt, baseRow, mutantRow jstestprovider.TestOutcome, r Request, d jstestprovider.ResponseMutationDefinition) bool {
	if baseRow.FullName != mutantRow.FullName || baseRow.Name != mutantRow.Name || !reflect.DeepEqual(baseRow.Anchor, mutantRow.Anchor) || !reflect.DeepEqual(baseRow.Project, mutantRow.Project) || !reflect.DeepEqual(baseRow.ImportedFiles, mutantRow.ImportedFiles) {
		return false
	}

	b, m := base.Freshness, mutant.Freshness
	if b == nil || m == nil || b.Mutation != nil || m.Mutation == nil || *m.Mutation != d || b.Leader == nil || m.Leader == nil || *b.Leader == *m.Leader {
		return false
	}
	derived, e := jstestprovider.ResponseMutation(b.Source, b.ArtifactPath, d)
	if e != nil {
		return false
	}
	for _, s := range []*jstestprovider.FreshServeObservation{m.ServedBefore, m.ServedAfter} {
		if s == nil || !bytes.Equal(s.Body, derived) {
			return false
		}
	}
	bi, mi := base.Identity, mutant.Identity
	bi.Argv = normalizedFreshArgv(base, r)
	mi.Argv = normalizedFreshArgv(mutant, r)
	if !reflect.DeepEqual(bi, mi) || !reflect.DeepEqual(base.TestRepositoryAtStart, mutant.TestRepositoryAtStart) || !reflect.DeepEqual(base.TestRepositoryAtPublish, mutant.TestRepositoryAtPublish) {
		return false
	}
	// Compare each command and source/environment observation without mutating
	// either receipt. Process and served-response fields are checked separately.
	if !reflect.DeepEqual(b.Dependencies, m.Dependencies) || b.DependencyDigest != m.DependencyDigest || !reflect.DeepEqual(b.DependenciesBefore, m.DependenciesBefore) || !reflect.DeepEqual(b.DependenciesAfter, m.DependenciesAfter) || b.ProductExpected != m.ProductExpected || b.TestExpected != m.TestExpected || !reflect.DeepEqual(b.ProductBefore, m.ProductBefore) || !reflect.DeepEqual(b.ProductAfter, m.ProductAfter) || b.ArtifactPath != m.ArtifactPath || !bytes.Equal(b.Source, m.Source) || b.SourceDigest != m.SourceDigest || b.ArtifactBefore != m.ArtifactBefore || b.ArtifactAfter != m.ArtifactAfter || b.DocumentURL != m.DocumentURL || !reflect.DeepEqual(b.Runner, m.Runner) || !reflect.DeepEqual(b.Server, m.Server) || !reflect.DeepEqual(b.Observer, m.Observer) || b.ObserverConfigBefore != m.ObserverConfigBefore || b.ObserverConfigAfter != m.ObserverConfigAfter || !reflect.DeepEqual(b.EnvironmentBefore, m.EnvironmentBefore) || !reflect.DeepEqual(b.EnvironmentAfter, m.EnvironmentAfter) {
		return false
	}
	ba, ma := base.ApplicationAttestation, mutant.ApplicationAttestation
	if ba == nil || ma == nil || !reflect.DeepEqual(ba.Provider, ma.Provider) || ba.Expectation != ma.Expectation {
		return false
	}
	for i, bo := range []*jstestprovider.ApplicationAttestationObservation{ba.Before, ba.After} {
		mo := []*jstestprovider.ApplicationAttestationObservation{ma.Before, ma.After}[i]
		if bo == nil || mo == nil {
			return false
		}
		bv, mv := bo.Attestation, mo.Attestation
		bv.Instance.ID = ""
		bv.Instance.StartGeneration = ""
		mv.Instance.ID = ""
		mv.Instance.StartGeneration = ""
		if bv != mv {
			return false
		}
	}
	return true
}

func joinedFreshControl(c Control, t Test, r Request, baselines []jstestprovider.Receipt) bool {
	if t.Control == nil || t.Tool == nil || c.ID != t.ID || c.Evidence == nil || c.PlanDigest != t.Control.Digest || c.ReceiptDigest != c.Evidence.Digest || c.Evidence.Tool != *t.Tool || cleanupState(c.Cleanup) != "observed-absent" || behaviorfalsify.VerifyReceipt(*c.Evidence, t.Control.Digest, t.Tool.Executable) != nil {
		return false
	}
	e := c.Evidence
	if e.Report.Executed != e.Report.Requested || e.Report.Executed == 0 || len(e.Report.CoverageGaps) != 0 || !e.Report.MutationScore.Defined || e.Report.MutationScore.Killed != e.Report.MutationScore.Denominator || len(e.RawAttempts) == 0 || len(baselines) != r.Repeat {
		return false
	}
	for _, control := range t.Control.Controls {
		if _, ok := responseDefinition(control); !ok {
			return false
		}
	}
	for _, raw := range e.RawAttempts {
		if raw.Ordinal < 1 || raw.Ordinal > len(t.Control.Controls) || raw.ControlID != t.Control.Controls[raw.Ordinal-1].ID || len(raw.Omissions) != 0 || Hash(raw.NativeBytes) != raw.NativeSHA256 || Hash(raw.HookBytes) != raw.HookSHA256 {
			return false
		}
		control := t.Control.Controls[raw.Ordinal-1]
		d, ok := responseDefinition(control)
		if !ok {
			return false
		}
		var hook behaviorfalsify.HookReceipt
		if behaviorfalsify.Decode(raw.HookBytes, &hook) != nil || hook.NativeReceiptSHA256 != raw.NativeSHA256 || hook.PerturbationSHA256 != control.PerturbationSHA256 || hook.Target != t.Control.Request.Target {
			return false
		}
		native, err := jstestprovider.DecodeFreshness(raw.NativeBytes)
		if err != nil || !freshRequestBinding(native, r) {
			return false
		}
		row, ok := freshNativeRow(native, t)
		if !ok || len(native.Tests) != 1 || jstestprovider.FreshnessCurrency(native, row) != testvalidity.FreshnessCurrent || !jstestprovider.TargetAssertionFailure(row, t.Control.Request.Target.AssertionID) {
			return false
		}
		for _, base := range baselines {
			baseRow, ok := freshNativeRow(base, t)
			if !ok || !sameFreshClosure(base, native, baseRow, row, r, d) {
				return false
			}
		}
	}
	return true
}

// classifyFreshness consumes every actual repeat receipt and recomputes axes.
// A later successful row cannot overwrite an earlier stale or absent row.
func classifyFreshness(report *Report, r Request) {
	report.Assessments = nil
	report.Reasons = []string{}
	report.Verdict = "accepted"
	for index, t := range r.Tests {
		a := Assessment{ID: t.ID, Verdict: "blocked", Reasons: []string{}, Validity: testvalidity.Project(testvalidity.Input{})}
		reject := false
		count := 0
		ordinals := map[int]bool{}
		baselines := []jstestprovider.Receipt{}
		cleanupStates := []string{}
		currency := testvalidity.FreshnessCurrent
		for _, run := range report.Runs {
			if run.Kind != "repeat" {
				continue
			}
			count++
			if run.Ordinal < 1 || run.Ordinal > r.Repeat || ordinals[run.Ordinal] {
				a.Reasons = append(a.Reasons, "repeat-ordinal-invalid")
			}
			ordinals[run.Ordinal] = true
			cleanupStates = append(cleanupStates, cleanupState(run.Cleanup))
			a.Reasons = append(a.Reasons, run.Reasons...)
			if cleanupState(run.Cleanup) == "survivors" {
				reject = true
				a.Reasons = append(a.Reasons, "observed-cleanup-survivor")
			}
			if cleanupState(run.Cleanup) != "observed-absent" {
				a.Reasons = append(a.Reasons, "cleanup-unknown")
			}
			native, err := jstestprovider.DecodeFreshness(run.NativeReceipt)
			// Early observed drift survives missing runner rows and later observations.
			if err == nil && Hash(run.NativeReceipt) == run.ReceiptSHA256 && r.Freshness != nil && native.Freshness != nil && native.Freshness.ProductExpected == r.Freshness.Provider.ProductExpected && native.Freshness.TestExpected == r.Freshness.Provider.TestExpected && native.Freshness.ArtifactPath == r.Freshness.Provider.ArtifactPath && native.Freshness.SourceDigest == r.Freshness.Provider.SourceDigest && native.Freshness.DocumentURL == r.Freshness.Provider.DocumentURL && jstestprovider.FreshnessCurrency(native, jstestprovider.TestOutcome{}) == testvalidity.FreshnessStale {
				currency = testvalidity.FreshnessStale
				reject = true
				a.Reasons = append(a.Reasons, "provider-source-stale")
			}

			if err != nil || Hash(run.NativeReceipt) != run.ReceiptSHA256 || !freshRequestBinding(native, r) {
				a.Reasons = append(a.Reasons, "native-repeat-invalid")
				if currency != testvalidity.FreshnessStale {
					currency = testvalidity.FreshnessUnknown
				}
				continue
			}
			row, ok := freshNativeRow(native, t)
			if !ok || len(native.Tests) != len(r.Tests) {
				a.Reasons = append(a.Reasons, "native-test-identity-unknown")
				if currency != testvalidity.FreshnessStale {
					currency = testvalidity.FreshnessUnknown
				}
				continue
			}
			p := jstestprovider.ReceiptTestProjection(native, row)
			a.Validity = p
			observeDuration(&a.Repeats, row.DurationMS)
			if p.Freshness.State == testvalidity.FreshnessStale {
				currency = testvalidity.FreshnessStale
				reject = true
				a.Reasons = append(a.Reasons, "provider-source-stale")
			} else if p.Freshness.State != testvalidity.FreshnessCurrent && currency != testvalidity.FreshnessStale {
				currency = testvalidity.FreshnessUnknown
				a.Reasons = append(a.Reasons, "provider-freshness-unknown")
			}
			carried := 0
			for _, declared := range run.Rows {
				if declared.ID != t.ID {
					continue
				}
				carried++
				if declared.ObservedID != row.ID || declared.State != row.State || declared.Attempts != len(row.Attempts) || declared.Retries != row.Retries || len(declared.IdentityUnknown) != 0 || !reflect.DeepEqual(declared.Validity, p) {
					a.Reasons = append(a.Reasons, "repeat-projection-drift")
				}
			}
			if carried != 1 {
				a.Reasons = append(a.Reasons, "repeat-row-missing-or-ambiguous")
			}
			switch row.State {
			case jstestprovider.StatePassed:
				a.Repeats.Passed++
			case jstestprovider.StateFailed:
				a.Repeats.Failed++
				reject = true
				a.Reasons = append(a.Reasons, "test-failed")
			default:
				a.Repeats.Other++
				reject = true
				a.Reasons = append(a.Reasons, "flaky-skipped-or-incomplete")
			}
			if row.Retries != 0 || len(row.Attempts) != 1 {
				reject = true
				a.Reasons = append(a.Reasons, "retry-or-attempt-count")
			}
			if p.Association.State != testvalidity.AssociationAssociated || p.Hygiene.State != testvalidity.HygieneEligible || p.Execution.State != testvalidity.ExecutionPassed {
				a.Reasons = append(a.Reasons, "provider-validity-incomplete")
			}
			baselines = append(baselines, native)
		}
		a.Validity.Freshness = testvalidity.Axis{State: currency, Reason: "all-baseline-observations"}
		a.Validity.Strength = testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: "native-control-join-incomplete"}
		if count != r.Repeat || len(baselines) != r.Repeat {
			a.Reasons = append(a.Reasons, "repeat-evidence-incomplete")
		}
		if index >= len(report.Controls) {
			a.Reasons = append(a.Reasons, "negative-control-missing")
		} else {
			c := report.Controls[index]
			cleanupStates = append(cleanupStates, cleanupState(c.Cleanup))
			if c.Status == "survived" {
				reject = true
				a.Reasons = append(a.Reasons, "negative-control-survived")
			}
			if cleanupState(c.Cleanup) == "survivors" {
				reject = true
				a.Reasons = append(a.Reasons, "control-cleanup-survivor")
			}
			if joinedFreshControl(c, t, r, baselines) && currency == testvalidity.FreshnessCurrent {
				a.Validity.Strength = testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: "verified-native-target-assertion-kill-joined-to-every-repeat"}
			} else {
				a.Reasons = append(a.Reasons, "native-control-join-incomplete")
			}
		}
		a.Cleanup = worstCleanup(cleanupStates)
		a.Order = orderEvidence(report.Runs, r, t.ID)
		if reject {
			a.Verdict = "rejected"
		} else if len(a.Reasons) == 0 && a.Repeats.Passed == r.Repeat && a.Cleanup == "observed-absent" && a.Validity.Strength.State == testvalidity.StrengthKilled {
			a.Verdict = "accepted"
		}
		slices.Sort(a.Reasons)
		a.Reasons = slices.Compact(a.Reasons)
		report.Assessments = append(report.Assessments, a)
		report.Reasons = append(report.Reasons, a.Reasons...)
		if a.Verdict == "rejected" {
			report.Verdict = "rejected"
		} else if a.Verdict != "accepted" && report.Verdict != "rejected" {
			report.Verdict = "blocked"
		}
	}
	slices.Sort(report.Reasons)
	report.Reasons = slices.Compact(report.Reasons)
	report.Body = renderBody(*report, r)
}
