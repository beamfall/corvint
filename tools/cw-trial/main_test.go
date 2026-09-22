package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

const goldReply = "I checked the tree.\n\n```json\n{\"claims\":[{\"kind\":\"test-file\",\"value\":\"tests/help_test.rs\",\"confidence\":\"certain\",\"evidence\":\"tests/help_test.rs\"}]}\n```\n"

func fixtureGold() map[string][]string {
	return map[string][]string{"test-file": {"tests/help_test.rs"}}
}

// fixtureSnapshot is a directory snapshot without .git whose gold file is
// findable by grep.
func fixtureSnapshot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"src/help.rs":        "fn quote_default_value() {}\n",
		"src/format.rs":      "fn format_error() {}\n",
		"tests/help_test.rs": "#[test] fn quote_empty_default_value_in_help() {}\n",
		"README.md":          "clap help output\n",
		"assets/logo.bin":    "PNG\x00\x00binary quote default help",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func fixtureTask(snapshot string) task {
	return task{ID: "t1", Kind: "retrieval", Repo: "clap-rs/clap", BaseCommit: "abc", Snapshot: snapshot,
		Text: "Which test file covers quoting empty default values in help output?", Gold: fixtureGold()}
}

func writeManifest(t *testing.T, snapshot string) string {
	t.Helper()
	data, err := json.Marshal(manifest{Partition: "pilot", Source: "fixture", Tasks: []task{fixtureTask(snapshot)}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeAgent writes a script that replies with body and, when marker is
// set, also tries to write a file into its working directory.
func fakeAgent(t *testing.T, body, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("script agent needs sh")
	}
	script := "#!/bin/sh\n"
	if marker != "" {
		script += "echo written > " + marker + "\n"
	}
	script += "cat <<'EOF'\n" + body + "EOF\n"
	path := filepath.Join(t.TempDir(), "agent.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		hash.Write([]byte(relative))
		hash.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(hash.Sum(nil))
}

func TestExtractClaimsPresentAbsentMalformed(t *testing.T) {
	claims, state := extractClaims(goldReply)
	if state != "PRESENT" || len(claims) != 1 || claims[0].Value != "tests/help_test.rs" {
		t.Fatalf("present: state %s claims %+v", state, claims)
	}
	if _, state := extractClaims("I do not know which test covers it."); state != "ABSENT" {
		t.Fatalf("absent: state %s", state)
	}
	if _, state := extractClaims("```json\n{\"claims\":[{\"kind\":\"test-file\",]}\n```"); state != "MALFORMED" {
		t.Fatalf("malformed: state %s", state)
	}
	if _, state := extractClaims("```json\n{\"claims\":\"tests/help_test.rs\"}\n```"); state != "MALFORMED" {
		t.Fatalf("claims not an array: state %s", state)
	}
	last, _ := extractClaims("```json\n{\"claims\":[]}\n```\nRevised:\n```json\n{\"claims\":[{\"kind\":\"answer\",\"value\":\"yes\",\"confidence\":\"unsure\",\"evidence\":\"none\"}]}\n```")
	if len(last) != 1 || last[0].Kind != "answer" {
		t.Fatalf("last block wins: %+v", last)
	}
	bare, state := extractClaims("{\"claims\":[]}")
	if state != "PRESENT" || len(bare) != 0 {
		t.Fatalf("bare object: state %s claims %+v", state, bare)
	}
}

func TestScoreArmSuccessConfidentlyWrongAbstain(t *testing.T) {
	cases := []struct {
		name   string
		reply  string
		expect map[string]float64
		state  string
	}{
		{"success", goldReply, map[string]float64{"success": 1, "confidently_wrong": 0, "abstained": 0, "gold_as_result": 1, "utilisation_gap": 0}, "PRESENT"},
		{"confidently wrong", "```json\n{\"claims\":[{\"kind\":\"test-file\",\"value\":\"tests/other_test.rs\",\"confidence\":\"certain\",\"evidence\":\"none\"},{\"kind\":\"test-file\",\"value\":\"src/help.rs\",\"confidence\":\"likely\",\"evidence\":\"none\"},{\"kind\":\"source-file\",\"value\":\"src/help.rs\",\"confidence\":\"certain\",\"evidence\":\"none\"},{\"kind\":\"file\",\"value\":\"x\",\"confidence\":\"certain\",\"evidence\":\"none\"}]}\n```",
			map[string]float64{"success": 0, "confidently_wrong": 1, "confidently_wrong_task": 1, "wrong_likely": 1, "unjudged": 1, "invalid": 1, "abstained": 0, "gold_as_result": 1, "utilisation_gap": 1}, "PRESENT"},
		{"abstain by silence", "No idea.", map[string]float64{"success": 0, "confidently_wrong": 0, "abstained": 1, "utilisation_gap": 1}, "ABSENT"},
		{"abstain by empty block", "```json\n{\"claims\":[]}\n```", map[string]float64{"abstained": 1}, "PRESENT"},
		{"malformed abstains", "```json\n{\"claims\":[}\n```", map[string]float64{"abstained": 1, "confidently_wrong": 0}, "MALFORMED"},
	}
	for _, item := range cases {
		record := &armRecord{Reply: item.reply, Context: "score=3 tests/help_test.rs"}
		scoreArm(record, &taskRecord{Gold: fixtureGold()})
		if record.BlockState != item.state {
			t.Errorf("%s: block state %s, want %s", item.name, record.BlockState, item.state)
		}
		for key, want := range item.expect {
			if record.Metrics[key] != want {
				t.Errorf("%s: %s = %v, want %v", item.name, key, record.Metrics[key], want)
			}
		}
	}
	errored := &armRecord{Error: "codex: timeout"}
	scoreArm(errored, &taskRecord{Gold: fixtureGold()})
	if errored.Metrics != nil {
		t.Fatalf("an errored invocation must score nothing: %v", errored.Metrics)
	}
}

// CWT-V0-004: a reply cut at the 1 MiB bound lost its tail, so a missing
// claims block is an error, never an abstention.
func TestScoreArmErrorsATruncatedReplyWithoutClaims(t *testing.T) {
	record := &armRecord{Reply: strings.Repeat("x", maxReplyBytes) + truncatedMarker, ReplyTruncated: true, ExitCode: 0}
	scoreArm(record, &taskRecord{Gold: fixtureGold()})
	if record.Error == "" || record.Metrics != nil {
		t.Fatalf("a truncated reply without claims scored as an abstention: %+v", record.Metrics)
	}
}

func TestJudgeClaimsGroundsEvidenceInContext(t *testing.T) {
	claims := []claim{
		{Kind: "test-file", Value: "./tests/help_test.rs", Confidence: "certain", Evidence: "tests/help_test.rs"},
		{Kind: "test-file", Value: "tests/help_test.rs", Confidence: "likely", Evidence: "none"},
		{Kind: "answer", Value: " Yes ", Confidence: "unsure", Evidence: "src/help.rs"},
	}
	judged := judgeClaims(claims, map[string][]string{"test-file": {"tests/help_test.rs"}, "answer": {"yes"}}, "score=3 tests/help_test.rs")
	if judged[0].Verdict != "TRUE" || judged[0].Grounded != true {
		t.Fatalf("normalised path: %+v", judged[0])
	}
	if judged[1].Grounded != false {
		t.Fatalf("none is not grounded: %+v", judged[1])
	}
	if judged[2].Verdict != "TRUE" || judged[2].Grounded != false {
		t.Fatalf("answer compares case-insensitively: %+v", judged[2])
	}
	if got := judgeClaims(claims[:1], nil, "")[0]; got.Verdict != "UNJUDGED" || got.Grounded != notApplicable {
		t.Fatalf("no gold, no context: %+v", got)
	}
}

func TestTrialWithScriptAgentScoresEveryArm(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	agentPath := fakeAgent(t, goldReply, "")
	configuration := options{tasks: writeManifest(t, snapshot), arms: []string{"none", "grep"}, agent: "script", agentCommand: agentPath, timeout: time.Minute, limit: 20}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	if !document.Pilot || document.Partition != "pilot" || document.Tasks != 1 || document.Corvint != notObserved {
		t.Fatalf("header: pilot %v partition %s tasks %d corvint %v", document.Pilot, document.Partition, document.Tasks, document.Corvint)
	}
	if document.SkeletonSHA256 != digestOf(skeleton) || document.Prologues["grep"]["sha256"] != digestOf(prologues["grep"]) {
		t.Fatal("skeleton and prologue digests must be recorded")
	}
	for _, name := range configuration.arms {
		arm := document.Details[0].Arms[name]
		if arm.Error != "" || arm.BlockState != "PRESENT" || arm.Metrics["success"] != 1 || arm.Metrics["confidently_wrong"] != 0 {
			t.Fatalf("%s: %+v", name, arm)
		}
		if arm.Tokens != notObserved || arm.ExitCode != 0 {
			t.Fatalf("%s: tokens %v exit %v", name, arm.Tokens, arm.ExitCode)
		}
		if _, ok := arm.WallMs.(int64); !ok {
			t.Fatalf("%s: wall time must be observed, got %v", name, arm.WallMs)
		}
		summary := document.Arms[name].(map[string]any)
		if summary["tokens"] != notObserved || summary["errors"] != 0 {
			t.Fatalf("%s summary: %v", name, summary)
		}
		if rate := summary["success"].(map[string]any)["rate"]; rate != 1.0 {
			t.Fatalf("%s success rate: %v", name, rate)
		}
	}
	none, grep := document.Details[0].Arms["none"], document.Details[0].Arms["grep"]
	if none.Context != "" || none.Claims[0].Grounded != notApplicable {
		t.Fatalf("none arm carries no context: %+v", none)
	}
	if !strings.HasPrefix(grep.Context, "score=") || !strings.Contains(grep.Context, "tests/help_test.rs") || strings.Contains(grep.Context, "logo.bin") {
		t.Fatalf("grep context: %q", grep.Context)
	}
	if grep.Claims[0].Grounded != true || grep.PromptSHA256 == none.PromptSHA256 {
		t.Fatalf("grep arm: %+v", grep.Claims[0])
	}
}

func TestTrialCountsANonZeroExitWithoutClaimsAsAnError(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	agentPath := fakeAgent(t, "", "")
	if err := os.WriteFile(agentPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	configuration := options{tasks: writeManifest(t, snapshot), arms: []string{"none"}, agent: "script", agentCommand: agentPath, timeout: time.Minute, limit: 20}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	check := func(stage string) {
		arm := document.Details[0].Arms["none"]
		summary := document.Arms["none"].(map[string]any)
		if arm.Error == "" || arm.Metrics != nil || summary["errors"] != 1 || summary["tasks"] != 1 {
			t.Fatalf("%s: arm %+v summary %v", stage, arm, summary)
		}
	}
	check("trial")
	arm := document.Details[0].Arms["none"]
	arm.Error, arm.ExitCode = "", float64(1)
	finalize(document)
	check("rescore")
}

func TestTrialFinalizeAndScoreAreByteIdentical(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	configuration := options{tasks: writeManifest(t, snapshot), arms: []string{"none", "grep"}, agent: "script", agentCommand: fakeAgent(t, goldReply, ""), timeout: time.Minute, limit: 20}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if code := write(document, "", &first, os.Stderr); code != 0 {
		t.Fatal("write failed")
	}
	finalize(document)
	if code := write(document, "", &second, os.Stderr); code != 0 {
		t.Fatal("write failed")
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("finalize is not idempotent")
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, first.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	var rescored bytes.Buffer
	if code := run(context.Background(), []string{"score", "--report", path}, &rescored, os.Stderr); code != 0 {
		t.Fatal("score failed")
	}
	if !bytes.Equal(first.Bytes(), rescored.Bytes()) {
		t.Fatalf("score output differs from run output:\n%s\n%s", first.String(), rescored.String())
	}
	keys := make([]string, 0)
	for key := range document.Arms {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "grep,none" {
		t.Fatalf("arms: %v", keys)
	}
}

func TestMaterializationNeverTouchesTheSnapshot(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	before := treeDigest(t, snapshot)
	configuration := options{tasks: writeManifest(t, snapshot), arms: []string{"none"}, agent: "script", agentCommand: fakeAgent(t, goldReply, "agent-wrote-here.txt"), timeout: time.Minute, limit: 20}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	if document.Details[0].Arms["none"].Error != "" {
		t.Fatal(document.Details[0].Arms["none"].Error)
	}
	if treeDigest(t, snapshot) != before {
		t.Fatal("the source snapshot changed")
	}
	if _, err := os.Stat(filepath.Join(snapshot, ".git")); err == nil {
		t.Fatal("the source snapshot was used in place")
	}
	if _, err := os.Stat(filepath.Join(snapshot, "agent-wrote-here.txt")); err == nil {
		t.Fatal("the agent ran inside the source snapshot")
	}
}

func TestChunkFileMaterializesOnlyFileRows(t *testing.T) {
	rows := []string{
		`{"kind":"file","path":"src/a.go","text":"package a\n"}`,
		`{"kind":"symbol","path":"src/a.go","text":"ignored"}`,
		`{"kind":"file","path":"../escape.txt","text":"no"}`,
		`{"kind":"file","path":"tests/a_test.go","text":"package a\n"}`,
	}
	chunk := filepath.Join(t.TempDir(), "abc.chunks.jsonl")
	if err := os.WriteFile(chunk, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := materialize(context.Background(), chunk, writeChunkRows)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	for _, path := range []string{"src/a.go", "tests/a_test.go", ".git"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.txt")); err == nil {
		t.Fatal("an escaping path was written")
	}
	if status, _ := git(context.Background(), root, "status", "--porcelain"); status != "" {
		t.Fatalf("copy is dirty: %q", status)
	}
}

// CWT-V0-008: reuse and score --tasks key records by task id, so two tasks
// sharing one id would cross their records.
func TestManifestRefusesDuplicateTaskIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	task := `{"id":"t1","kind":"retrieval","text":"find it","gold":{"test-file":["a_test.go"]}}`
	if err := os.WriteFile(path, []byte(`{"partition":"pilot","tasks":[`+task+`,`+task+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readManifest(options{tasks: path}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("readManifest error = %v, want a duplicate id refusal", err)
	}
}

func TestParseCodexEventsReadsUsageAndLastMessage(t *testing.T) {
	events := parseCodexEvents([]byte(`{"type":"thread.started","thread_id":"x"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"ok"}}
{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"/bin/zsh -lc 'ls -la'","exit_code":0,"status":"completed"}}
{"type":"item.started","item":{"id":"item_2","type":"command_execution","command":"/bin/zsh -lc 'cat x'","status":"in_progress"}}
{"type":"turn.completed","usage":{"input_tokens":21361,"cached_input_tokens":0,"output_tokens":5}}
`))
	if events.lastMessage != "ok" {
		t.Fatalf("last message %q", events.lastMessage)
	}
	if events.toolCalls != 1 || len(events.commands) != 1 || events.commands[0] != "/bin/zsh -lc 'ls -la'" {
		t.Fatalf("completed commands are tool calls: %d %v", events.toolCalls, events.commands)
	}
	usage, ok := events.tokens.(map[string]any)
	if !ok || usage["input_tokens"] != 21361.0 {
		t.Fatalf("tokens %v", events.tokens)
	}
	if parseCodexEvents([]byte("not json\n")).tokens != notObserved {
		t.Fatal("tokens without a usage event must be NOT_OBSERVED")
	}
}

// TestAccessNoneRemovesTheCopyBeforeDispatch proves that under --access none
// every arm's context is produced from the copy, the copy is gone before the
// agent runs, and the agent runs in an empty directory under the no-access
// skeleton.
func TestAccessNoneRemovesTheCopyBeforeDispatch(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	script := filepath.Join(t.TempDir(), "agent.sh")
	body := "#!/bin/sh\nprintf 'cwd=%s entries=%s\\n' \"$PWD\" \"$(ls -A | wc -l | tr -d ' ')\"\nprintf '%s' '```json\n{\"claims\":[]}\n```\n'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	configuration := options{tasks: writeManifest(t, snapshot), arms: []string{"none", "grep"}, agent: "script", agentCommand: script, timeout: time.Minute, limit: 20, access: "none"}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	if document.Access != "none" || document.SkeletonSHA256 != digestOf(skeletonNoAccess) {
		t.Fatalf("access %q skeleton %s", document.Access, document.SkeletonSHA256)
	}
	grep := document.Details[0].Arms["grep"]
	if !strings.Contains(grep.Context, "tests/help_test.rs") {
		t.Fatalf("the grep context must still come from the copy: %q", grep.Context)
	}
	for name, arm := range document.Details[0].Arms {
		if arm.Error != "" || !strings.Contains(arm.Reply, "entries=0\n") || arm.ToolCalls != notObserved {
			t.Fatalf("%s: %+v", name, arm)
		}
		cwd := strings.TrimPrefix(strings.SplitN(arm.Reply, " ", 2)[0], "cwd=")
		if _, err := os.Stat(cwd); err == nil {
			t.Fatalf("%s: the empty directory %s must be removed after the task", name, cwd)
		}
	}
	summary := document.Arms["none"].(map[string]any)
	if summary["tool_calls"] != notObserved || summary["explored_tasks"] != notObserved {
		t.Fatalf("script agents observe no tool calls: %v", summary)
	}
}

func TestRunOptionsRejectIncompleteArms(t *testing.T) {
	cases := [][]string{
		{"--tasks", "x.json", "--arms", "corvint"},
		{"--tasks", "x.json", "--arms", "none"},
		{"--tasks", "x.json", "--arms", "none", "--agent", "script"},
		{"--tasks", "x.json", "--arms", "bogus", "--model", "m"},
		{"--arms", "none", "--model", "m"},
	}
	for _, arguments := range cases {
		if _, err := parseRunOptions(arguments); err == nil {
			t.Errorf("%v: expected an error", arguments)
		}
	}
	if _, err := parseRunOptions([]string{"--tasks", "x.json", "--arms", "none,grep", "--model", "m"}); err != nil {
		t.Fatal(err)
	}
}

func TestGoldPlacementSeparatesResultRowsFromMentions(t *testing.T) {
	gold := map[string][]string{"source-file": {"pkg/a.go"}}
	packet := `{"context":{"results":[{"id":"pkg/b.go:Run","evidence":[{"path":"pkg/b.go"}]}],"exclusions":{"samples":["pkg/a.go"]}}}`
	if anywhere, asResult := goldPlacement(packet, gold); anywhere != 1 || asResult != 0 {
		t.Fatalf("mention-only packet: anywhere=%v asResult=%v", anywhere, asResult)
	}
	if anywhere, asResult := goldPlacement(`{"results":[{"id":"pkg/a.go:Run"}]}`, gold); anywhere != 1 || asResult != 1 {
		t.Fatalf("result-row packet: anywhere=%v asResult=%v", anywhere, asResult)
	}
	if anywhere, asResult := goldPlacement("score=3 pkg/c.go\nscore=1 pkg/a.go\n", gold); anywhere != 1 || asResult != 1 {
		t.Fatalf("grep listing: anywhere=%v asResult=%v", anywhere, asResult)
	}
}

func TestTrialReusesIdenticalPromptsAndRunsWorkersUnderNoAccess(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	tasks := writeManifest(t, snapshot)
	first := options{tasks: tasks, arms: []string{"none", "grep"}, agent: "script", agentCommand: fakeAgent(t, goldReply, ""), timeout: time.Minute, limit: 20, access: "none", workers: 2}
	document, err := trial(context.Background(), first, newAgent(first))
	if err != nil {
		t.Fatal(err)
	}
	prior := filepath.Join(t.TempDir(), "prior.json")
	if code := write(document, prior, io.Discard, os.Stderr); code != 0 {
		t.Fatal("write failed")
	}
	second := first
	second.agentCommand = fakeAgent(t, "```json\n{\"claims\":[]}\n```", "")
	second.reuse = prior
	reused, err := trial(context.Background(), second, newAgent(second))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range second.arms {
		arm := reused.Details[0].Arms[name]
		if !strings.HasPrefix(arm.ReusedFrom, "sha256:") || arm.Metrics["success"] != 1 || arm.Reply != document.Details[0].Arms[name].Reply {
			t.Fatalf("%s: reply must come from the prior report: %+v", name, arm)
		}
		if reused.Arms[name].(map[string]any)["reused"] != 1 {
			t.Fatalf("%s summary must count the reuse: %v", name, reused.Arms[name])
		}
	}
	second.reuse = ""
	fresh, err := trial(context.Background(), second, newAgent(second))
	if err != nil {
		t.Fatal(err)
	}
	if arm := fresh.Details[0].Arms["grep"]; arm.ReusedFrom != "" || arm.Metrics["abstained"] != 1 {
		t.Fatalf("without --reuse the new agent's reply must be used: %+v", arm)
	}
	if _, err := parseRunOptions([]string{"--tasks", tasks, "--agent", "script", "--agent-command", "x", "--workers", "2"}); err == nil {
		t.Fatal("workers above 1 must need --access none")
	}
}

// CWT-V0-004: an older report recorded a failed invocation without an error,
// so reuse reads the exit code and re-runs it; a non-zero exit that left a
// claims block was scored and stays reusable.
func TestReuseRerunsANonZeroExitWithoutClaims(t *testing.T) {
	failed := &armRecord{PromptSHA256: "p", WallMs: float64(10), ExitCode: float64(1), Reply: "refused"}
	scored := &armRecord{PromptSHA256: "p", WallMs: float64(10), ExitCode: float64(1), Reply: goldReply}
	source := &reuseSource{identity: "sha256:x", records: map[string]*armRecord{"t\x00none": failed, "t\x00grep": scored}}
	if source.apply("t", "none", &armRecord{PromptSHA256: "p"}) {
		t.Fatal("an invocation that exited 1 without a claims block was reused")
	}
	if !source.apply("t", "grep", &armRecord{PromptSHA256: "p"}) {
		t.Fatal("a non-zero exit that left a claims block must stay reusable")
	}
}

func TestTrialCheckpointsEveryInvocationAndResumesOnlyTheUnfinished(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	output := filepath.Join(t.TempDir(), "report.json")
	first := options{tasks: writeManifest(t, snapshot), arms: []string{"none", "grep"}, agent: "script", agentCommand: fakeAgent(t, goldReply, ""), timeout: time.Minute, limit: 20, access: "none", workers: 1, output: output}
	document, err := trial(context.Background(), first, newAgent(first))
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := checkpointPath(output)
	raw, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Fatalf("the run must leave its last checkpoint for the wrapper to remove: %v", err)
	}
	var partial report
	if err := json.Unmarshal(raw, &partial); err != nil || !partial.Partial || len(partial.Details) != 1 {
		t.Fatalf("checkpoint = %s (%v)", raw, err)
	}
	// Forge an interrupted run: the grep invocation never finished.
	partial.Details[0].Arms["grep"].WallMs, partial.Details[0].Arms["grep"].Reply = notObserved, ""
	forged, _ := json.Marshal(partial)
	if err := os.WriteFile(checkpoint, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	second := first
	second.agentCommand = fakeAgent(t, "```json\n{\"claims\":[]}\n```", "")
	second.resume, second.reuse = true, checkpoint
	resumed, err := trial(context.Background(), second, newAgent(second))
	if err != nil {
		t.Fatal(err)
	}
	none, grep := resumed.Details[0].Arms["none"], resumed.Details[0].Arms["grep"]
	if none.ReusedFrom == "" || none.Reply != document.Details[0].Arms["none"].Reply {
		t.Fatalf("the finished arm must be reused: %+v", none)
	}
	if grep.ReusedFrom != "" || grep.Metrics["abstained"] != 1 {
		t.Fatalf("the unfinished arm must be rerun: %+v", grep)
	}
	if _, err := parseRunOptions([]string{"--tasks", first.tasks, "--agent", "script", "--agent-command", "x", "--access", "none", "--resume"}); err == nil {
		t.Fatal("--resume must need --output")
	}
}

func TestHistorySnapshotCarriesCommitsAndLeavesTheSourceUntouched(t *testing.T) {
	source := generateRepository(t)
	base := strings.TrimSpace(mustGit(t, source, "rev-parse", "HEAD~1"))
	before := mustGit(t, source, "worktree", "list")
	gitDir := filepath.Join(source, ".git")
	entriesBefore, _ := os.ReadDir(gitDir)
	space := newWorkspaces()
	defer space.close()
	item := task{ID: "h1", Kind: "change", Repo: "example/gen", BaseCommit: base, ChangedFile: "cache/cache.go", Text: "t"}
	root, err := space.open(context.Background(), options{history: source}, item)
	if err != nil {
		t.Fatal(err)
	}
	if head := strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD")); head != base {
		t.Fatalf("copy HEAD = %s, want %s", head, base)
	}
	if space.commits[root] != 2 {
		t.Fatalf("history commits = %d, want 2 (base and its parent)", space.commits[root])
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatal("the copy must be the base tree, not the tip")
	}
	entriesAfter, _ := os.ReadDir(gitDir)
	if after := mustGit(t, source, "worktree", "list"); after != before || len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("the source repository must be untouched: worktrees %q -> %q, .git entries %d -> %d", before, after, len(entriesBefore), len(entriesAfter))
	}
	fallback := task{ID: "h2", Kind: "retrieval", Repo: "example/gen", BaseCommit: "0000000000000000000000000000000000000000", Snapshot: fixtureSnapshot(t), Text: "t"}
	other, err := space.open(context.Background(), options{history: source}, fallback)
	if err != nil || space.commits[other] != 0 {
		t.Fatalf("an unknown base must fall back to the chunk rebuild: %v %d", err, space.commits[other])
	}
}

func mustGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	output, err := git(context.Background(), root, arguments...)
	if err != nil {
		t.Fatalf("git %v: %v", arguments, err)
	}
	return output
}

func TestCorvintPrologueBindsClaimsToRows(t *testing.T) {
	cases := []struct{ name string }{{name: "CWT-V0-003 corvint prologue says to act on action and bounds confidence"}}
	t.Run(cases[0].name, func(t *testing.T) {
		prologue := prologues["corvint"]
		for _, want := range []string{"`action` says what to do with the file", "act on it", "`medium` or `low` row backs at most `likely`", "NO_CANDIDATES"} {
			if !strings.Contains(prologue, want) {
				t.Fatalf("corvint prologue lacks %q", want)
			}
		}
	})
}

func TestAiderArmRunsTheProducerWithTheSameSeeds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub producer needs sh")
	}
	stub := filepath.Join(t.TempDir(), "map.sh")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf 'rank=1 tests/help_test.rs\\nrank=2 %s\\n' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := fixtureSnapshot(t)
	configuration := options{tasks: writeManifest(t, snapshot), arms: []string{"aider"}, agent: "script", agentCommand: fakeAgent(t, goldReply, ""), timeout: time.Minute, limit: 20, aiderCommand: stub}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	arm := document.Details[0].Arms["aider"]
	if !strings.HasPrefix(arm.Context, "rank=1 tests/help_test.rs") || !strings.Contains(arm.Context, "--limit 20") || !strings.Contains(arm.Context, "--task-file") {
		t.Fatalf("aider context = %q", arm.Context)
	}
	if arm.Metrics["success"] != 1 || arm.Claims[0].Grounded != true || document.Aider.(map[string]any)["command"] != stub {
		t.Fatalf("aider arm must score and record its producer: %+v %v", arm.Metrics, document.Aider)
	}
	if _, err := parseRunOptions([]string{"--tasks", configuration.tasks, "--arms", "aider", "--agent", "script", "--agent-command", "x"}); err == nil {
		t.Fatal("the aider arm must need --aider-command")
	}
}

func TestRetrievalMetricsReadTheContextRowsAndTheHardGold(t *testing.T) {
	gold := map[string][]string{"source-file": {"src/help.rs", "src/other.rs"}, "test-file": {"tests/help_test.rs"}}
	item := &taskRecord{Gold: gold, Subject: "src/help.rs", GoldAtBase: []string{"src/other.rs"}, GoldChecked: true}
	listing := &armRecord{Context: "score=9 src/help.rs\nscore=9 src/noise.rs\nscore=8 src/other.rs\nscore=7 tests/help_test.rs", Reply: "```json\n{\"claims\":[{\"kind\":\"test-file\",\"value\":\"tests/help_test.rs\",\"confidence\":\"likely\",\"evidence\":\"none\"},{\"kind\":\"source-file\",\"value\":\"src/wrong.rs\",\"confidence\":\"likely\",\"evidence\":\"none\"}]}\n```"}
	scoreArm(listing, item)
	want := map[string]float64{"packet_top_1": 0, "packet_top_3": 1, "hard_gold_in_context": 1, "hard_success": 0, "success_retrievable": 1, "f1": 0.4}
	for key, value := range want {
		if listing.Metrics[key] != value {
			t.Errorf("listing %s = %v, want %v", key, listing.Metrics[key], value)
		}
	}
	packet := &armRecord{Context: `{"results":[{"id":"src/other.rs"},{"id":"src/noise.rs"}]}`, Reply: "```json\n{\"claims\":[{\"kind\":\"source-file\",\"value\":\"src/other.rs\",\"confidence\":\"certain\",\"evidence\":\"none\"}]}\n```"}
	scoreArm(packet, &taskRecord{Gold: gold})
	if packet.Metrics["packet_top_1"] != 1 || packet.Metrics["f1"] != 0.5 {
		t.Errorf("packet rows: %v", packet.Metrics)
	}
	for _, absent := range []string{"hard_success", "hard_gold_in_context", "success_retrievable"} {
		if _, present := packet.Metrics[absent]; present {
			t.Errorf("%s must be absent without a subject or a base check", absent)
		}
	}
	if got := hardGold(map[string][]string{"test-file": {"Tests/PlaybackTests.swift", "Tests/OtherTests.swift"}}, "Sources/Playback.swift"); len(got) != 1 || !got["Tests/OtherTests.swift"] {
		t.Errorf("hard gold drops the stem counterpart: %v", got)
	}
	answered := &armRecord{Reply: "```json\n{\"claims\":[{\"kind\":\"answer\",\"value\":\"42\",\"confidence\":\"certain\",\"evidence\":\"none\"},{\"kind\":\"test-file\",\"value\":\"tests/help_test.rs\",\"confidence\":\"certain\",\"evidence\":\"none\"}]}\n```"}
	scoreArm(answered, &taskRecord{Gold: map[string][]string{"answer": {"42"}, "test-file": {"tests/help_test.rs"}}})
	if answered.Metrics["success"] != 1 || answered.Metrics["f1"] != 0.666667 {
		t.Errorf("a TRUE answer is not a correct path, so f1 stays within [0,1]: %v", answered.Metrics)
	}
}

// nonReproducingReports are the committed reports whose recorded
// CWT-V0-013 metrics no committed scorer produces (decision 0211). They stay
// as recorded; the test requires that they still differ, so the set is exact.
var nonReproducingReports = map[string]bool{
	"cw-trial-unseen-beamfall-apple-run-1.json": true,
	"cw-trial-unseen-beamfall-apple-run-2.json": true,
	"cw-trial-unseen-beamfall-run-1.json":       true,
}

func TestCommittedReportsReproduceTheirRetrievalMetricsUnderScore(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "benchmarks", "results", "cw-trial-*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no committed cw-trial reports: %v", err)
	}
	checked := 0
	for _, path := range paths {
		recorded, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(recorded, []byte(`"packet_top_1"`)) {
			continue
		}
		checked++
		var rescored bytes.Buffer
		if code := run(context.Background(), []string{"score", "--report", path}, &rescored, io.Discard); code != 0 {
			t.Fatalf("%s: score failed", path)
		}
		name := filepath.Base(path)
		if reproduces := bytes.Equal(recorded, rescored.Bytes()); reproduces == nonReproducingReports[name] {
			t.Errorf("%s: reproduces under score = %v, listed as non-reproducing = %v", name, reproduces, nonReproducingReports[name])
		}
	}
	if checked == 0 {
		t.Fatal("no committed report carries CWT-V0-013 metrics")
	}
}
