package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/linecov"
)

// GLS-V0-040/044: the workflow needs the proposed SHADOW lists for its
// comparison, while the plan retains the state and reason that forbid
// execution narrowing. Unknown states still keep the original lists.
func TestWorkflowShadowComparisonPreservesAuthority(t *testing.T) {
	for _, state := range []string{linecov.StateShadow, linecov.StateFallback, "UNKNOWN"} {
		t.Run(state, func(t *testing.T) {
			sel := linecov.Selection{Profile: linecov.SelectionProfile, State: state, Reason: "unqualified", Detail: "no qualification record",
				Units: []linecov.UnitSelection{
					{Unit: "go:m/drop", Mode: linecov.Exclude},
					{Unit: "go:m/tests", Mode: linecov.Tests, Run: "^(TestA)$"},
					{Unit: "go:m/split", Mode: linecov.Exclude},
				}}
			out := t.TempDir()
			whole := []string{"m/drop", "m/tests", "m/keep"}
			slices := []string{"-run ^TestSplit$ m/split"}
			if err := ciSelection(selectionFile(t, sel, true), whole, slices, out); err != nil {
				t.Fatal(err)
			}
			read := func(name string) []byte {
				t.Helper()
				data, err := os.ReadFile(filepath.Join(out, name))
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			var plan CIPlan
			if err := json.Unmarshal(read("plan.json"), &plan); err != nil {
				t.Fatal(err)
			}
			if plan.State != state || plan.Reason != sel.Reason || plan.Detail != sel.Detail {
				t.Fatalf("report changed authority or lost its reason: %+v", plan)
			}
			wantPackages, wantSlices := strings.Join(whole, "\n")+"\n", slices[0]+"\n"
			if state == linecov.StateShadow {
				wantPackages, wantSlices = "m/keep\n", "-run ^(TestA)$ m/tests\n"+wantSlices
				if strings.Join(plan.Excluded, ",") != "m/drop" || strings.Join(plan.Sliced, ",") != "m/tests" {
					t.Fatalf("shadow comparison lost proposed exclusions: %+v", plan)
				}
			}
			if string(read("packages")) != wantPackages || string(read("slices")) != wantSlices {
				t.Fatal("comparison lists differ from the proposed shadow or fallback")
			}
		})
	}
}

// GLS-V0-034, GLS-V0-009: the CI workflow checks the selector's qualification
// under the runner the shards and the index name: the same build settings, the
// 50m test deadline and the test-confine wrapper. The workflow's own
// invocation, run against a record qualified under the wrapper, admits it;
// the same invocation without its -exec flag computes another runner
// identity and is refused, which in CI keeps narrowing in SHADOW.
func TestWorkflowQualifiedInvocationMatchesWrapperRecord(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	const binary = `"$RUNNER_TEMP/linecov" qualified `
	const wrapperToken = `"$RUNNER_TEMP/test-confine"`
	var line, affected string
	shardExec := 0
	for _, l := range strings.Split(string(data), "\n") {
		switch {
		case strings.Contains(l, binary):
			if line != "" {
				t.Fatalf("two qualified invocations:\n%s\n%s", line, l)
			}
			line = l
		case strings.Contains(l, `"$RUNNER_TEMP/linecov-corvint" --root`):
			affected = l
		case strings.Contains(l, "go test -exec "+wrapperToken+" ") && strings.Contains(l, "-timeout 50m"):
			shardExec++
		}
	}
	if line == "" || affected == "" || shardExec == 0 {
		t.Fatalf("workflow lines not found: qualified %q, affected %q, shards %d", line, affected, shardExec)
	}
	head, rest, _ := strings.Cut(line, binary)
	argText, _, ok := strings.Cut(rest, ")")
	if !ok {
		t.Fatalf("unterminated qualified invocation %q", line)
	}
	_, envText, ok := strings.Cut(head, "&& ")
	if !ok {
		t.Fatalf("no environment before the qualified invocation %q", line)
	}
	// The check runs under the build settings the affected call (and so the
	// shard) passes to its runner identity.
	if !strings.Contains(affected, strings.TrimSpace(envText)+` "$RUNNER_TEMP/linecov-corvint"`) {
		t.Fatalf("qualified runs under %q, affected under %q", envText, affected)
	}
	for _, f := range strings.Fields(envText) {
		name, value, ok := strings.Cut(f, "=")
		if !ok {
			t.Fatalf("unexpected word %q before the qualified invocation", f)
		}
		t.Setenv(name, value)
	}

	root := t.TempDir()
	for _, p := range qualifiedSources {
		write(t, root, p, "package x // "+p+"\n")
	}
	run(t, root, "git", "init", "-q")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "test-confine")
	if err := execfile.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The index workflow collects with --line-test-timeout 50m under the wrapper.
	runnerID, _, err := runner(root, 50*time.Minute, wrapper)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, p := range qualifiedSources {
		sources[p] = strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD:"+p))
	}
	r := QualReport{Sources: sources, Profile: QualificationProfile, Seed: 7, Index: ReportIndex{Commit: "c", Digest: "sha256:d", Runner: runnerID},
		Packages: []string{"p"}, Suite: []string{"p"}, Start: 0, Sites: 4, Order: 10,
		Controls: []MutantResult{{Site: Site{Path: "p/p.go", Kind: "statement", Operator: "delete"}, State: "control-passed"}}}
	for i := 0; i < 4; i++ {
		r.Mutants = append(r.Mutants, MutantResult{Index: i, Site: Site{Path: "p/p.go", Kind: "statement", Operator: "delete"}, State: "killed", KeptCount: 1, Failing: []string{"p.TestA"}})
	}
	report := filepath.Join(dir, "report.json")
	if err := writeJSON(report, r); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(dir, "qualification.json")
	if err := summarizeReports(root, record, []string{report}); err != nil {
		t.Fatal(err)
	}

	var args, bare []string
	words := strings.Fields(argText)
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch w {
		case ".":
			w = root
		case `"$d/qualification.json"`:
			w = record
		case `"$tools"`:
			w = "HEAD"
		case wrapperToken:
			w = wrapper
		default:
			if strings.ContainsAny(w, `"$`) {
				t.Fatalf("unmapped word %q in the qualified invocation %q", w, line)
			}
		}
		args = append(args, w)
		if words[i] == "-exec" && i+1 < len(words) {
			if words[i+1] != wrapperToken {
				t.Fatalf("qualified -exec %s, the shards run under %s", words[i+1], wrapperToken)
			}
			continue
		}
		if i > 0 && words[i-1] == "-exec" {
			continue
		}
		bare = append(bare, w)
	}
	if len(bare) == len(args) {
		t.Fatalf("the workflow's qualified invocation passes no -exec wrapper: %q", argText)
	}
	if err := qualified(args); err != nil {
		t.Fatalf("the workflow's invocation refuses a record qualified under its wrapper: %v", err)
	}
	if err := qualified(bare); err == nil || !strings.Contains(err.Error(), "runner mismatch") {
		t.Fatalf("an invocation without the wrapper accepted the wrapper-bound record: %v", err)
	}
	if timeout, ok := linecov.RunnerTestTimeout(runnerID); !ok || timeout != 50*time.Minute || !strings.Contains(runnerID, `exec="sha256:`) {
		t.Fatalf("the record's runner does not bind the wrapper and deadline: %q", runnerID)
	}
}
