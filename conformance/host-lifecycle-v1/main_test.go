package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvelopedReceipt(t *testing.T) {
	good := `{"hookSpecificOutput":{"additionalContext":"guidance\n` + envelopeBegin + `\nuntrusted\n{\"ok\":true,\"mutates\":false,\"profile\":\"corvint-dogfood-event/0\",\"context\":{\"task_evidence\":[{\"path\":\"add.go\",\"blob_hash\":\"b1\"}]}}\n` + envelopeEnd + `\n"}}`
	receipt, _, err := envelopedReceipt(good)
	if err != nil {
		t.Fatal(err)
	}
	if !citesPath(receipt, "add.go", "b1") || citesPath(receipt, "add.go", "b2") {
		t.Fatalf("citesPath disagrees with the receipt evidence")
	}
	// AHI-045: a current adapter injects the corvint-hook-context/0 projection instead.
	projected := `{"hookSpecificOutput":{"additionalContext":"` + envelopeBegin + `\nuntrusted\n{\"profile\":\"corvint-hook-context/0\",\"event\":\"session-start\",\"governance\":[{\"path\":\"AGENTS.md\",\"blob_hash\":\"g1\"}],\"task_evidence\":[{\"path\":\"add.go\",\"blob_hash\":\"b1\"}]}\n` + envelopeEnd + `\n"}}`
	projection, _, err := envelopedReceipt(projected)
	if err != nil {
		t.Fatal(err)
	}
	if !citesPath(projection, "add.go", "b1") || !citesSection(projection, "governance", "AGENTS.md", "g1") || citesSection(projection, "governance", "AGENTS.md", "b1") {
		t.Fatalf("citesSection disagrees with the projection evidence")
	}
	for name, output := range map[string]string{
		"not json":    "{",
		"no envelope": `{"hookSpecificOutput":{"additionalContext":"{\"ok\":true}"}}`,
		"failed":      `{"hookSpecificOutput":{"additionalContext":"` + envelopeBegin + `\n{\"ok\":false,\"mutates\":false,\"profile\":\"corvint-dogfood-event/0\"}\n` + envelopeEnd + `"}}`,
		"mutating":    `{"hookSpecificOutput":{"additionalContext":"` + envelopeBegin + `\n{\"ok\":true,\"mutates\":true,\"profile\":\"corvint-dogfood-event/0\"}\n` + envelopeEnd + `"}}`,
	} {
		if _, _, err := envelopedReceipt(output); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMissingCoreVerbs(t *testing.T) {
	help := "Commands:\n  frontier  ... Experimental.\nCommand maturity:\n  Core (decision 0332):\n    init, adopt, index, query, context, impact, affected, prove, cem, ocm,\n    frontier, dogfood\n  Experimental, no stability promise:\n    mcp\n"
	if missing := missingCoreVerbs(help); len(missing) != 0 {
		t.Fatalf("missing %v", missing)
	}
	partial := strings.Replace(help, "ocm,", "", 1)
	if missing := missingCoreVerbs(partial); strings.Join(missing, ",") != "ocm" {
		t.Fatalf("missing %v, want ocm", missing)
	}
	if missing := missingCoreVerbs("Command maturity:\n  Experimental, none\n"); len(missing) != len(coreVerbs) {
		t.Fatalf("absent Core section reported %v", missing)
	}
}

func TestListed(t *testing.T) {
	if !listed("Installed plugins:\n\n  ❯ corvint@corvint\n    Version: 0.2.3\n", "corvint@corvint") {
		t.Error("Claude Code row not recognized")
	}
	if !listed("corvint@corvint-source  installed, enabled  0.2.2  /x\n", "corvint@corvint-source") {
		t.Error("Codex row not recognized")
	}
	if listed("No plugins installed\n", "corvint@corvint") || listed("corvint@corvint-source  not installed\n", "corvint@corvint-source") {
		t.Error("absent plugin reported as listed")
	}
}

func TestRowIsScopedToSelector(t *testing.T) {
	claude := "Installed plugins:\n\n  ❯ other@m\n    Version: 0.2.3\n    Status: ✔ enabled\n  ❯ corvint@corvint\n    Version: 0.2.2\n    Status: ✘ disabled\n"
	text := row(claude, "corvint@corvint")
	if !strings.Contains(text, "disabled") || strings.Contains(text, "0.2.3") || strings.Contains(text, "✔ enabled") {
		t.Errorf("Claude Code row %q", text)
	}
	codex := "corvint@corvint-source  installed, disabled  0.2.2  /x\nother@m  installed, enabled  0.2.3  /y\n"
	text = row(codex, "corvint@corvint-source")
	if strings.Contains(text, "0.2.3") || strings.Contains(text, " enabled") {
		t.Errorf("Codex row %q", text)
	}
	if row(codex, "absent@m") != "" {
		t.Error("absent selector has a row")
	}
}

func TestReadHooks(t *testing.T) {
	directory := t.TempDir()
	argvShape := filepath.Join(directory, "argv.json")
	shellShape := filepath.Join(directory, "shell.json")
	if err := os.WriteFile(argvShape, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint","args":["adapter","claude-code","stop"]}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shellShape, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint adapter codex","timeout":2}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	argv, err := readHooks(argvShape, false)
	if err != nil || strings.Join(argv["Stop"], " ") != "corvint adapter claude-code stop" {
		t.Fatalf("argv shape: %v %v", argv, err)
	}
	shell, err := readHooks(shellShape, true)
	if err != nil || strings.Join(shell["Stop"], " ") != "corvint adapter codex" {
		t.Fatalf("shell shape: %v %v", shell, err)
	}
	several := filepath.Join(directory, "several.json")
	if err := os.WriteFile(several, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"corvint adapter codex"}]},{"hooks":[{"type":"command","command":"other"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readHooks(several, true); err == nil {
		t.Fatal("an event with two commands was accepted")
	}
	later := filepath.Join(directory, "later.json")
	if err := os.WriteFile(later, []byte(`{"hooks":{"Stop":[{"matcher":"x","hooks":[]},{"hooks":[{"type":"command","command":"corvint adapter codex"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hooks, err := readHooks(later, true); err != nil || strings.Join(hooks["Stop"], " ") != "corvint adapter codex" {
		t.Fatalf("command in a later group: %v %v", hooks, err)
	}
}

func TestSessionKeyPattern(t *testing.T) {
	key := strings.Repeat("ab", 32)
	for _, text := range []string{
		`["corvint","--root","/f","dogfood","status","--session-key","` + key + `"]`,
		`[\"corvint\",\"--session-key\",\"` + key + `\"]`,
	} {
		match := sessionKeyPattern.FindStringSubmatch(text)
		if match == nil || match[1] != key {
			t.Errorf("no key in %s", text)
		}
	}
}

func TestSummaryExitRequiresAllNinePass(t *testing.T) {
	var results []result
	for _, name := range caseOrder {
		results = append(results, result{name, "PASS", ""})
	}
	if summaryExit(results) != 0 {
		t.Fatal("nine PASS cases did not exit 0")
	}
	results[7].status = "NOT_RUN"
	if summaryExit(results) != 1 {
		t.Fatal("a NOT_RUN case exited 0")
	}
	if summaryExit(results[:8]) != 1 {
		t.Fatal("a missing case exited 0")
	}
}

func TestMissingEnvelopeRetainsDiagnostic(t *testing.T) {
	t.Run("HLQ-V1-007 missing envelope quotes bounded received context", func(t *testing.T) {
		for _, text := range []string{"Corvint FALLBACK degraded: adapter-host-kill-deadline; coding continues.", "first\nsecond\t\"quoted\"\x1b", strings.Repeat("x", 2048) + "tail", ""} {
			output, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{"additionalContext": text}})
			receipt, received, err := envelopedReceipt(string(output))
			if receipt != nil || received != text || err == nil {
				t.Fatalf("receipt=%v received=%q err=%v", receipt, received, err)
			}
			prefix := text[:min(len(text), 2048)]
			if !strings.Contains(err.Error(), fmt.Sprintf("received additionalContext=%q", prefix)) || strings.ContainsAny(err.Error(), "\n\t\x1b") {
				t.Fatalf("lost or unquoted diagnostic: %v", err)
			}
			if len(text) > 2048 && (!strings.Contains(err.Error(), "4 bytes omitted") || strings.Contains(err.Error(), "tail")) {
				t.Fatalf("unbounded diagnostic: %v", err)
			}
		}
	})
}

func TestContextHookRetainsRejectedText(t *testing.T) {
	t.Run("HLQ-V1-002 fallback reaches the case report", func(t *testing.T) {
		text := "Corvint FALLBACK degraded: adapter-host-kill-deadline; coding continues."
		output, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{"additionalContext": text}})
		directory := t.TempDir()
		filename := filepath.Join(directory, "hook.json")
		if err := os.WriteFile(filename, output, 0o600); err != nil {
			t.Fatal(err)
		}
		cat, err := exec.LookPath("cat")
		if err != nil {
			t.Fatal(err)
		}
		r := &runner{fixture: directory, environment: os.Environ(), hooks: map[string][]string{"SessionStart": {cat, filename}}}
		receipt, received, err := r.contextHook("SessionStart", nil)
		if receipt != nil || received != text || err == nil || !strings.Contains(err.Error(), "SessionStart: additionalContext carries no repository-data envelope") || !strings.Contains(err.Error(), "adapter-host-kill-deadline") {
			t.Fatalf("receipt=%v received=%q err=%v", receipt, received, err)
		}
		r.results = []result{{"upgrade", "FAIL", err.Error()}}
		if !strings.Contains(r.render(), "adapter-host-kill-deadline") {
			t.Fatal("report discarded diagnostic")
		}
	})
}

// fakeFrontierRunner builds a runner over a committed fixture whose registered Stop hook is a
// shell script and whose private PATH holds a fake corvint for dogfood begin and cancel. The
// enrolled incomplete Stop fails open with the given degradation output degraded times before it
// blocks; the unenrolled and recursive Stops release.
func fakeFrontierRunner(t *testing.T, degraded int, degradation string) (*runner, string) {
	t.Helper()
	work := t.TempDir()
	r := &runner{host: "codex", work: work, bin: filepath.Join(work, "bin"), fixture: filepath.Join(work, "fixture")}
	if err := os.MkdirAll(r.bin, 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	r.environment = []string{"PATH=" + r.bin + ":" + filepath.Dir(gitPath) + ":/usr/bin:/bin", "HOME=" + work, "LANG=C", "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.invalid"}
	if err := r.makeFixture(); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(work, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "degraded"), []byte(fmt.Sprint(degraded)), 0o644); err != nil {
		t.Fatal(err)
	}
	// The state path is single-quoted so a temporary directory with spaces or quotes stays one word.
	quoted := "'" + strings.ReplaceAll(state, "'", `'\''`) + "'"
	corvint := "#!/bin/sh\nstate=" + quoted + "\ncase \"$2\" in begin) : > \"$state/enrolled\" ;; cancel) /bin/rm -f \"$state/enrolled\" ;; esac\n"
	stop := "#!/bin/sh\ninput=$(cat)\nstate=" + quoted + "\n" +
		"case \"$input\" in *'\"stop_hook_active\":true'*) echo '{}'; exit 0 ;; esac\n" +
		"[ -f \"$state/enrolled\" ] || { echo '{}'; exit 0; }\n" +
		"left=$(cat \"$state/degraded\")\n" +
		"if [ \"$left\" -gt 0 ]; then echo $((left - 1)) > \"$state/degraded\"; echo 'trace on stderr' >&2; cat \"$state/degradation\"; exit 0; fi\n" +
		"echo '{\"decision\":\"block\",\"reason\":\"Corvint local completion policy is incomplete. Frontier authority remains unavailable.\"}'\n"
	output, _ := json.Marshal(map[string]any{"systemMessage": degradation})
	for name, body := range map[string]string{filepath.Join(r.bin, "corvint"): corvint, filepath.Join(work, "stop.sh"): stop, filepath.Join(state, "degradation"): string(output)} {
		if err := os.WriteFile(name, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r.hooks = map[string][]string{"Stop": {filepath.Join(work, "stop.sh")}}
	return r, state
}

func TestHookTimeBoundDegradation(t *testing.T) {
	deadline := "Corvint FALLBACK degraded: corvint-event-rejected:dogfood-event-deadline; coding continues"
	t.Run("HLQ-V1-009 an enrolled Stop that fails open on its deadline is retried and still must block", func(t *testing.T) {
		r, _ := fakeFrontierRunner(t, 1, deadline)
		r.step("frontier", r.pluginFrontier)
		got := r.results[0]
		if got.status != "PASS" || !strings.Contains(got.detail, "time-bound hook degradations retried: Stop attempt 1 corvint-event-rejected:dogfood-event-deadline after ") {
			t.Fatalf("result %+v", got)
		}
	})
	t.Run("HLQ-V1-009 exhausted attempts name the degradation and the hook streams instead of decision <nil>", func(t *testing.T) {
		r, _ := fakeFrontierRunner(t, hookAttempts, deadline)
		r.step("frontier", r.pluginFrontier)
		got := r.results[0]
		for _, want := range []string{
			"Stop hook failed open with time-bound degradation corvint-event-rejected:dogfood-event-deadline on all 3 attempts",
			"last hook Stop: exit=0 elapsed=", "degradation=corvint-event-rejected:dogfood-event-deadline",
			`stdout="{\"systemMessage\":`, `stderr="trace on stderr\n"`, "Stop attempt 3 ",
		} {
			if got.status != "FAIL" || !strings.Contains(got.detail, want) {
				t.Fatalf("result %+v lacks %q", got, want)
			}
		}
		if strings.Contains(got.detail, "decision <nil>") || strings.Count(r.render(), "\n") != 11 {
			t.Fatalf("silent nil or a forged report row: %q", r.render())
		}
	})
	t.Run("HLQ-V1-009 a non-time-bound fail-open is not retried and the failure names it", func(t *testing.T) {
		r, state := fakeFrontierRunner(t, 2, "Corvint FALLBACK degraded: corvint-event-rejected:dogfood-event-unavailable; coding continues")
		r.step("frontier", r.pluginFrontier)
		got := r.results[0]
		if got.status != "FAIL" || !strings.Contains(got.detail, "enrolled incomplete Stop returned decision <nil>") ||
			!strings.Contains(got.detail, "degradation=corvint-event-rejected:dogfood-event-unavailable") || strings.Contains(got.detail, "retried") {
			t.Fatalf("result %+v", got)
		}
		if left, _ := os.ReadFile(filepath.Join(state, "degraded")); strings.TrimSpace(string(left)) != "1" {
			t.Fatalf("a non-time-bound degradation was retried: %q runs left", left)
		}
	})
	t.Run("HLQ-V1-009 hook streams in a failed case are bounded", func(t *testing.T) {
		call := &hookCall{event: "Stop", stdout: strings.Repeat("o", hookStreamLimit+10), stderr: "e\tf", elapsed: 1712345 * time.Microsecond}
		text := call.String()
		if !strings.Contains(text, "exit=0 elapsed=1.712s degradation=none") || !strings.Contains(text, "(10 bytes omitted)") || strings.ContainsAny(text, "\t\n") {
			t.Fatalf("diagnostic %q", text)
		}
	})
}

func TestDegradationCode(t *testing.T) {
	for output, want := range map[string]string{
		`{"systemMessage":"Corvint FALLBACK degraded: adapter-host-kill-deadline; coding continues"}`:                                                                    "adapter-host-kill-deadline",
		`{"hookSpecificOutput":{"additionalContext":"Corvint FALLBACK degraded: corvint-event-rejected:dogfood-event-index-snapshot-stale; coding continues\nrefresh"}}`: "corvint-event-rejected:dogfood-event-index-snapshot-stale",
		`{"hookSpecificOutput":{"additionalContext":"Corvint fallback: corvint-event-rejected:dogfood-event-deadline; unrelated coding continues."}}`:                    "corvint-event-rejected:dogfood-event-deadline",
		`{"decision":"block","reason":"x"}`: "",
		`not json`:                          "",
	} {
		if got := degradationCode(output); got != want {
			t.Errorf("degradationCode(%s) = %q, want %q", output, got, want)
		}
		if want != "" && !timeBound(want) {
			t.Errorf("%s is not time-bound", want)
		}
	}
	if timeBound("malformed-hook-json") || timeBound("corvint-event-rejected:dogfood-event-unavailable") {
		t.Error("a semantic degradation counted as time-bound")
	}
}

// HLQ-V1-010 (V1-1121): a run is retained whenever any case is not PASS or names a time-bound
// retry, whatever case the caller studies; each retained copy is a new bounded file.
func TestFailedReportRetention(t *testing.T) {
	passing := func() []result {
		results := make([]result, 0, len(caseOrder))
		for _, name := range caseOrder {
			results = append(results, result{name, "PASS", "ok"})
		}
		return results
	}
	if retainReport(passing()) {
		t.Fatal("an all-PASS run without a retry is retained")
	}
	uninstall := passing()
	uninstall[len(uninstall)-1] = result{"uninstall", "FAIL", "private HOME retains corvint state: x"}
	retried := passing()
	retried[5].detail = "blocks" + retriedMarker + "Stop attempt 1 dogfood-event-deadline after 1.6s"
	missing := passing()[:8]
	for name, results := range map[string][]result{"non-target uninstall FAIL": uninstall, "PASS after a time-bound retry": retried, "missing case": missing} {
		if !retainReport(results) {
			t.Fatalf("%s is not retained", name)
		}
	}

	directory := t.TempDir()
	now := time.Date(2026, 10, 10, 18, 0, 0, 0, time.UTC)
	text := "case\tupgrade\tPASS\tok\ncase\tuninstall\tFAIL\tprivate HOME retains corvint state: x\n"
	first, err := keepReport(directory, "claude-code", text, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := keepReport(directory, "claude-code", text, now)
	if err != nil || second == first {
		t.Fatalf("a second report at the same instant is %q, %v; want a new file beside %q", second, err, first)
	}
	data, err := os.ReadFile(first)
	if err != nil || string(data) != text {
		t.Fatalf("retained report is %q, %v", data, err)
	}
	if info, err := os.Stat(first); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("retained report mode %v, %v; want 0600", info.Mode().Perm(), err)
	}
	long, err := keepReport(directory, "claude-code", strings.Repeat("x", retainedReportLimit+10), now)
	if err != nil {
		t.Fatal(err)
	}
	marker := truncationMarker(10 + len(truncationMarker(retainedReportLimit+10)))
	if data, _ := os.ReadFile(long); len(data) > retainedReportLimit || !strings.HasSuffix(string(data), marker) {
		t.Fatalf("an over-limit report is not bounded to %d bytes with the explicit omitted count %q: %d bytes, tail %q", retainedReportLimit, marker, len(data), data[len(data)-40:])
	}

	// A failed --report write still retains the copy, and both outcomes are reported.
	var stderr strings.Builder
	retainedDirectory := t.TempDir()
	if exit := writeReports(&stderr, "claude-code", text, filepath.Join(t.TempDir(), "missing", "report.tsv"), retainedDirectory, uninstall, now); exit != 2 {
		t.Fatalf("a failed --report write exits %d; want 2", exit)
	}
	if kept, _ := filepath.Glob(filepath.Join(retainedDirectory, "hlq-claude-code-*.tsv")); len(kept) != 1 || !strings.Contains(stderr.String(), "report retained at") {
		t.Fatalf("a failed --report write lost the retained copy: %v; stderr %q", kept, stderr.String())
	}
	if exit := writeReports(&stderr, "claude-code", text, "", retainedDirectory, passing(), now); exit != 0 {
		t.Fatalf("an all-PASS run exits %d; want 0", exit)
	}

	// An unusable directory is a setup error before any case runs: the corvint executable named
	// here does not exist, so only the directory check can produce this error.
	unusable := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(unusable, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(readOnly, 0o700) })
	directories := []string{unusable}
	if os.Geteuid() != 0 {
		directories = append(directories, readOnly)
	}
	for _, directory := range directories {
		stderr.Reset()
		var stdout strings.Builder
		exit := run([]string{"--host", "cli", "--corvint", filepath.Join(t.TempDir(), "absent"), "--failed-reports", directory}, &stdout, &stderr)
		if exit != 2 || !strings.Contains(stderr.String(), directory) || stdout.Len() != 0 {
			t.Fatalf("--failed-reports %s exits %d with stderr %q, stdout %q; want a setup error naming it", directory, exit, stderr.String(), stdout.String())
		}
		if leftovers, _ := filepath.Glob(filepath.Join(readOnly, ".hlq-probe-*")); len(leftovers) != 0 {
			t.Fatalf("the writability probe was left behind: %v", leftovers)
		}
	}
	probed := t.TempDir()
	if err := usableReportDirectory(probed); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(probed); len(entries) != 0 {
		t.Fatalf("the writability probe was left behind: %v", entries)
	}
}
