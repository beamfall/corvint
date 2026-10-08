package testvaliditydoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// keptReceipt is a passing external receipt from a keep-reporters run.
func keptReceipt() jstestprovider.Receipt {
	identity := jstestprovider.Identity{RunnerName: "playwright", RunnerVersion: "1.60.0", NodeVersion: "v22", Argv: []string{"npx", "playwright", "--config=/tmp/a/config.cjs", "--project=one"}, ConfigFile: "/repo/config.cjs", ConfigDigest: "config", TestFileDigests: map[string]string{"/repo/test.cjs": "test"}, ConfigInputDigests: map[string]string{"/repo/config.cjs": "config", "/repo/project-reporter.cjs": strings.Repeat("b", 64)}}
	test := jstestprovider.TestOutcome{Name: "pass", FullName: "one > pass", State: jstestprovider.StatePassed, Anchor: &jstestprovider.Anchor{File: "/repo/test.cjs", Line: 1}, Project: &jstestprovider.ProjectIdentity{Name: "one", Browser: "chromium", Device: "unknown", Use: json.RawMessage(`{}`), ConfigDigest: "config"}, Attempts: []jstestprovider.Attempt{{State: jstestprovider.StatePassed, FailureKind: "none"}}}
	test.ID = boundTestID(identity, test)
	return jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", Identity: identity, Tests: []jstestprovider.TestOutcome{test}, External: &jstestprovider.ExternalLifecycle{
		Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true,
		ReadyURL: "http://127.0.0.1:3002", DeclaredAppIdentity: "fixture", ConfigOverride: "controlled-fixture-config, keepReporters:true}]]};\n",
		ProjectReporters: &jstestprovider.ProjectReporters{Effects: "unknown", Entries: []jstestprovider.ProjectReporter{
			{Name: "list", Module: "builtin", Options: "absent"},
			{Name: "/repo/project-reporter.cjs", Module: "bound", ModuleDigest: strings.Repeat("b", 64), Options: "bound", OptionsDigest: strings.Repeat("c", 64)},
			{Name: "/outside/reporter.mjs", Module: "unknown", Options: "unknown"},
		}},
	}}
}

// boundTestID is the provider's qualified test ID for this fixture's argv.
func boundTestID(identity jstestprovider.Identity, test jstestprovider.TestOutcome) string {
	bound := identity
	bound.Argv = []string{"npx", "playwright", "--config=/repo/config.cjs", "--project=one"}
	raw, _ := json.Marshal(struct {
		Identity jstestprovider.Identity
		Project  *jstestprovider.ProjectIdentity
		Anchor   *jstestprovider.Anchor
		FullName string
	}{bound, test.Project, test.Anchor, test.FullName})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// PWP-V0-016: test-validity accepts a retained provider-first keep-reporters
// receipt carrying a matching qualification and recomputes a passing
// projection; the same receipt without it abstains.
func TestKeepReportersQualifiedReceiptProjects(t *testing.T) {
	// Real reporter-observed config inputs carry hex digests.
	hexConfig := func(r *jstestprovider.Receipt) {
		digest := strings.Repeat("a", 64)
		r.Identity.ConfigDigest, r.Identity.ConfigInputDigests["/repo/config.cjs"], r.Tests[0].Project.ConfigDigest = digest, digest, digest
		r.Tests[0].ID = boundTestID(r.Identity, r.Tests[0])
	}
	keep := keptReceipt()
	hexConfig(&keep)
	keep.External.ConfigOverride = "controlled-fixture-config, keepReporters:true}], ...keptReporters(original.reporter)]};\n"
	keep.External.ProjectReporters.Entries = keep.External.ProjectReporters.Entries[:2]
	control := keptReceipt()
	control.External.ConfigOverride, control.External.ProjectReporters = "controlled-fixture-config", nil
	control.Identity.ConfigInputDigests = map[string]string{"/repo/config.cjs": "config"}
	hexConfig(&control)
	record := jstestprovider.QualifyKeepReporters(control, nil, keep, nil)
	if record.Verdict != jstestprovider.KeepReportersQualified {
		t.Fatalf("fixture pair not qualified: %v", record.Reasons)
	}
	for _, carried := range []bool{false, true} {
		r := keptReceipt()
		hexConfig(&r)
		r.External.ConfigOverride, r.External.ProjectReporters.Entries = keep.External.ConfigOverride, keep.External.ProjectReporters.Entries
		if carried {
			r.External.ProjectReporters.Qualification = &record
		}
		data, err := jstestprovider.EncodeQualified(r)
		if err != nil {
			t.Fatal(err)
		}
		input, err := Decode(data)
		if err != nil {
			t.Fatalf("carried=%v refused: %v", carried, err)
		}
		if passed := Project(input).Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed; passed != carried {
			t.Fatalf("carried=%v projected passing=%v", carried, passed)
		}
	}
}

// PWP-V0-013: test-validity accepts the retained receipt of a keep-reporters
// run, keeps its binding and recomputes an abstaining projection.
func TestKeepReportersRetainedReceiptAccepted(t *testing.T) {
	data, err := jstestprovider.EncodeQualified(keptReceipt())
	if err != nil {
		t.Fatal(err)
	}
	input, err := Decode(data)
	if err != nil {
		t.Fatalf("keep-reporters receipt refused: %v", err)
	}
	doc := Project(input)
	if len(doc.Tests) != 1 || doc.Playwright.External.ProjectReporters == nil || len(doc.Playwright.External.ProjectReporters.Entries) != 3 {
		t.Fatalf("keep-reporters binding not retained: %+v", doc)
	}
	// PWP-V0-012: the mode abstains until its own live qualification; the
	// same receipt without keep-reporters passes.
	if doc.Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("unqualified keep-reporters receipt projected passing")
	}
	control := keptReceipt()
	control.External.ConfigOverride, control.External.ProjectReporters = "controlled-fixture-config", nil
	controlData, err := jstestprovider.EncodeQualified(control)
	if err != nil {
		t.Fatal(err)
	}
	if controlInput, err := Decode(controlData); err != nil || Project(controlInput).Tests[0].Projection.Execution.State != testvalidity.ExecutionPassed {
		t.Fatalf("control receipt did not pass: %v", err)
	}
	forged := bytes.Replace(data, []byte(`"effects":"unknown"`), []byte(`"effects":"none"`), 1)
	if input, err := Decode(forged); err == nil && Project(input).Tests[0].Projection.Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("forged reporter effects projected passing")
	}
	r := keptReceipt()
	r.External.ProjectReporters = nil
	if _, err := jstestprovider.EncodeQualified(r); err == nil {
		t.Fatal("keep-reporters config without its binding encoded")
	}
}
