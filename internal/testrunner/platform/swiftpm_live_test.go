//go:build darwin

package platform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// swiftPMLiveFixture returns the operator-built SwiftPM package and pinned swift
// executable. Nothing is downloaded; absent inputs skip the actual witness.
func swiftPMLiveFixture(t *testing.T) (root, exe, sha, out string, inputs map[string]string) {
	t.Helper()
	root, exe = os.Getenv("CORVINT_SWIFTPM_LIVE_ROOT"), os.Getenv("CORVINT_SWIFTPM_LIVE_EXE")
	if root == "" || exe == "" {
		t.Skip("operator-built SwiftPM XCTest fixture and swift executable not supplied")
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	sha = tr.Digest(b)
	inputs = map[string]string{}
	for _, n := range []string{"Package.swift", "Sources/Proof/Proof.swift", "Tests/ProofTests/Proof.swift", "Tests/ProofTests/Hang.swift"} {
		b, e := os.ReadFile(filepath.Join(root, n))
		if e != nil {
			t.Fatal(e)
		}
		inputs[n] = tr.Digest(b)
	}
	out = os.Getenv("CORVINT_SWIFTPM_LIVE_OUT")
	if out == "" {
		if out, e = os.MkdirTemp("", "swiftpm-live-"); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("retained live evidence: %s", out)
	return root, exe, sha, out, inputs
}

func retainSwiftPMLive(t *testing.T, out, name string, r tr.Request, v tr.Invocation, x tr.Execution, o tr.Observation, execErr, parseErr error) {
	t.Helper()
	errText := func(e error) string {
		if e == nil {
			return ""
		}
		return e.Error()
	}
	raw, e := json.MarshalIndent(struct {
		Request      tr.Request
		Invocation   tr.Invocation
		Execution    tr.Execution
		Stdout       string
		Observation  tr.Observation
		ExecuteError string
		ParseError   string
	}{r, v, x, string(x.Input.Stdout), o, errText(execErr), errText(parseErr)}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(out, name+".json"), append(raw, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}

// TestSwiftPMXCTestLiveThreeOutcomes is the actual TRE-V0-024 witness: the
// serial native transport keeps pass, assertion failure and XCTSkip distinct
// through the shared executor, where SwiftPM's xUnit writer reports the skip
// as an ordinary pass.
func TestSwiftPMXCTestLiveThreeOutcomes(t *testing.T) {
	root, exe, sha, out, inputs := swiftPMLiveFixture(t)
	r := tr.Request{Runner: "swift-xctest", Root: root, Executable: exe, ExecutableSha256: sha, Config: filepath.Join(root, "Package.swift"), ConfigSha256: inputs["Package.swift"], InputFiles: inputs, ReportDir: filepath.Join(out, "three"), TimeoutSeconds: 600, Selectors: []string{"ProofTests.Proof/testPass", "ProofTests.Proof/testFail", "ProofTests.Proof/testSkip"}}
	v, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	x, execErr := tr.Execute(context.Background(), r, v)
	o, parseErr := Parse(x.Input)
	o = tr.Normalize(x.Input, o)
	retainSwiftPMLive(t, out, "three", r, v, x, o, execErr, parseErr)
	if execErr != nil || parseErr != nil || x.Input.ExitCode != 1 || !o.Complete || len(o.Tests) != 3 {
		t.Fatalf("execute=%v parse=%v exit=%d observation=%+v", execErr, parseErr, x.Input.ExitCode, o)
	}
	want := map[string]string{"ProofTests.Proof/testPass": tr.Passed, "ProofTests.Proof/testFail": tr.Failed, "ProofTests.Proof/testSkip": tr.Skipped}
	for _, row := range o.Tests {
		if want[row.ID] != row.State {
			t.Fatalf("%s: got %s want %s", row.ID, row.State, want[row.ID])
		}
		if row.State == tr.Failed && row.FailureKind != tr.Assertion {
			t.Fatalf("assertion failure kind lost: %+v", row)
		}
	}
}
