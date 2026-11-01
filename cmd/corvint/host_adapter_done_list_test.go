package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// AHI-196: a receipt is kept only while HEAD still names the commit read before any verb ran, and
// it names that commit as head.
func TestAHI196DoneListRefusesWhenHeadMoves(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	ctx := context.Background()
	start, err := doneListHead(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	receipt := map[string]any{"profile": doneListProfile, "ok": true, "base": ""}
	if kept := doneListBindHead(ctx, root, start, receipt); kept["ok"] != true || kept["head"] != start {
		t.Fatalf("an unmoved HEAD refused the receipt or did not name it: %v", kept)
	}
	affectedGit(t, root, "commit", "--allow-empty", "-q", "-m", "moved")
	refused := doneListBindHead(ctx, root, start, map[string]any{"profile": doneListProfile, "ok": true, "base": ""})
	if refused["ok"] != false || refused["error"].(map[string]any)["code"] != "head-moved" || refused["affected"] != nil || refused["head"] != nil {
		t.Fatalf("a moved HEAD kept the receipt: %v", refused)
	}
}

// AHI-196: the command and the hook receipts name the validated commit they describe as head.
func TestAHI196DoneListReceiptNamesHead(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	receipt, code := runDoneListCLI(t, "--root", root)
	if code != 0 || receipt["ok"] != true || receipt["head"] != head {
		t.Fatalf("done-list receipt does not name HEAD %s (exit %d): %v", head, code, receipt)
	}
	if hooked := doneList(context.Background(), root, "", false); hooked["head"] != head {
		t.Fatalf("hook receipt does not name HEAD %s: %v", head, hooked)
	}
}

// AHI-196: a failed HEAD probe refuses with head-unavailable; two failed probes never agree.
func TestAHI196DoneListRefusesWithoutHead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	affectedGit(t, root, "init", "-q")
	ctx := context.Background()
	if head, err := doneListHead(ctx, root); err == nil || head != "" {
		t.Fatalf("an unborn HEAD probe succeeded: %q", head)
	}
	for name, refused := range map[string]map[string]any{
		"bind":    doneListBindHead(ctx, root, "", map[string]any{"profile": doneListProfile, "ok": true, "base": ""}),
		"receipt": doneList(ctx, root, "", false),
	} {
		if refused["ok"] != false || refused["error"].(map[string]any)["code"] != "head-unavailable" || refused["head"] != nil {
			t.Fatalf("%s: a missing HEAD kept a receipt: %v", name, refused)
		}
	}
	receipt, code := runDoneListCLI(t, "--root", root)
	if code != 1 || receipt["error"].(map[string]any)["code"] != "head-unavailable" {
		t.Fatalf("command on an unborn HEAD: exit %d %v", code, receipt)
	}
	output := runClaudeAdapter(adapterEnvContext(ctx, map[string]string{"CLAUDE_PROJECT_DIR": root, doneListOptIn: "1"}), "stop-done-list",
		map[string]any{"session_id": "done-list-session", "hook_event_name": "Stop", "stop_hook_active": false})
	if output["systemMessage"] != "Corvint done-list: not reported: HEAD names no commit (head-unavailable)" || len(output) != 1 {
		t.Fatalf("hook on an unborn HEAD: %v", output)
	}
}

// AHI-196: the status scan is the isolated gitstatus read. A configured clean filter on a
// stat-dirty tracked file runs under a raw git status, and must not run under the hook or command.
func TestAHI196DoneListRunsNoRepositoryFilter(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.txt filter=probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "data.txt")
	if err := os.WriteFile(data, []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-q", "-m", "filtered")
	marker := filepath.Join(t.TempDir(), "filter-ran")
	affectedGit(t, root, "config", "filter.probe.clean", "touch '"+marker+"'; cat")
	stale := func() {
		later := time.Now().Add(time.Hour)
		if err := os.Chtimes(data, later, later); err != nil {
			t.Fatal(err)
		}
	}
	stale()
	ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root, doneListOptIn: "1"})
	output := runClaudeAdapter(ctx, "stop-done-list", map[string]any{"session_id": "done-list-session", "hook_event_name": "Stop", "stop_hook_active": false})
	if _, ok := output["systemMessage"].(string); !ok {
		t.Fatalf("hook under a refused status must still report: %v", output)
	}
	receipt, _ := runDoneListCLI(t, "--root", root)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("done-list ran the repository clean filter: %v", err)
	}
	if receipt["ok"] == true && receipt["mutation"].(map[string]any)["reason"] != "new-tests-unavailable" {
		t.Fatalf("a filtered status was not refused: %v", receipt)
	}
	stale()
	affectedGit(t, root, "--no-optional-locks", "status", "--porcelain")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the fixture is not adversarial: a raw git status did not run the filter: %v", err)
	}
}

// AHI-807 (V1-2514): the plugin declares a boolean stop_done_list userConfig option; the hook
// honours it as the host exports it, and a set CORVINT_STOP_DONE_LIST keeps precedence.
func TestAHI807StopDoneListPluginOptionAndPrecedence(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "integrations", "claude-code", "plugins", "corvint", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		UserConfig map[string]struct {
			Type    string `json:"type"`
			Default *bool  `json:"default"`
		} `json:"userConfig"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if option, ok := manifest.UserConfig["stop_done_list"]; !ok || option.Type != "boolean" || option.Default == nil || *option.Default {
		t.Fatalf("plugin.json stop_done_list option: %+v", manifest.UserConfig)
	}
	for _, test := range []struct {
		env, option string
		want        bool
	}{{"", "", false}, {"", "true", true}, {"", "1", true}, {"", "false", false}, {"1", "", true}, {"1", "false", true}, {"0", "true", false}, {"true", "true", false}} {
		values := map[string]string{}
		if test.env != "" {
			values[doneListOptIn] = test.env
		}
		if test.option != "" {
			values[doneListPluginOpt] = test.option
		}
		if got := doneListOptedIn(adapterEnvContext(context.Background(), values)); got != test.want {
			t.Fatalf("env=%q option=%q: opted in %v, want %v", test.env, test.option, got, test.want)
		}
	}
	// End to end: the option alone yields the done-list on a dirty tree.
	root := cliRepository(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root, doneListPluginOpt: "true"})
	if output := runClaudeAdapter(ctx, "stop-done-list", map[string]any{"session_id": "s", "hook_event_name": "Stop", "stop_hook_active": false}); output["systemMessage"] == nil {
		t.Fatalf("the plugin option did not enable the done-list: %v", output)
	}
}
