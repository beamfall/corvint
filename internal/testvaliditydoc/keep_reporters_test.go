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
	bound := identity
	bound.Argv = []string{"npx", "playwright", "--config=/repo/config.cjs", "--project=one"}
	raw, _ := json.Marshal(struct {
		Identity jstestprovider.Identity
		Project  *jstestprovider.ProjectIdentity
		Anchor   *jstestprovider.Anchor
		FullName string
	}{bound, test.Project, test.Anchor, test.FullName})
	digest := sha256.Sum256(raw)
	test.ID = hex.EncodeToString(digest[:])
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

// PWP-V0-013: test-validity accepts the retained receipt of a keep-reporters
// run, keeps its binding, and refuses a binding whose effects claim trust.
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
	if len(doc.Tests) != 1 || doc.Tests[0].Projection.Execution.State != testvalidity.ExecutionPassed || doc.Playwright.External.ProjectReporters == nil || len(doc.Playwright.External.ProjectReporters.Entries) != 3 {
		t.Fatalf("keep-reporters receipt not projected passing with its binding: %+v", doc)
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
