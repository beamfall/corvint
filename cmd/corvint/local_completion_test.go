package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

// This fixture runs the actual in-process coordinator and checker (LCP-V0-014)
// over a real Git tree that still carries the verifier source and the former
// scripts, which finish must no longer read. No fake gate or verifier result
// supplies its positive completion verdict.
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
	embeddedReporter := "internal/jstestprovider/qualified-reporter.cjs"
	if _, err := os.Stat(filepath.Join(root, embeddedReporter)); err != nil {
		t.Fatalf("embedded fixture asset %s: %v", embeddedReporter, err)
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
		if len(result.NextActions) != 0 || result.NoActionReason != "workflow-satisfied" {
			t.Fatalf("LCP-V0-020 satisfied reason: %#v", result)
		}
		for _, action := range []string{"status", "finish"} {
			code, stdout, stderr := runCLI(t, "--root", root, "dogfood", action, "--session-key", key)
			var document struct {
				Policy localcompletion.Evaluation `json:"policy"`
			}
			if err := json.Unmarshal([]byte(stdout), &document); err != nil || code != 0 || document.Policy.NoActionReason != "workflow-satisfied" || len(document.Policy.NextActions) != 0 {
				t.Fatalf("LCP-V0-020 satisfied %s: exit=%d stdout=%s stderr=%s err=%v", action, code, stdout, stderr, err)
			}
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
	t.Run("V1-1894 sealed completion", func(t *testing.T) {
		// Establish a real successful completion before exercising the required seal.
		result, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err != nil || !result.Satisfied {
			t.Fatalf("pre-seal completion: %#v %v", result, err)
		}
		publicLocal(t, root, "dogfood", "seal", base)
		seal := cemGit(t, root, "rev-parse", "HEAD")
		// Reproduce the old Stop worklist: rerunning checks clears the terminal.
		if _, err = localcompletion.Verify(ctx, root, key, "actual-test"); err != nil {
			t.Fatal(err)
		}
		result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err != nil {
			t.Fatalf("post-seal finish: %#v %v", result, err)
		}
		if !result.Satisfied && result.ReportSetDigest != "" {
			if _, err = localcompletion.Review(ctx, root, key, result.ReportSetDigest); err != nil {
				t.Fatal(err)
			}
			result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		}
		if err != nil || !result.Satisfied || result.Target != seal {
			t.Fatalf("sealed completion: %#v %v", result, err)
		}
		if got := cemGit(t, root, "status", "--porcelain"); got != "" {
			t.Fatalf("finish changed sealed worktree: %s", got)
		}
		assertSealedCompletionControls(t, root, key, base, seal, result)
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

// TestLocalCompletionVerifyOKMirrorsCheckResult pins LCP-V0-017 (V1-1012):
// a selected check that exits 1 yields exit 1 and top-level ok:false, with the
// observation still reporting qualified:false and the observed exit; a passing
// check yields exit 0 and ok:true.
func TestLocalCompletionVerifyOKMirrorsCheckResult(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, ".gitignore", ".corvint/\n")
	cemWrite(t, root, "intent.md", "# Intent\n\n## Requirements\n\n- `FIXTURE-001`: preserve behavior.\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	plan := localcompletion.Plan{Base: cemGit(t, root, "rev-parse", "HEAD"), Intents: []string{"intent.md"}, Checks: []localcompletion.Check{
		{ID: "fails", Argv: []string{"sh", "-c", "exit 1"}, TimeoutSeconds: 30},
		{ID: "passes", Argv: []string{"sh", "-c", "exit 0"}, TimeoutSeconds: 30},
	}}
	raw, _ := json.Marshal(plan)
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err = os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	key := localcompletion.HashSession(t.Name())
	run := func(args ...string) (int, map[string]any) {
		var stdout, stderr strings.Builder
		code := runLocalCompletion(context.Background(), root, append(args, "--session-key", key), strings.NewReader(""), &stdout, &stderr)
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("%v exit=%d stdout=%s stderr=%s: %v", args, code, stdout.String(), stderr.String(), err)
		}
		return code, envelope
	}
	if code, _ := run("begin", "--plan", planPath); code != 0 {
		t.Fatalf("begin exit %d", code)
	}
	for _, want := range []struct {
		check     string
		code      int
		qualified bool
		exit      float64
	}{{"fails", 1, false, 1}, {"passes", 0, true, 0}} {
		code, envelope := run("verify", "--check", want.check)
		if code != want.code || envelope["ok"] != (want.code == 0) || envelope["tool"] != "dogfood-verify" {
			t.Fatalf("%s: exit=%d envelope=%v", want.check, code, envelope)
		}
		found := false
		for _, item := range envelope["policy"].(map[string]any)["checks"].([]any) {
			check := item.(map[string]any)
			if check["id"] == want.check {
				found = check["qualified"] == want.qualified && check["exit"] == want.exit
			}
		}
		if !found {
			t.Fatalf("%s: check observation hidden or wrong: %v", want.check, envelope["policy"])
		}
	}
}

// TestDogfoodBeginPlanIntentRefusalNamesTheRule pins LCP-V0-018 (V1-1043): an
// unsorted, duplicated, empty or over-long plan intents array keeps its stable
// code and names the sorted, de-duplicated 1-16 repository-relative spec path
// rule in the nested message.
func TestDogfoodBeginPlanIntentRefusalNamesTheRule(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "a.md", "# A\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	many := make([]string, 17)
	for i := range many {
		many[i] = "docs/spec-" + string(rune('a'+i)) + ".md"
	}
	for _, tc := range []struct {
		name    string
		intents []string
		code    string
	}{
		{"unsorted", []string{"docs/b.md", "docs/a.md"}, "invalid-intent-scope"},
		{"duplicated", []string{"docs/a.md", "docs/a.md"}, "invalid-intent-scope"},
		{"empty", []string{}, "plan-bound-exceeded"},
		{"over-long", many, "plan-bound-exceeded"},
	} {
		plan := localcompletion.Plan{Base: base, Intents: tc.intents, Checks: []localcompletion.Check{{ID: "c", Argv: []string{"true"}, TimeoutSeconds: 30}}}
		raw, _ := json.Marshal(plan)
		planPath := filepath.Join(t.TempDir(), "plan.json")
		if err = os.WriteFile(planPath, raw, 0600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr strings.Builder
		code := runLocalCompletion(context.Background(), root, []string{"begin", "--plan", planPath, "--session-key", localcompletion.HashSession(t.Name())}, strings.NewReader(""), &stdout, &stderr)
		var envelope struct {
			Code  string `json:"code"`
			Error struct {
				Code, Message string
			} `json:"error"`
		}
		if err = json.Unmarshal([]byte(stderr.String()), &envelope); err != nil || code != 2 || stdout.Len() != 0 {
			t.Fatalf("%s: exit=%d stdout=%q stderr=%q: %v", tc.name, code, stdout.String(), stderr.String(), err)
		}
		if envelope.Code != tc.code || envelope.Error.Code != tc.code || !strings.HasPrefix(envelope.Error.Message, tc.code+": ") {
			t.Fatalf("%s: envelope %+v", tc.name, envelope)
		}
		for _, want := range []string{"1-16 repository-relative spec paths", "sorted in byte order with no duplicates"} {
			if !strings.Contains(envelope.Error.Message, want) {
				t.Fatalf("%s: message %q lacks %q", tc.name, envelope.Error.Message, want)
			}
		}
	}
}

// LCP-V0-020: unavailable Frontier authority does not erase workflow guidance.
func TestLocalCompletionFrontierUnavailableNextActionOrReason(t *testing.T) {
	root, key := handoffRepository(t)
	for _, lifecycle := range []string{"active", "inactive", "cancelled"} {
		t.Run(lifecycle, func(t *testing.T) {
			session, reason := key, ""
			if lifecycle == "inactive" {
				session, reason = localcompletion.HashSession("unenrolled"), "enrollment-inactive"
			} else if lifecycle == "cancelled" {
				if _, err := localcompletion.Cancel(context.Background(), root, key, "test cancel"); err != nil {
					t.Fatal(err)
				}
				reason = "enrollment-cancelled"
			}
			evaluation, err := localcompletion.Evaluate(context.Background(), root, session)
			if err != nil {
				t.Fatal(err)
			}
			event := dogfoodEventEnvelope(options{event: "session-start"}, map[string]any{}, gokernel.Repository{}, evaluation)
			frontier := event["frontier"].(map[string]any)
			if frontier["state"] != "UNAVAILABLE" || frontier["reason"] != "frontier-authority-unavailable" {
				t.Fatalf("frontier=%v", frontier)
			}
			for _, action := range []string{"status", "finish"} {
				code, stdout, stderr := runCLI(t, "--root", root, "dogfood", action, "--session-key", session)
				if action == "finish" && lifecycle != "active" {
					var failure struct {
						Code string `json:"code"`
					}
					if err := json.Unmarshal([]byte(stderr), &failure); err != nil || code != 2 || failure.Code == "" || stdout != "" {
						t.Fatalf("named refusal: exit=%d stdout=%s stderr=%s err=%v", code, stdout, stderr, err)
					}
					continue
				}
				var document struct {
					Policy localcompletion.Evaluation `json:"policy"`
				}
				if err := json.Unmarshal([]byte(stdout), &document); err != nil {
					t.Fatal(err)
				}
				got := document.Policy
				wantExit := 0
				if action == "finish" {
					wantExit = 1
				}
				if code != wantExit || got.Lifecycle != lifecycle || got.NoActionReason != reason || (reason == "") != (len(got.NextActions) > 0) {
					t.Fatalf("%s: exit=%d policy=%#v stderr=%s", action, code, got, stderr)
				}
				if reason == "" && strings.Join(got.NextActions[0], " ") != "corvint dogfood verify --session-key "+session+" --check check" {
					t.Fatalf("keyed action: %v", got.NextActions)
				}
			}
		})
	}
}

// LCP-V0-020: pin the additive field without the all-mode enumeration registry.
func TestLocalCompletionInactiveStatusGolden(t *testing.T) {
	document := runCoreMode(t, func(t *testing.T) []string {
		return []string{"--root", impactCLIRepository(t), "dogfood", "status", "--session-key", strings.Repeat("a", 64)}
	}, 0)
	compareCoreGolden(t, "$", readCoreGolden(t, filepath.Join("testdata", "core-freeze", "dogfood-status-retained-outcome.json")), document)
}

// LCP-V0-020: a snapshot failure still explains why a partial policy has no action.
func TestLocalCompletionUnavailableEvaluationReason(t *testing.T) {
	root, key := handoffRepository(t)
	if result, err := localcompletion.Evaluate(context.Background(), root, key); err != nil || len(result.NextActions) == 0 {
		t.Fatalf("fixture must evaluate before injection: %#v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr strings.Builder
	code := runLocalCompletion(ctx, root, []string{"status", "--session-key", key}, strings.NewReader(""), &stdout, &stderr)
	var document struct {
		Policy localcompletion.Evaluation `json:"policy"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &document); err != nil || code != 2 || document.Policy.Lifecycle != "active" || len(document.Policy.NextActions) != 0 || document.Policy.NoActionReason != "evaluation-unavailable" || stderr.Len() == 0 {
		t.Fatalf("partial evaluation: exit=%d stdout=%s stderr=%s err=%v", code, &stdout, &stderr, err)
	}
}

// TestDogfoodBeginCheckBoundNamesTheLimit covers V1-1011 (LCP-V0-032): the
// invalid-check-bound refusal keeps its code and names the per-check bounds and
// the requested values.
func TestDogfoodBeginCheckBoundNamesTheLimit(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "a.md", "# A\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	for _, tc := range []struct {
		name    string
		id      string
		argv    []string
		timeout int
		want    string
	}{
		{"timeout over the cap", "slow", []string{"true"}, 4000, "check slow requested 1 argv elements and timeoutSeconds 4000"},
		{"too many argv elements", "slow", make([]string, 65), 30, "check slow requested 65 argv elements and timeoutSeconds 30"},
		// A secret-shaped id passes the id grammar but is never echoed.
		{"secret-shaped check id", "gh" + "p_" + strings.Repeat("a1B2", 9), []string{"true"}, 4000, "check [REDACTED] requested 1 argv elements"},
	} {
		argv := tc.argv
		for index := range argv {
			if argv[index] == "" {
				argv[index] = "x"
			}
		}
		plan := localcompletion.Plan{Base: base, Intents: []string{"a.md"}, Checks: []localcompletion.Check{{ID: tc.id, Argv: argv, TimeoutSeconds: tc.timeout}}}
		raw, _ := json.Marshal(plan)
		planPath := filepath.Join(t.TempDir(), "plan.json")
		if err = os.WriteFile(planPath, raw, 0600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr strings.Builder
		code := runLocalCompletion(context.Background(), root, []string{"begin", "--plan", planPath, "--session-key", localcompletion.HashSession(t.Name())}, strings.NewReader(""), &stdout, &stderr)
		var envelope struct {
			Code  string `json:"code"`
			Error struct {
				Code, Message string
			} `json:"error"`
		}
		if err = json.Unmarshal([]byte(stderr.String()), &envelope); err != nil || code != 2 || stdout.Len() != 0 {
			t.Fatalf("%s: exit %d stdout %q stderr %q", tc.name, code, stdout.String(), stderr.String())
		}
		if envelope.Code != "invalid-check-bound" || envelope.Error.Code != "invalid-check-bound" {
			t.Fatalf("%s: code drifted: %s", tc.name, stderr.String())
		}
		if tc.id != "slow" && strings.Contains(stderr.String(), tc.id) {
			t.Fatalf("%s: refusal echoed the check id: %s", tc.name, stderr.String())
		}
		for _, want := range []string{"1-64 argv elements", "timeoutSeconds 1-3600", tc.want} {
			if !strings.Contains(envelope.Error.Message, want) {
				t.Fatalf("%s: message %q lacks %q", tc.name, envelope.Error.Message, want)
			}
		}
	}
}

// TestDogfoodCancelActiveEnrollmentRequiresReason covers LCP-V0-038 (V1-2177):
// the CLI refuses a bare cancel of an active enrollment with the counts and the
// fresh-worktree route, and a --reason cancel succeeds and echoes the reason.
func TestDogfoodCancelActiveEnrollmentRequiresReason(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "a.md", "# A\n\n## Requirements\n\n- `FIXTURE-001`: preserve behavior.\n")
	cemWrite(t, root, ".gitignore", ".corvint/\n")
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	plan := localcompletion.Plan{Base: cemGit(t, root, "rev-parse", "HEAD"), Intents: []string{"a.md"}, Checks: []localcompletion.Check{{ID: "unit", Argv: []string{"true"}, TimeoutSeconds: 30}}}
	raw, _ := json.Marshal(plan)
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err = os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	key := localcompletion.HashSession(t.Name())
	call := func(args ...string) (int, string, string) {
		var stdout, stderr strings.Builder
		code := runLocalCompletion(context.Background(), root, append(args, "--session-key", key), strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := call("begin", "--plan", planPath); code != 0 {
		t.Fatalf("begin: exit %d stdout %s stderr %s", code, stdout, stderr)
	}
	code, stdout, stderr := call("cancel")
	var envelope struct {
		Code  string                         `json:"code"`
		Error struct{ Code, Message string } `json:"error"`
	}
	if err = json.Unmarshal([]byte(stderr), &envelope); err != nil || code != 2 || stdout != "" || envelope.Code != "cancel-reason-required" {
		t.Fatalf("bare cancel: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	for _, want := range []string{"--reason TEXT", "fresh worktree or clone", "closes 1 selected checks and 0 saved observations"} {
		if !strings.Contains(envelope.Error.Message, want) {
			t.Fatalf("message %q lacks %q", envelope.Error.Message, want)
		}
	}
	if code, stdout, _ = call("status"); code != 0 || !strings.Contains(stdout, `"lifecycle":"active"`) {
		t.Fatalf("refused cancel changed the enrollment: exit %d %s", code, stdout)
	}
	code, stdout, stderr = call("cancel", "--reason", "base moved")
	if code != 0 || !strings.Contains(stdout, `"cancelReason":"base moved"`) || !strings.Contains(stdout, `"lifecycle":"cancelled"`) {
		t.Fatalf("cancel --reason: exit %d stdout %s stderr %s", code, stdout, stderr)
	}
}

// TestDogfoodPriorCompletionStaleNamesStagedRoute covers V1-0953: the
// prior-completion-stale refusal keeps its code and names the staged-binding
// route instead of carrying no next action.
func TestDogfoodPriorCompletionStaleNamesStagedRoute(t *testing.T) {
	t.Parallel()
	var stderr strings.Builder
	if code := emitLocalCompletionFailure(&stderr, "prior-completion-stale"); code != 2 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{`"code":"prior-completion-stale"`, "fresh worktree or clone", "staged binding"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("refusal %q lacks %q", stderr.String(), want)
		}
	}
}

// LCP-V0-040: controls start from the real successful sealed completion above.
func assertSealedCompletionControls(t *testing.T, root, key, base, seal string, positive localcompletion.Evaluation) {
	t.Helper()
	ctx := context.Background()
	if positive.EvidenceTarget == "" || positive.EvidenceTarget == seal || positive.Checks[0].TestedCommit != seal {
		t.Fatalf("missing target distinction: %#v", positive)
	}
	statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
	stateRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	outcome := filepath.Join(root, ".git/corvint/local-outcome.json")
	outcomeRaw, err := os.ReadFile(outcome)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"status", "finish"} {
		var result localcompletion.Evaluation
		if action == "status" {
			result, err = localcompletion.Evaluate(ctx, root, key)
		} else {
			result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		}
		if err != nil || !result.Satisfied {
			t.Fatalf("repeat %s: %#v %v", action, result, err)
		}
		after, _ := os.ReadFile(statePath)
		afterOutcome, _ := os.ReadFile(outcome)
		if !bytes.Equal(stateRaw, after) || !bytes.Equal(outcomeRaw, afterOutcome) {
			t.Fatalf("repeat %s mutated evidence", action)
		}
	}
	var savedDocument struct {
		Sealed struct {
			Prior struct {
				Reports      []struct{ Path string }
				Observations []struct{ Stdout struct{ Path string } }
			}
		} `json:"sealedContinuation"`
	}
	if err = json.Unmarshal(stateRaw, &savedDocument); err != nil {
		t.Fatal(err)
	}
	// A private artifact fault leaves the tracked tree clean, reaching evidence
	// validation rather than merely exercising the dirty-worktree guard.
	for _, name := range []string{positive.Reports[0], outcome, positive.Checks[0].Stdout, savedDocument.Sealed.Prior.Reports[0].Path, savedDocument.Sealed.Prior.Observations[0].Stdout.Path} {
		original, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(name, append(append([]byte{}, original...), []byte("injected drift")...), 0600); e != nil {
			t.Fatal(e)
		}
		if got := cemGit(t, root, "status", "--porcelain"); got != "" {
			t.Fatal("private fault made fixture dirty")
		}
		result, e := localcompletion.Evaluate(ctx, root, key)
		if e == nil && result.Satisfied {
			t.Fatalf("private drift accepted: %s", name)
		}
		if e = os.WriteFile(name, original, 0600); e != nil {
			t.Fatal(e)
		}
		result, e = localcompletion.Evaluate(ctx, root, key)
		if e != nil || !result.Satisfied {
			t.Fatalf("fault restoration: %#v %v", result, e)
		}
	}
	// Repointing the parent report to new, internally consistent outcome bytes
	// must not bypass the original coordination receipt. A terminal-artifact
	// mismatch alone is insufficient: the report set itself must become stale.
	coordinatorReportPath := filepath.Join(root, ".corvint/dogfood-report.json")
	coordinatorReportRaw, e := os.ReadFile(coordinatorReportPath)
	if e != nil {
		t.Fatal(e)
	}
	changedOutcome := append(append([]byte{}, outcomeRaw...), '\n')
	oldDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(outcomeRaw))
	newDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(changedOutcome))
	changedReport := bytes.ReplaceAll(coordinatorReportRaw, []byte(oldDigest), []byte(newDigest))
	if bytes.Equal(changedReport, coordinatorReportRaw) {
		t.Fatal("coordinator digest injection did not reach report")
	}
	if e = os.WriteFile(outcome, changedOutcome, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(coordinatorReportPath, changedReport, 0600); e != nil {
		t.Fatal(e)
	}
	var consistent struct {
		Outcome string `json:"localOutcomeEvidenceSha256"`
	}
	if e = json.Unmarshal(changedReport, &consistent); e != nil || consistent.Outcome != newDigest {
		t.Fatalf("injection not internally consistent: %v %#v", e, consistent)
	}
	drifted, e := localcompletion.Evaluate(ctx, root, key)
	if e != nil || drifted.Satisfied || drifted.ReportSetDigest != "" || !slices.Contains(drifted.Unmet, "report-set-stale") {
		t.Fatalf("changed coordinated outcome remained current: %#v %v", drifted, e)
	}
	if e = os.WriteFile(outcome, outcomeRaw, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(coordinatorReportPath, coordinatorReportRaw, 0600); e != nil {
		t.Fatal(e)
	}
	restored, e := localcompletion.Evaluate(ctx, root, key)
	if e != nil || !restored.Satisfied {
		t.Fatalf("coordinator restoration: %#v %v", restored, e)
	}
	shared := filepath.Join(root, ".corvint/change.cem.json")
	if e := os.WriteFile(shared, []byte("unexpected ignored map"), 0600); e != nil {
		t.Fatal(e)
	}
	result, e := localcompletion.Evaluate(ctx, root, key)
	if e == nil && result.Satisfied {
		t.Fatal("unexpected shared map accepted")
	}
	if e = os.Remove(shared); e != nil {
		t.Fatal(e)
	}
	// Force the final-check path without claiming that the prior terminal survives.
	var state map[string]any
	if e = json.Unmarshal(stateRaw, &state); e != nil {
		t.Fatal(e)
	}
	observations := state["observations"].([]any)
	latest := observations[len(observations)-1].(map[string]any)
	latest["passed"] = false
	latest["exit"] = "1"
	failedRaw, _ := json.Marshal(state)
	if e = os.WriteFile(statePath, failedRaw, 0600); e != nil {
		t.Fatal(e)
	}
	failed, failureErr := localcompletion.Evaluate(ctx, root, key)
	if failureErr != nil || failed.Satisfied || len(failed.Checks) != 1 || failed.Checks[0].Qualified {
		t.Fatalf("later failed observation accepted: %#v %v", failed, failureErr)
	}
	if e = json.Unmarshal(stateRaw, &state); e != nil {
		t.Fatal(e)
	}
	state["terminal"] = nil
	state["lifecycle"] = "active"
	altered, _ := json.Marshal(state)
	if e = os.WriteFile(statePath, altered, 0600); e != nil {
		t.Fatal(e)
	}
	reportPath := filepath.Join(root, ".corvint/dogfood-report.json")
	reportRaw, e := os.ReadFile(reportPath)
	if e != nil {
		t.Fatal(e)
	}
	// LCP-V0-043 (V1-3192): a supervised signal during the final strict check is
	// finish-interrupted naming the step, its retained logs and the rerun, and
	// writes no state; a genuine checker failure stays final-check-failed and
	// names the logs. Both are followed by a real finish reusing the bound results.
	interruptedState, e := os.ReadFile(statePath)
	if e != nil {
		t.Fatal(e)
	}
	// LCP-V0-060: the same interruption delivered as a real SIGINT and SIGTERM
	// to the `corvint dogfood finish` CLI.
	assertFinishSignalCLI(t, root, key, statePath)
	signalled, signal := context.WithCancel(ctx)
	interrupts := 0
	interruptChecker := func(ctx context.Context, root string, args []string, out, errout io.Writer) int {
		if len(args) > 1 && args[0] == "cem" && args[1] == "status" {
			interrupts++
			if interrupts > 1 {
				signal()
				return 2
			}
		}
		return localCompletionPublicCommand(ctx, root, args, out, errout)
	}
	result, e = localcompletion.Finish(signalled, root, key, interruptChecker)
	var final *localcompletion.FinalCheckError
	if interrupts < 2 || !errors.As(e, &final) || e.Error() != "finish-interrupted" || !final.Interrupted || final.Step != "final-check" || result.Satisfied {
		t.Fatalf("interrupted final check misreported: calls=%d result=%#v err=%v", interrupts, result, e)
	}
	if !strings.Contains(final.Detail(), "rerun=corvint dogfood finish --session-key "+key) || len(final.Logs) != 2 {
		t.Fatalf("interrupted detail: %s", final.Detail())
	}
	for _, name := range final.Logs {
		if _, err := os.Stat(filepath.Join(root, ".git", strings.TrimPrefix(name, "$GIT_DIR/"))); err != nil {
			t.Fatalf("interrupted log %s not retained: %v", name, err)
		}
	}
	if after, _ := os.ReadFile(statePath); !bytes.Equal(after, interruptedState) {
		t.Fatal("interrupted final check wrote state")
	}
	statuses := 0
	failChecker := func(ctx context.Context, root string, args []string, out, errout io.Writer) int {
		if len(args) > 1 && args[0] == "cem" && args[1] == "status" {
			statuses++
			if statuses > 1 {
				return 2
			}
		}
		return localCompletionPublicCommand(ctx, root, args, out, errout)
	}
	result, e = localcompletion.Finish(ctx, root, key, failChecker)
	if statuses < 2 || !errors.As(e, &final) || e.Error() != "final-check-failed" || final.Interrupted || result.Satisfied {
		t.Fatalf("final checker fault not reached/refused: calls=%d result=%#v err=%v", statuses, result, e)
	}
	if detail := final.Detail(); !strings.Contains(detail, "step=final-check logs=$GIT_DIR/corvint/local-completion/"+key+"/") || !strings.HasSuffix(detail, "/final-check.stderr") {
		t.Fatalf("failed final check hides its logs: %s", detail)
	}
	if after, _ := os.ReadFile(reportPath); !bytes.Equal(after, reportRaw) {
		t.Fatal("sealed checker changed parent report")
	}
	// Keep the failed terminal absent: success must be regained by a real check.
	result, e = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
	if e != nil || !result.Satisfied {
		t.Fatalf("real checker recovery: %#v %v", result, e)
	}
	// LCP-V0-060: the recovery finish reused the bound verification results; it
	// appended or rewrote no check observation, so no check was rerun.
	var held, recovered struct {
		Observations []map[string]any `json:"observations"`
	}
	recoveredState, e := os.ReadFile(statePath)
	if e != nil || json.Unmarshal(interruptedState, &held) != nil || json.Unmarshal(recoveredState, &recovered) != nil {
		t.Fatalf("state read: %v", e)
	}
	if len(held.Observations) == 0 || !reflect.DeepEqual(held.Observations, recovered.Observations) {
		t.Fatalf("recovery finish reverified: before=%v after=%v", held.Observations, recovered.Observations)
	}
	if got := cemGit(t, root, "rev-parse", "HEAD"); got != seal {
		t.Fatal("continuation moved HEAD")
	}
	if _, e = os.Stat(shared); !os.IsNotExist(e) {
		t.Fatal("continuation restored shared map")
	}
}

// TestLocalCompletionSealedCloseoutRepin pins LCP-V0-041 and LCP-V0-042
// (V1-3194): an enrolled strict finish, then the real one-command lane closeout,
// then the sealed finish reaches satisfied with no manual file edits, although
// the closeout's own dogfood record rewrote the pinned local outcome. Positive
// first; then tampered, retargeted, unreferenced and unrecorded replacements
// and a foreign OCM aggregate change each refuse sealed-evidence-drift naming
// the artifact with its pinned and current digests, and re-pin nothing.
func TestLocalCompletionSealedCloseoutRepin(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := localCompletionRepo(t)
	key := localcompletion.HashSession(t.Name())
	ctx := context.Background()
	plan := localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture"}, TimeoutSeconds: 60}}}
	raw, _ := json.Marshal(plan)
	if _, err := localcompletion.Begin(ctx, root, key, raw); err != nil {
		t.Fatal(err)
	}
	publicLocal(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	publicLocal(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
	cemGit(t, root, "add", "-f", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	bound := cemGit(t, root, "rev-parse", "HEAD")
	publicLocal(t, root, "ocm", "prepare", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--intent", "intent.md", "--expected-base", base, "--target", bound)
	publicLocal(t, root, "ocm", "link", "--map", ".corvint/change.ocm.001.json", "--cem", ".corvint/change.cem.json", "--obligation", "FIXTURE-LCP-001", "--hunk", "1", "--test-path", "fixture/fixture_test.go", "--claim", "test:TestAnswer/case:fixture-lcp", "--expected-base", base, "--target", bound)
	finishReviewed := func(stage string) localcompletion.Evaluation {
		t.Helper()
		if _, err := localcompletion.Verify(ctx, root, key, "actual-test"); err != nil {
			t.Fatalf("%s verify: %v", stage, err)
		}
		result, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		if err == nil && !result.Satisfied && result.ReportSetDigest != "" {
			if _, err = localcompletion.Review(ctx, root, key, result.ReportSetDigest); err != nil {
				t.Fatalf("%s review: %v", stage, err)
			}
			result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		}
		if err != nil || !result.Satisfied {
			dumpLocalCompletionFailure(t, root, key)
			t.Fatalf("%s finish: %#v %v", stage, result, err)
		}
		return result
	}
	finishReviewed("enrolled")
	outcomePath := filepath.Join(root, ".git/corvint/local-outcome.json")
	enrolled, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatal(err)
	}
	env, args := finishLaneInputs(t, "1\tintent.md\t1:5\tspecification\n", "intent.md\n")
	if code, stdout, stderr := run.exec(t, root, append(env[:3:3], finishLaneIdentity...), append([]string{"dogfood", "finish-lane", "--base", base}, args...)...); code != 0 {
		t.Fatalf("closeout exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	seal := cemGit(t, root, "rev-parse", "HEAD")
	if parent := cemGit(t, root, "rev-parse", "HEAD^"); parent != bound {
		t.Fatalf("closeout committed beyond the seal: parent %s, bound %s", parent, bound)
	}
	closeout, err := os.ReadFile(outcomePath)
	if err != nil || bytes.Equal(closeout, enrolled) {
		t.Fatalf("closeout did not rewrite the pinned outcome: %v", err)
	}
	statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
	reportPath := filepath.Join(root, ".corvint/dogfood-report.json")
	statusPath := filepath.Join(root, ".corvint/change.ocm-status.json")
	storePath := filepath.Join(root, ".context-corvint/traces", bound+".jsonl")
	if _, err = localcompletion.Verify(ctx, root, key, "actual-test"); err != nil {
		t.Fatal(err)
	}
	snapshot := map[string][]byte{}
	for _, name := range []string{statePath, outcomePath, reportPath, statusPath, storePath} {
		if snapshot[name], err = os.ReadFile(name); err != nil {
			t.Fatal(err)
		}
	}
	restore := func() {
		t.Helper()
		for name, original := range snapshot {
			if err := os.WriteFile(name, original, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Positive control: the closeout's own record is re-pinned and a new
	// review of a report set naming both digests is required.
	pending, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
	if err != nil || pending.Satisfied || pending.ReportSetDigest == "" {
		dumpLocalCompletionFailure(t, root, key)
		t.Fatalf("sealed finish after closeout: %#v %v", pending, err)
	}
	report, err := os.ReadFile(pending.Reports[0])
	enrolledDigest, closeoutDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(enrolled)), fmt.Sprintf("sha256:%x", sha256.Sum256(closeout))
	if err != nil || !strings.Contains(string(report), "replaced the coordinated outcome `"+enrolledDigest+"` with `"+closeoutDigest+"`") {
		t.Fatalf("continuation report hides the re-pin: %v %s", err, report)
	}
	if _, err = localcompletion.Review(ctx, root, key, pending.ReportSetDigest); err != nil {
		t.Fatal(err)
	}
	result, err := localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
	if err != nil || !result.Satisfied || result.Target != seal || result.EvidenceTarget != bound {
		dumpLocalCompletionFailure(t, root, key)
		t.Fatalf("sealed completion after closeout: %#v %v", result, err)
	}
	if got := cemGit(t, root, "status", "--porcelain"); got != "" {
		t.Fatalf("finish changed sealed worktree: %s", got)
	}
	retarget := func(raw []byte) []byte {
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		document["target"] = base
		document["trace"].(map[string]any)["revision"] = base
		out, _ := json.Marshal(document)
		return out
	}
	failed := func(raw []byte) []byte {
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		document["trace"].(map[string]any)["outcome"] = "failed"
		out, _ := json.Marshal(document)
		return out
	}
	referenced := func(outcome []byte) {
		t.Helper()
		changed := bytes.ReplaceAll(snapshot[reportPath], []byte(closeoutDigest), []byte(fmt.Sprintf("sha256:%x", sha256.Sum256(outcome))))
		if bytes.Equal(changed, snapshot[reportPath]) {
			t.Fatal("report does not reference the closeout outcome")
		}
		if err := os.WriteFile(reportPath, changed, 0600); err != nil {
			t.Fatal(err)
		}
	}
	firstRow := snapshot[storePath][:bytes.IndexByte(snapshot[storePath], '\n')+1]
	for _, control := range []struct {
		name, artifact, reason, path string
		content                      []byte
		before                       func([]byte)
	}{
		{"tampered", "local-outcome.json", "replacement-tampered", outcomePath, failed(closeout), referenced},
		{"retargeted", "local-outcome.json", "replacement-retargeted", outcomePath, retarget(closeout), referenced},
		{"unreferenced", "local-outcome.json", "replacement-unreferenced", outcomePath, append(append([]byte{}, closeout...), '\n'), nil},
		{"unrecorded", "local-outcome.json", "replacement-unrecorded", storePath, firstRow, nil},
		{"foreign-ocm-status", "change.ocm-status.json", "foreign-change", statusPath, append(append([]byte{}, snapshot[statusPath]...), ' '), nil},
	} {
		restore()
		if control.before != nil {
			control.before(control.content)
		}
		if err = os.WriteFile(control.path, control.content, 0600); err != nil {
			t.Fatal(err)
		}
		_, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
		var drift *localcompletion.SealedDriftError
		if !errors.As(err, &drift) || err.Error() != "sealed-evidence-drift" || drift.Artifact != control.artifact || drift.Reason != control.reason {
			t.Fatalf("%s: %v", control.name, err)
		}
		pinned, current := enrolledDigest, ""
		if control.artifact == "local-outcome.json" {
			now, _ := os.ReadFile(outcomePath)
			current = fmt.Sprintf("sha256:%x", sha256.Sum256(now))
		} else {
			pinned, current = fmt.Sprintf("sha256:%x", sha256.Sum256(snapshot[statusPath])), fmt.Sprintf("sha256:%x", sha256.Sum256(control.content))
		}
		if drift.Pinned != pinned || drift.Current != current {
			t.Fatalf("%s digests: %#v", control.name, drift)
		}
		if after, _ := os.ReadFile(statePath); !bytes.Equal(after, snapshot[statePath]) {
			t.Fatalf("%s re-pinned or saved state", control.name)
		}
		if control.name == "tampered" {
			code, _, stderr := runCLI(t, "--root", root, "dogfood", "finish", "--session-key", key)
			if code == 0 || !strings.Contains(stderr, "sealed-evidence-drift") || !strings.Contains(stderr, "artifact=local-outcome.json reason=replacement-tampered pinned="+pinned+" current="+current) {
				t.Fatalf("CLI hides the drifted artifact: exit=%d %s", code, stderr)
			}
		}
	}
	restore()
	if result = finishReviewed("restored"); result.Target != seal {
		t.Fatalf("restored completion: %#v", result)
	}
}
