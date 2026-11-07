// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	json "encoding/json/v2"
	"path/filepath"
	"strings"
	"testing"
)

// TestAffectedJobsCLI is AFP-V0-2341: a documentation-only range skips every
// declared job, a declared input runs its job only, and an invalid
// declaration exits 3 with no job.
func TestAffectedJobsCLI(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	root := materializationFixture(t)
	reuseFixtureWrite(t, root, map[string]string{
		"go.mod":                   "module example.com/m\n\ngo 1.27\n",
		"cmd/x/main.go":            "package main\n\nfunc main() {}\n",
		"interop/m/go.mod":         "module example.com/i\n\ngo 1.27\n",
		"interop/m/a.go":           "package i\n",
		"docs/a.md":                "# a\n",
		".github/workflows/ci.yml": "name: CI\n",
		".corvint/ci-job-inputs.json": `{"profile":"corvint-ci-job-inputs/0","jobs":[` +
			`{"workflow":".github/workflows/ci.yml","job":"go-interop","paths":["interop/m/"]},` +
			`{"workflow":".github/workflows/ci.yml","job":"archive","goPackages":{"packages":["./cmd/x"],"platforms":["linux/amd64"]},"exportedTree":true}]}` + "\n",
	})
	base := reuseFixtureCommit(t, root, "base")
	plan := func(wantCode int, arguments ...string) map[string]string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"--root", root, "affected", "jobs"}, arguments...), nil, &stdout, &stderr)
		if code != wantCode {
			t.Fatalf("affected jobs %v: exit %d, want %d\n%s%s", arguments, code, wantCode, stdout.String(), stderr.String())
		}
		if wantCode == 2 {
			return nil
		}
		var document struct {
			Profile          string `json:"profile"`
			DeclarationError string `json:"declarationError"`
			Jobs             []struct {
				Job      string `json:"job"`
				Decision string `json:"decision"`
				Reason   string `json:"reason"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &document, json.RejectUnknownMembers(false)); err != nil || document.Profile != "corvint-ci-job-plan/0" {
			t.Fatalf("plan: %v\n%s", err, stdout.String())
		}
		decided := map[string]string{"error": document.DeclarationError}
		for _, job := range document.Jobs {
			decided[job.Job] = job.Decision + " " + job.Reason
		}
		return decided
	}
	reuseFixtureWrite(t, root, map[string]string{"docs/a.md": "# changed\n"})
	reuseFixtureCommit(t, root, "docs")
	got := plan(0, "--base", base)
	if got["go-interop"] != "SKIP NO_DECLARED_INPUT_CHANGED" || got["archive"] != "SKIP NO_DECLARED_INPUT_CHANGED" {
		t.Fatalf("documentation-only range: %v", got)
	}
	reuseFixtureWrite(t, root, map[string]string{"interop/m/a.go": "package i\n\nvar X = 1\n"})
	reuseFixtureCommit(t, root, "interop")
	got = plan(0, "--base", base)
	if got["go-interop"] != "RUN INPUT_PATH_CHANGED" || got["archive"] != "SKIP NO_DECLARED_INPUT_CHANGED" {
		t.Fatalf("interop range: %v", got)
	}
	reuseFixtureWrite(t, root, map[string]string{"cmd/x/main.go": "package main\n\nfunc main() { println() }\n"})
	got = plan(0, "--base", base)
	if got["archive"] != "RUN GO_PACKAGE_DIRECTORY_CHANGED" {
		t.Fatalf("a dirty closure file: %v", got)
	}
	reuseFixtureWrite(t, root, map[string]string{"bad.json": "{}"})
	if got = plan(3, "--base", base, "--declaration", filepath.Join(root, "bad.json")); !strings.HasPrefix(got["error"], "DECLARATION_INVALID") || len(got) != 1 {
		t.Fatalf("invalid declaration: %v", got)
	}
	plan(2)
	plan(2, "--base", base, "--unknown")
}
