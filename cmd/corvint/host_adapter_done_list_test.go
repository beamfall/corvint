package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/Beamfall/corvint/internal/repoenvelope"
)

func decodeDoneList(t *testing.T, stdout []byte) map[string]any {
	t.Helper()
	text := strings.TrimSuffix(string(stdout), "\n")
	if !strings.HasPrefix(text, repoenvelope.Prefix) || !strings.HasSuffix(text, repoenvelope.Suffix) {
		t.Fatalf("done-list output is not enveloped: %s", stdout)
	}
	var receipt map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(text, repoenvelope.Prefix), repoenvelope.Suffix)), &receipt); err != nil {
		t.Fatalf("done-list payload: %v\n%s", err, stdout)
	}
	if receipt["profile"] != doneListProfile || receipt["mutates"] != false {
		t.Fatalf("done-list receipt: %v", receipt)
	}
	return receipt
}

func runDoneListCLI(t *testing.T, arguments ...string) (map[string]any, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), append([]string{"adapter", "done-list"}, arguments...), strings.NewReader(""), &stdout, &stderr)
	return decodeDoneList(t, stdout.Bytes()), code
}

// AHI-191, AHI-192, AHI-193: the Stop done-list is silent unless opted in, on a continued Stop, in a
// subagent and on a clean tree; opted in on a dirty tree it is one systemMessage and nothing that
// could continue the turn, and it leaves the tree untouched.
func TestAHI191StopDoneListOptInSilenceAndShape(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	env := func(optIn bool) context.Context {
		values := map[string]string{"CLAUDE_PROJECT_DIR": root}
		if optIn {
			values[doneListOptIn] = "1"
		}
		return adapterEnvContext(context.Background(), values)
	}
	payload := func(extra map[string]any) map[string]any {
		value := map[string]any{"session_id": "done-list-session", "hook_event_name": "Stop", "stop_hook_active": false}
		for key, item := range extra {
			value[key] = item
		}
		return value
	}
	if output := runClaudeAdapter(env(true), "stop-done-list", payload(nil)); len(output) != 0 {
		t.Fatalf("a clean tree produced a done-list: %v", output)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra_test.go"), []byte("package extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, root)
	for name, test := range map[string]struct {
		ctx     context.Context
		payload map[string]any
	}{
		"not opted in":     {env(false), payload(nil)},
		"continued stop":   {env(true), payload(map[string]any{"stop_hook_active": true})},
		"subagent":         {env(true), payload(map[string]any{"agent_id": "agent-1"})},
		"opt-in not exact": {adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root, doneListOptIn: "true"}), payload(nil)},
	} {
		if output := runClaudeAdapter(test.ctx, "stop-done-list", test.payload); len(output) != 0 {
			t.Fatalf("%s: emitted %v", name, output)
		}
	}
	output := runClaudeAdapter(env(true), "stop-done-list", payload(nil))
	message, ok := output["systemMessage"].(string)
	if !ok || len(output) != 1 {
		t.Fatalf("opted-in done-list must be one systemMessage: %v", output)
	}
	for _, want := range []string{"Corvint done-list:", "tests: ", "witness-requires-committed-range", "mutation-not-run-from-hook", "extra_test.go", "\"--mutate\""} {
		if !strings.Contains(message, want) {
			t.Fatalf("done-list lacks %q:\n%s", want, message)
		}
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if !strings.Contains(message, `"witness","--base","`+head+`"`) {
		t.Fatalf("done-list does not name the current HEAD as the next base:\n%s", message)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("the Stop done-list changed the working tree")
	}
}

// AHI-193: the rendered message is capped and carries no control or hidden character from
// repository data.
func TestAHI193DoneListMessageBoundedAndPlain(t *testing.T) {
	t.Parallel()
	units := []any{}
	for range doneListUnitLimit {
		units = append(units, map[string]any{"unitId": "go:" + strings.Repeat("x", 300) + "\u202e\x1b[2J", "testCount": 1})
	}
	receipt := map[string]any{
		"affected": map[string]any{"state": "COMPLETED", "selected": 40, "tiers": "tier0=0 tier1=0 tier2=40", "units": units, "unknown": 2},
		"witness":  map[string]any{"state": "NOT_RUN", "reason": "witness-requires-committed-range", "argv": []string{"corvint", "witness"}},
		"mutation": map[string]any{"state": "NOT_RUN", "reason": "mutation-not-run-from-hook", "newTests": []any{map[string]any{"path": "a\nb_test.go", "reason": "mutation-not-run-from-hook"}}},
	}
	text := renderDoneList("/repo", receipt)
	if len(text) > doneListMessageCap || !strings.HasSuffix(text, " [truncated]") {
		t.Fatalf("message is %d bytes, cap %d", len(text), doneListMessageCap)
	}
	for _, r := range text {
		if r != '\n' && (unicode.IsControl(r) || r == '\u202e') {
			t.Fatalf("message carries %U", r)
		}
	}
	small := renderDoneList("/repo", receipt)
	if strings.Count(small, "\n") > 3 {
		t.Fatalf("a repository path broke the message into extra lines:\n%s", small)
	}
}

// AHI-195: the explicit command refuses malformed arguments, and --mutate without a committed base.
func TestAHI195DoneListCommandRefusals(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	for _, arguments := range [][]string{
		{"--root", root, "--mutate"},
		{"--root", root, "--base", "HEAD"},
		{"--root", root, "--unknown"},
		{"--root"},
	} {
		receipt, code := runDoneListCLI(t, arguments...)
		if code != 2 || receipt["ok"] != false || receipt["error"].(map[string]any)["code"] != "invalid-arguments" {
			t.Fatalf("%q: exit %d receipt %v", arguments, code, receipt)
		}
	}
	receipt, code := runDoneListCLI(t, "--root", t.TempDir())
	if code != 2 || receipt["error"].(map[string]any)["code"] != "not-a-git-repository" {
		t.Fatalf("outside a repository: exit %d receipt %v", code, receipt)
	}
}

// AHI-196, AHI-197, V1-0403: on a committed range the done-list reports the selected tests, the
// unwitnessed obligations and one mutation verdict per new Go test; a tautological new test kills
// no mutant, an asserting one kills some, and the tree is unchanged.
func TestAHI197DoneListJudgesNewTestsByMutation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, verdict string
	}{
		{"tautological", "func TestNew(t *testing.T) {}", "KILLS_NO_MUTANT"},
		{"asserting", `func TestNew(t *testing.T) { if !IsPositive(5) || IsPositive(-1) || IsPositive(11) { t.Fatal("range") } }`, "KILLS_MUTANTS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := proveMutateRepository(t, proveMutateKillingTest)
			base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			source := "package calc\n\nimport \"testing\"\n\n" + test.body + "\n"
			if err := os.WriteFile(filepath.Join(root, "pkg", "calc", "new_test.go"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			commitCalc(t, root, strings.Replace(proveMutateCalc, "if n > 0 {", "if n > 0 && n < 10 {", 1), "bound condition with a new test")
			before := treeDigest(t, root)
			receipt, code := runDoneListCLI(t, "--root", root, "--base", base, "--mutate")
			if code != 0 || receipt["ok"] != true {
				t.Fatalf("exit %d: %v", code, receipt)
			}
			if after := treeDigest(t, root); after != before {
				t.Fatal("the done-list changed the working tree")
			}
			affected := receipt["affected"].(map[string]any)
			if affected["state"] != "COMPLETED" || affected["selected"].(float64) < 1 {
				t.Fatalf("affected: %v", affected)
			}
			witness := receipt["witness"].(map[string]any)
			if witness["state"] != "COMPLETED" {
				t.Fatalf("witness: %v", witness)
			}
			mutation := receipt["mutation"].(map[string]any)
			tests := mutation["newTests"].([]any)
			if mutation["state"] != "COMPLETED" || len(tests) != 1 {
				t.Fatalf("mutation: %v", mutation)
			}
			entry := tests[0].(map[string]any)
			if entry["path"] != "pkg/calc/new_test.go" || entry["verdict"] != test.verdict || entry["mutants"].(float64) == 0 {
				t.Fatalf("new test verdict: %v", entry)
			}
			if killed := entry["killed"].(float64); (killed == 0) != (test.verdict == "KILLS_NO_MUTANT") {
				t.Fatalf("killed %v contradicts %s", killed, test.verdict)
			}
			plain, code := runDoneListCLI(t, "--root", root, "--base", base)
			unrun := plain["mutation"].(map[string]any)
			if code != 0 || unrun["reason"] != "mutation-not-requested" || unrun["newTests"].([]any)[0].(map[string]any)["verdict"] != "NOT_RUN" {
				t.Fatalf("without --mutate: exit %d %v", code, unrun)
			}
		})
	}
}

// AHI-197: uncompilable or skipped mutants are not executions, so an attack that executed none is
// NOT_RUN, never KILLS_NO_MUTANT.
func TestAHI197DoneListVerdictNeedsExecutedMutants(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		attack          map[string]any
		verdict, reason string
	}{
		{map[string]any{"status": "COMPLETED", "mutants": 3.0, "killed": 0.0, "survived": 0.0, "uncompilable": 3.0}, "NOT_RUN", "no-mutant-executed"},
		{map[string]any{"status": "PARTIAL", "mutants": 3.0, "killed": 0.0, "survived": 0.0, "skipped": 3.0}, "NOT_RUN", "no-mutant-executed"},
		{map[string]any{"status": "PARTIAL", "mutants": 3.0, "killed": 0.0, "survived": 1.0, "skipped": 2.0}, "KILLS_NO_MUTANT", "PARTIAL"},
		{map[string]any{"status": "COMPLETED", "mutants": 3.0, "killed": 1.0, "survived": 2.0}, "KILLS_MUTANTS", "COMPLETED"},
	} {
		verdict, reason := doneListVerdict(map[string]any{"attack": test.attack}, map[string]any{})
		if verdict != test.verdict || reason != test.reason {
			t.Fatalf("%v: got %s %s, want %s %s", test.attack, verdict, reason, test.verdict, test.reason)
		}
	}
}

// AHI-196: a receipt is kept only while HEAD still names the commit read before any verb ran.
func TestAHI196DoneListRefusesWhenHeadMoves(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	ctx := context.Background()
	start := doneListHead(ctx, root)
	receipt := map[string]any{"profile": doneListProfile, "ok": true, "base": ""}
	if kept := doneListBindHead(ctx, root, start, receipt); kept["ok"] != true {
		t.Fatalf("an unmoved HEAD refused the receipt: %v", kept)
	}
	affectedGit(t, root, "commit", "--allow-empty", "-q", "-m", "moved")
	refused := doneListBindHead(ctx, root, start, receipt)
	if refused["ok"] != false || refused["error"].(map[string]any)["code"] != "head-moved" || refused["affected"] != nil {
		t.Fatalf("a moved HEAD kept the receipt: %v", refused)
	}
}
