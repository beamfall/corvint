// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-9730 (V1-4968): a binary whose Go source differs from the analyzed
// checkout's HEAD must not return its plan with exit 0, because that plan can
// select fewer tests than the checkout's own build would and a consumer that
// reads only stdout and the exit status takes it for the current selection.
// Every affected form and corvint test exit 2 with the self-binary-stale code
// and a fix: line, printing no plan; --accept-stale-binary restores the
// degraded plan; a fresh binary and an unverified one are not refused.
func TestStaleBinaryRefusesThePlanUnlessAccepted_AFPV09730(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	forms := [][]string{
		nil,
		{"--full"},
		{"--summary"},
		{"--format", "text"},
		{"--summary", "--format", "text"},
		{"--by-path", "--path", "self.go"},
		{"--by-path", "--path", "self.go", "--format", "text"},
		{"--base", built},
	}
	for _, options := range forms {
		stdout, stderr, code := runAffectedForm(t, root, false, options...)
		if code != 0 || strings.Contains(stdout+stderr, selfBinaryStaleCode) {
			t.Fatalf("%v: a binary built at HEAD was refused or degraded (exit %d):\n%s%s", options, code, stdout, stderr)
		}
	}
	// The source change is the one the stale binary cannot see: its plan is
	// this binary's own and may be narrower than current source gives.
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	freshness := selfBinaryStale(context.Background(), root)
	if freshness.Code != selfBinaryStaleCode {
		t.Fatalf("the fixture binary is not stale: %+v", freshness)
	}
	for _, options := range forms {
		for _, merged := range []bool{false, true} {
			stdout, stderr, code := runAffectedForm(t, root, merged, options...)
			output := stdout + stderr
			if code != 2 {
				t.Fatalf("%v merged=%v: a stale binary exited %d, want 2:\n%s", options, merged, code, output)
			}
			var refusal struct {
				Code  string `json:"code"`
				Error string `json:"error"`
				OK    bool   `json:"ok"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &refusal); err != nil {
				t.Fatalf("%v merged=%v: the refusal is not the only output:\n%s", options, merged, output)
			}
			if refusal.Code != selfBinaryStaleCode || refusal.OK || !strings.HasPrefix(refusal.Error, freshness.Detail) || !strings.Contains(refusal.Error, "; fix: ") {
				t.Fatalf("%v merged=%v: refusal %+v lacks the code, the detail or its fix line", options, merged, refusal)
			}
			for _, recovery := range []string{"go install ./cmd/corvint", "go run ./cmd/corvint affected", "--accept-stale-binary"} {
				if !strings.Contains(refusal.Error, recovery) {
					t.Fatalf("%v: the fix line lacks %q: %s", options, recovery, refusal.Error)
				}
			}
			if !merged && stdout != "" {
				t.Fatalf("%v: a refused plan still wrote stdout:\n%s", options, stdout)
			}
		}
		// Accepted, the plan is the AFP-V0-846 degraded plan: exit 0 and the
		// degradation named in the document, never silently.
		stdout, stderr, code := runAffectedForm(t, root, false, append([]string{"--accept-stale-binary"}, options...)...)
		if code != 0 || !strings.Contains(stderr, "corvint: DEGRADED "+selfBinaryStaleCode) || !strings.Contains(stdout, selfBinaryStaleCode) {
			t.Fatalf("%v --accept-stale-binary: exit %d, want the degraded plan:\n%s%s", options, code, stdout, stderr)
		}
	}
	if _, stderr, code := runAffectedForm(t, root, false, "--accept-stale-binary", "--accept-stale-binary"); code != 2 || !strings.Contains(stderr, "--accept-stale-binary: does not accept a value or repetition") {
		t.Fatalf("a repeated flag was accepted: exit %d %s", code, stderr)
	}

	// affected jobs and affected audit (AFP-V0-11033): the same refusal and
	// opt-in as the plan forms; accepted, the job plan and the audit report
	// carry the degradations member, on separate and on merged streams.
	commitSelfBinaryFiles(t, root, map[string]string{".corvint/ci-job-inputs.json": `{"profile":"corvint-ci-job-inputs/0","jobs":[{"workflow":".github/workflows/ci.yml","job":"self","paths":["self.go"]}]}` + "\n"})
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	manifest := filepath.Join(t.TempDir(), "changes.json")
	if err := os.WriteFile(manifest, []byte(`{"profile":"corvint-affected-audit-input/0","changes":[{"id":"self","base":"`+built+`","head":"`+head+`"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, form := range []struct {
		options  []string
		accepted int
	}{
		{[]string{"jobs", "--base", built}, 0},
		{[]string{"audit", "--changes", manifest}, 3},
	} {
		for _, merged := range []bool{false, true} {
			stdout, stderr, code := runAffectedForm(t, root, merged, form.options...)
			output := stdout + stderr
			var refusal struct {
				Code  string `json:"code"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &refusal); code != 2 || err != nil || refusal.Code != selfBinaryStaleCode || !strings.Contains(refusal.Error, "go run ./cmd/corvint affected "+form.options[0]+" ...") || !strings.Contains(refusal.Error, "--accept-stale-binary") || (!merged && stdout != "") {
				t.Fatalf("affected %v merged=%v: a stale binary exited %d, want 2 with only the refusal:\n%s", form.options, merged, code, output)
			}
		}
		for _, merged := range []bool{false, true} {
			stdout, stderr, code := runAffectedForm(t, root, merged, append(append([]string{}, form.options...), "--accept-stale-binary")...)
			var document struct {
				Degradations       []string          `json:"degradations"`
				DegradationDetails map[string]string `json:"degradationDetails"`
			}
			if err := json.Unmarshal([]byte(stdout), &document); code != form.accepted || err != nil || len(document.Degradations) != 1 || document.Degradations[0] != selfBinaryStaleCode || !strings.Contains(document.DegradationDetails[selfBinaryStaleCode], "go run ./cmd/corvint") {
				t.Fatalf("affected %v --accept-stale-binary merged=%v: exit %d, want %d with the degraded document (%v):\n%s%s", form.options, merged, code, form.accepted, err, stdout, stderr)
			}
			if merged == strings.Contains(stdout+stderr, "corvint: DEGRADED "+selfBinaryStaleCode) {
				t.Fatalf("affected %v --accept-stale-binary merged=%v: the stderr notice is printed exactly when the streams differ:\n%s%s", form.options, merged, stdout, stderr)
			}
		}
		if _, stderr, code := runAffectedForm(t, root, false, append(append([]string{}, form.options...), "--accept-stale-binary", "--accept-stale-binary")...); code != 2 || !strings.Contains(stderr, "--accept-stale-binary: does not accept a value or repetition") {
			t.Fatalf("affected %v: a repeated flag was accepted: exit %d %s", form.options, code, stderr)
		}
	}

	// corvint test: the same refusal, the same opt-in.
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "`+selfBinaryStaleCode+`"`) || !strings.Contains(stderr.String(), "go run ./cmd/corvint test") || strings.Contains(stderr.String(), "corvint-test: DEGRADED") {
		t.Fatalf("corvint test --dry-run with a stale binary exited %d, want 2 with the refusal:\n%s%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run", "--accept-stale-binary"}, strings.NewReader(""), &stdout, &stderr)
	if code == 2 || !strings.Contains(stderr.String(), "corvint-test: DEGRADED "+selfBinaryStaleCode) {
		t.Fatalf("corvint test --dry-run --accept-stale-binary exited %d without the degraded plan:\n%s%s", code, stdout.String(), stderr.String())
	}

	// A binary whose commit cannot be compared is unverified, not stale: it
	// keeps the AFP-V0-846 degradation and is not refused (retained limit).
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", strings.Repeat("ab", 20)))
	receipt, _, stderr2, code := runAffectedCLI(t, root)
	if degradations, _ := receipt["degradations"].([]any); code != 0 || len(degradations) != 1 || degradations[0] != selfBinaryUnverifiedCode || !strings.Contains(stderr2, "corvint: DEGRADED "+selfBinaryUnverifiedCode) {
		t.Fatalf("an unverified binary was refused or lost its degradation: exit %d %v %s", code, receipt["degradations"], stderr2)
	}
}
