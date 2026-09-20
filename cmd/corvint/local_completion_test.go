package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

// This fixture compiles the actual native verifier from two real Git trees and
// runs the repository's actual coordinator/checker scripts. No fake gate or
// verifier result supplies its positive completion verdict.
func localCompletionRepo(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := moduleRoot(t)
	// Only the verifier's import closure: the same binary builds, and the
	// fixture's copy, git add and commit stop carrying the rest of internal/.
	for _, directory := range repositoryImportClosure(t, source, "cmd/corvint") {
		entries, err := os.ReadDir(filepath.Join(source, directory))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			relative := filepath.Join(directory, entry.Name())
			selected := strings.HasSuffix(relative, ".go") && !strings.HasSuffix(relative, "_test.go") || relative == "internal/gokernel/host-schema.json" || relative == "internal/betarung/admissions.json" || relative == "internal/jstestprovider/qualified-reporter.cjs"
			if entry.IsDir() || !selected {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(source, relative))
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(root, relative)
			if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(destination, raw, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, relative := range []string{"go.mod", "VERSION", "script/dogfood-change.sh", "script/dogfood-check.sh"} {
		raw, err := os.ReadFile(filepath.Join(source, relative))
		if err != nil {
			t.Fatal(err)
		}
		cemWrite(t, root, relative, string(raw))
	}
	cemWrite(t, root, ".gitignore", ".corvint/\n.context-corvint/\n")
	cemWrite(t, root, "AGENTS.md", "# Fixture authority\nUse intent.md to govern the fixture.\n")
	cemWrite(t, root, "intent.md", "# Intent\n\n## Requirements\n\n- `FIXTURE-LCP-001`: The answer is two.\n- `FIXTURE-LCP-002`: The existing companion behavior remains unchanged.\n")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 1 }\n")
	cemWrite(t, root, "fixture/fixture_test.go", "package fixture\nimport \"testing\"\nfunc TestAnswer(t *testing.T) {\n t.Run(\"FIXTURE-LCP-001\",func(t *testing.T){ if Answer()!=2 { t.Fatal(\"answer\") } })\n}\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n")
	cemGit(t, root, "add", "fixture/fixture.go")
	cemGit(t, root, "commit", "-qm", "change")
	return root, base
}

func publicLocal(t *testing.T, root string, args ...string) {
	t.Helper()
	var stdout, stderr strings.Builder
	if code := localCompletionPublicCommand(context.Background(), root, args, &stdout, &stderr); code != 0 {
		t.Fatalf("public %v exit=%d stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
	}
}

func TestLocalCompletionRealEvidenceWorkflow(t *testing.T) {
	t.Parallel()
	root, base := localCompletionRepo(t)
	key := localcompletion.HashSession(t.Name())
	ctx := context.Background()
	plan := localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture"}, TimeoutSeconds: 60}}}
	raw, _ := json.Marshal(plan)
	if _, err := localcompletion.Begin(ctx, root, key, raw); err != nil {
		t.Fatal(err)
	}
	t.Run("LCP-V0-005 binding", func(t *testing.T) {
		if _, err := localcompletion.Verify(ctx, root, key, "actual-test"); err != nil {
			t.Fatal(err)
		}
		result, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err == nil || result.Satisfied {
			t.Fatal("missing CEM accepted")
		}
		publicLocal(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
		publicLocal(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
		cemGit(t, root, "add", "-f", ".corvint/change.cem.json")
		cemGit(t, root, "commit", "-qm", "bind evidence")
		target := cemGit(t, root, "rev-parse", "HEAD")
		publicLocal(t, root, "ocm", "prepare", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--intent", "intent.md", "--expected-base", base, "--target", target)
		if _, err = localcompletion.Verify(ctx, root, key, "actual-test"); err != nil {
			t.Fatal(err)
		}
		publicLocal(t, root, "ocm", "mark", "--map", ".corvint/change.ocm.001.json", "--obligation", "FIXTURE-LCP-001", "--reason", "no-test-claim")
		incomplete, finishErr := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if finishErr == nil || finishErr.Error() != "ocm-bindings-required" || len(incomplete.Evidence) < 2 || incomplete.Evidence[1].Tool != "ocm-status" || len(incomplete.Evidence[1].Worklist) != 2 {
			t.Fatalf("OCM gap hidden: %#v %v", incomplete.Evidence, finishErr)
		}
		gap := incomplete.Evidence[1].Worklist[0].(map[string]any)
		if gap["id"] != "FIXTURE-LCP-001" || gap["disposition"] != "unknown" || gap["reason"] != "no-test-claim" {
			t.Fatalf("assessed gap rewritten: %#v", gap)
		}
		for _, action := range incomplete.NextActions {
			if len(action) > 1 && action[1] == "ocm" && strings.Contains(strings.Join(action, " "), "--max-unknown") {
				t.Fatal("next action applies zero-unknown policy to untouched obligations")
			}
		}
		publicLocal(t, root, "ocm", "link", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--obligation", "FIXTURE-LCP-001", "--hunk", "1", "--test-path", "fixture/fixture_test.go", "--claim", "test:TestAnswer/case:fixture-lcp", "--expected-base", base, "--target", target)
	})
	var reviewed localcompletion.Evaluation
	t.Run("LCP-V0-006 inspection", func(t *testing.T) {
		result, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err != nil {
			t.Fatal(err)
		}
		if result.Satisfied || result.ReportSetDigest == "" || len(result.Reports) != 2 {
			t.Fatalf("reports: %#v", result)
		}
		if len(result.NextActions) != 1 || strings.Join(result.NextActions[0], " ") != "corvint dogfood review --session-key "+key+" --report-set "+result.ReportSetDigest {
			t.Fatalf("review is the next prerequisite: %v", result.NextActions)
		}
		assertLocalCompletionUnassessed(t, root, base, result.Target)
		report, err := os.ReadFile(result.Reports[1])
		if err != nil || !strings.Contains(string(report), "`2` total, `1` linked, `1` unknown") || !strings.Contains(string(report), "`FIXTURE-LCP-002` | `unknown` | `unassessed` | 0 | 0") {
			t.Fatalf("report hides unassessed obligation: %s %v", report, err)
		}
		if result.Plan == nil || len(result.Checks) != 1 || !result.Checks[0].Qualified || !filepath.IsAbs(result.Checks[0].Argv[0]) || result.Checks[0].TestedCommit != result.Target || result.Checks[0].Stdout == "" {
			t.Fatal("review hides actual check provenance")
		}
		if _, err = os.Stat(filepath.Join(root, ".git/corvint/local-outcome.json")); !os.IsNotExist(err) {
			t.Fatal("outcome recorded before review")
		}
		if _, err = localcompletion.Review(ctx, root, key, strings.Repeat("0", 64)); err == nil {
			t.Fatal("wrong review digest accepted")
		}
		original, err := os.ReadFile(result.Reports[0])
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(result.Reports[0], append(original, []byte("stale")...), 0600)
		if _, err = localcompletion.Review(ctx, root, key, result.ReportSetDigest); err == nil {
			t.Fatal("changed report accepted")
		}
		os.WriteFile(result.Reports[0], original, 0600)
		reviewed, err = localcompletion.Review(ctx, root, key, result.ReportSetDigest)
		if err != nil {
			t.Fatal(err)
		}
		if reviewed.Satisfied {
			t.Fatal("inspection alone satisfied")
		}
		if len(reviewed.NextActions) != 1 || strings.Join(reviewed.NextActions[0], " ") != "corvint dogfood finish --session-key "+key {
			t.Fatalf("finish after review: %v", reviewed.NextActions)
		}
	})
	t.Run("LCP-V0-007 completion", func(t *testing.T) {
		result, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err != nil {
			dumpLocalCompletionFailure(t, root, key)
			t.Fatal(err)
		}
		if !result.Satisfied || result.Lifecycle != "satisfied" {
			t.Fatalf("not satisfied: %#v", result)
		}
		if len(result.NextActions) != 0 {
			t.Fatalf("satisfied enrollment suggests more work: %v", result.NextActions)
		}
		assertLocalCompletionUnassessed(t, root, base, result.Target)
		outcome := filepath.Join(root, ".git/corvint/local-outcome.json")
		before, err := os.ReadFile(outcome)
		if err != nil {
			t.Fatal(err)
		}
		var outcomeDocument struct {
			Trace struct {
				Verification []string `json:"verification"`
			} `json:"trace"`
		}
		json.Unmarshal(before, &outcomeDocument)
		if len(outcomeDocument.Trace.Verification) != 1 || !strings.HasPrefix(outcomeDocument.Trace.Verification[0], "local-completion-checks-sha256:") {
			t.Fatal("missing bounded check provenance reference")
		}
		result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err != nil || !result.Satisfied {
			t.Fatalf("repeat: %#v %v", result, err)
		}
		after, _ := os.ReadFile(outcome)
		if string(before) != string(after) {
			t.Fatal("repeat changed outcome")
		}
		// A removed terminal artifact list must not disable evidence checking.
		statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
		stateRaw, _ := os.ReadFile(statePath)
		var state map[string]any
		json.Unmarshal(stateRaw, &state)
		state["terminal"].(map[string]any)["artifacts"] = []any{}
		altered, _ := json.Marshal(state)
		os.WriteFile(statePath, altered, 0600)
		result, err = localcompletion.Evaluate(ctx, root, key)
		if err != nil || result.Satisfied {
			t.Fatalf("missing terminal artifacts accepted: %#v %v", result, err)
		}
		os.WriteFile(statePath, stateRaw, 0600)
		// A failed real checker remains failure even if it can see a previously
		// true outputsAgree. The successful observation itself is authoritative
		// only for this local workflow; saved stdout alone cannot fabricate it.
		state["terminal"] = nil
		state["lifecycle"] = "active"
		altered, _ = json.Marshal(state)
		os.WriteFile(statePath, altered, 0600)
		cemGit(t, root, "config", "--local", "corvint.dogfood.anchor", "HEAD")
		result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err == nil || result.Satisfied {
			t.Fatal("checker nonzero accepted")
		}
		cemGit(t, root, "config", "--local", "--unset", "corvint.dogfood.anchor")
	})
}

func assertLocalCompletionUnassessed(t *testing.T, root, base, target string) {
	t.Helper()
	aggregate, err := dogfoodocm.Status(context.Background(), dogfoodocm.Options{Root: root, CEMPath: ".corvint/change.cem.json", ExpectedBase: base, Target: target})
	if err != nil || !aggregate.OK || aggregate.Aggregate.Coverage != (dogfoodocm.Coverage{Total: 2, Linked: 1, Unknown: 1}) || len(aggregate.Worklist) != 2 {
		t.Fatalf("raw aggregate counts changed: %#v %v", aggregate, err)
	}
	companion := aggregate.Worklist[1]
	if companion.ID != "FIXTURE-LCP-002" || companion.Disposition != "unknown" || companion.Reason != "unassessed" || len(companion.ClaimIDs) != 0 || len(companion.HunkIDs) != 0 {
		t.Fatalf("untouched obligation was inferred or rewritten: %#v", companion)
	}
}

func dumpLocalCompletionFailure(t *testing.T, root, key string) {
	t.Helper()
	raw, _ := os.ReadFile(filepath.Join(root, ".git/corvint/local-completion", key, "state.json"))
	var saved struct {
		Generation string `json:"generation"`
	}
	json.Unmarshal(raw, &saved)
	for _, name := range []string{filepath.Join(root, ".git/corvint/local-completion", key, saved.Generation, "coordination-time.stderr"), filepath.Join(root, ".git/corvint/local-completion", key, saved.Generation, "coordination-time.stdout"), filepath.Join(root, ".corvint/dogfood-report.json"), filepath.Join(root, ".git/corvint/local-completion", key, saved.Generation, "final-check.stderr")} {
		raw, err := os.ReadFile(name)
		if err == nil {
			t.Logf("%s: %s", filepath.Base(name), raw)
		}
	}
}

func TestLocalCompletionCLIRejectsUnknownAndSecretInputs(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{}, {"nope"}, {"verify", "--check", "x", "--check", "y"}, {"status", "--plan", "x"}} {
		var stdout, stderr strings.Builder
		if runLocalCompletion(context.Background(), t.TempDir(), args, strings.NewReader(""), &stdout, &stderr) != 2 {
			t.Fatalf("accepted %v", args)
		}
	}
}
