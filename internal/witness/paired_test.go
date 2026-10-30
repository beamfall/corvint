package witness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// pairedTestsFixture commits a separate tests repository whose range changes
// e2e/login.spec.ts, and returns its head index and base.
func pairedTestsFixture(t *testing.T) (*contextindex.Index, string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := initTestRepo(t, git)
	fixtureGit := func(args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)...)
		command.Dir, command.Env = root, gitEnvironment()
		raw, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	write := func(body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, "e2e"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "e2e", "login.spec.ts"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("test('login', async () => {});\n")
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	base := fixtureGit("rev-parse", "HEAD")
	write("test('login works', async () => {});\n")
	fixtureGit("commit", "--quiet", "-am", "change")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index, base
}

// pairedReceipt is an E2E receipt that ran in the tests checkout at its head,
// declares one test in e2e/login.spec.ts, and reports importing the app
// checkout's a/a.go with the app head digest; its application attestation
// names the app head commit and tree, clean before and after.
func pairedReceipt(app, tests *contextindex.Index) jstestprovider.Receipt {
	identity := &jstestprovider.ApplicationRepositoryIdentity{RootCommit: evidenceRootCommit, Revision: tests.CommitRevision, Tree: tests.Revision, DirtyState: "clean"}
	publish := *identity
	application := jstestprovider.ApplicationRepositoryIdentity{RootCommit: evidenceRootCommit, Revision: app.CommitRevision, Tree: app.Revision, DirtyState: "clean"}
	testsRoot, appRoot := filepath.ToSlash(tests.Root), filepath.ToSlash(app.Root)
	receipt := jstestprovider.Receipt{
		Kind: "e2e", TestRepositoryAtStart: identity, TestRepositoryAtPublish: &publish,
		Tests: []jstestprovider.TestOutcome{{
			Name: "login works", FullName: "login works", State: jstestprovider.StatePassed,
			Anchor:        &jstestprovider.Anchor{File: "login.spec.ts", Line: 1},
			ImportedFiles: map[string]string{appRoot + "/a/a.go": headDigest(app, "a/a.go"), testsRoot + "/e2e/login.spec.ts": headDigest(tests, "e2e/login.spec.ts")},
		}},
		ApplicationAttestation: &jstestprovider.ApplicationAttestationReceipt{
			Before:   &jstestprovider.ApplicationAttestationObservation{Attestation: jstestprovider.ApplicationAttestation{Repository: application}},
			After:    &jstestprovider.ApplicationAttestationObservation{Attestation: jstestprovider.ApplicationAttestation{Repository: application}},
			Failures: []string{},
		},
	}
	receipt.Identity.TestFileDigests = map[string]string{testsRoot + "/e2e/login.spec.ts": headDigest(tests, "e2e/login.spec.ts")}
	return receipt
}

func projected(receipt jstestprovider.Receipt) map[string]any {
	return map[string]any{"schema": testvaliditydoc.Schema, "playwright": receipt}
}

func compilePaired(t *testing.T, app, tests *contextindex.Index, appBase, testsBase string, receipt jstestprovider.Receipt) *PairedReport {
	t.Helper()
	report, err := CompilePaired(context.Background(),
		PairedInput{Root: app.Root, Index: app, Options: Options{Base: appBase}},
		PairedInput{Root: tests.Root, Index: tests, Options: Options{Base: testsBase, Evidence: []string{writeEvidence(t, projected(receipt))}}})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func sourceNamed(report *Report, name string) *Source {
	for position := range report.Sources {
		if report.Sources[position].Name == name {
			return &report.Sources[position]
		}
	}
	return nil
}

// TestPairedReceiptCitesApplicationObligations pins AGW-V0-013 and
// AGW-V0-014: the paired report names both ranges, the receipt binds to the
// tests head, and its attested import of the app head's a/a.go cites the app
// change obligation as a non-closing cross-range witness whose evidence path
// is the tests-range spec file; the summary sums both ranges.
func TestPairedReceiptCitesApplicationObligations(t *testing.T) {
	app, appBase := packetFixture(t, "example.test/fixture")
	tests, testsBase := pairedTestsFixture(t)
	report := compilePaired(t, app, tests, appBase, testsBase, pairedReceipt(app, tests))
	if report.Profile != PairedProfile || len(report.Ranges) != 2 || report.Ranges[0].Name != AppRange || report.Ranges[1].Name != TestsRange {
		t.Fatalf("report = %+v", report)
	}
	appReport, testsReport := report.Ranges[0].Report, report.Ranges[1].Report
	if appReport.Range.Head != app.CommitRevision || testsReport.Range.Head != tests.CommitRevision {
		t.Fatalf("ranges %+v %+v", appReport.Range, testsReport.Range)
	}
	if evidence := sourceNamed(testsReport, "evidence"); evidence == nil || evidence.Status != "BOUND" {
		t.Fatalf("tests evidence = %+v", evidence)
	}
	cross := sourceNamed(appReport, crossRangeSourceName)
	if cross == nil || cross.Status != "BOUND" || cross.Basis != 1 || cross.Unattached != 0 {
		t.Fatalf("cross-range source = %+v", cross)
	}
	found := 0
	for _, item := range appReport.Obligations {
		for _, candidate := range item.Witnesses {
			if candidate.Source != crossRangeSourceName {
				continue
			}
			if item.Path != "a/a.go" || candidate.EvidencePath != "e2e/login.spec.ts" || candidate.Closing || candidate.Authority != "CALLER_REPORTED" || candidate.Test != "login works" || candidate.Relation != "test-run:passed" {
				t.Fatalf("cross-range witness on %s = %+v", item.Path, candidate)
			}
			found++
		}
		if item.Verdict == Closed {
			t.Fatalf("a cross-range witness closed %+v", item)
		}
	}
	if found != 1 {
		t.Fatalf("cross-range witnesses = %d, want 1", found)
	}
	if want := appReport.Summary.Opened + testsReport.Summary.Opened; report.Summary.Opened != want || appReport.Summary.CandidateWitnessed == 0 {
		t.Fatalf("summary %+v app %+v", report.Summary, appReport.Summary)
	}
	if text := RenderPaired(report); !strings.Contains(text, "range app base="+appBase) || !strings.Contains(text, "range tests base="+testsBase) || !strings.Contains(text, "cross-range") {
		t.Fatalf("rendered:\n%s", text)
	}
}

// TestPairedReceiptWithoutApplicationProofCitesNothing pins the AGW-V0-014
// failure modes: a receipt bound to the tests head whose application
// attestation or app digests do not prove the app head is UNBOUND on the app
// range and cites nothing there, while it still binds on the tests range.
func TestPairedReceiptWithoutApplicationProofCitesNothing(t *testing.T) {
	app, appBase := packetFixture(t, "example.test/fixture")
	tests, testsBase := pairedTestsFixture(t)
	other := strings.Repeat("ab", 20)
	cases := []struct {
		name   string
		change func(*jstestprovider.Receipt)
		reason string
	}{
		{"no attestation", func(r *jstestprovider.Receipt) { r.ApplicationAttestation = nil }, "carries no application attestation"},
		{"failures", func(r *jstestprovider.Receipt) { r.ApplicationAttestation.Failures = []string{"health-failed"} }, "records failures: health-failed"},
		{"no after", func(r *jstestprovider.Receipt) { r.ApplicationAttestation.After = nil }, "before and after the run"},
		{"moved", func(r *jstestprovider.Receipt) {
			after := *r.ApplicationAttestation.After
			after.Attestation.Repository.Revision = other
			r.ApplicationAttestation.After = &after
		}, "changed between before and after"},
		{"dirty", func(r *jstestprovider.Receipt) {
			for _, side := range []*jstestprovider.ApplicationAttestationObservation{r.ApplicationAttestation.Before, r.ApplicationAttestation.After} {
				side.Attestation.Repository.DirtyState = "dirty"
			}
		}, "not clean during the run"},
		{"revision", func(r *jstestprovider.Receipt) {
			for _, side := range []*jstestprovider.ApplicationAttestationObservation{r.ApplicationAttestation.Before, r.ApplicationAttestation.After} {
				side.Attestation.Repository.Revision = appBase
			}
		}, "not the application head " + app.CommitRevision},
		{"tree", func(r *jstestprovider.Receipt) {
			for _, side := range []*jstestprovider.ApplicationAttestationObservation{r.ApplicationAttestation.Before, r.ApplicationAttestation.After} {
				side.Attestation.Repository.Tree = other
			}
		}, "not the application head tree"},
		{"app digest", func(r *jstestprovider.Receipt) {
			r.Tests[0].ImportedFiles[filepath.ToSlash(app.Root)+"/a/a.go"] = strings.Repeat("0", 64)
		}, "digest for a/a.go differs from the head blob"},
		{"no app root", func(r *jstestprovider.Receipt) {
			delete(r.Tests[0].ImportedFiles, filepath.ToSlash(app.Root)+"/a/a.go")
			r.Tests[0].ImportedFiles["a/a.go"] = headDigest(app, "a/a.go")
		}, "no single application checkout root"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			receipt := pairedReceipt(app, tests)
			testCase.change(&receipt)
			report := compilePaired(t, app, tests, appBase, testsBase, receipt)
			appReport, testsReport := report.Ranges[0].Report, report.Ranges[1].Report
			if evidence := sourceNamed(testsReport, "evidence"); evidence == nil || evidence.Status != "BOUND" {
				t.Fatalf("tests evidence = %+v", evidence)
			}
			cross := sourceNamed(appReport, crossRangeSourceName)
			if cross == nil || cross.Status != "UNBOUND" || !strings.Contains(cross.Detail, testCase.reason) || cross.Basis != 0 {
				t.Fatalf("cross-range source = %+v, want UNBOUND naming %q", cross, testCase.reason)
			}
			for _, item := range appReport.Obligations {
				if len(item.Witnesses) != 0 {
					t.Fatalf("an unbound cross-range receipt cited %s: %+v", item.Path, item.Witnesses)
				}
			}
		})
	}
}

// TestCompilePairedRefusesSingleRepositoryOptions pins AGW-V0-013: evidence
// belongs to the tests range, and --cem or a stable artifact root names one
// repository, so either refuses the paired report.
func TestCompilePairedRefusesSingleRepositoryOptions(t *testing.T) {
	app, appBase := packetFixture(t, "example.test/fixture")
	for _, testCase := range []struct {
		app, tests Options
		reason     string
	}{
		{Options{Base: appBase, Evidence: []string{"/x.json"}}, Options{Base: appBase}, "only for the tests range"},
		{Options{Base: appBase}, Options{Base: appBase, CEMPath: "x.json"}, "--cem and --stable-artifact-root"},
	} {
		if _, err := CompilePaired(context.Background(), PairedInput{Index: app, Options: testCase.app}, PairedInput{Index: app, Options: testCase.tests}); err == nil || !strings.Contains(err.Error(), testCase.reason) {
			t.Fatalf("err = %v, want %q", err, testCase.reason)
		}
	}
}
