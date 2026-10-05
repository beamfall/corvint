package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type claudeFixture struct {
	s        *leaseStore
	ticketID string
	scripts  string
	config   store.ProgramConfig
}

// newClaudeFixture is a real Git queue repository, a pinned fake Claude Code
// host and a pinned fake Core CLI under a policy whose supervised host is
// policyHost. The host asks one WAIT question, implements hello.txt on the
// resumed session, then accepts every claim in an independent session. Each
// result object reports input 2 (+3 cache read) and output 1 tokens.
func newClaudeFixture(t *testing.T, policyHost string) *claudeFixture {
	t.Helper()
	s := newLeaseStore(t)
	payload := createPayload("claude")
	effects, _ := payload.Obj.Get("effects")
	effects.Obj.Set("touchPaths", wire.Strings([]string{"hello.txt"}))
	report := mutate(t, s.repo, envelope("create-claude", "CREATE", "", "", payload))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %+v", report)
	}
	multiCommitted(t, s.repo.PrimaryWorktree, "hello.txt", "base\n")

	scripts := fixture.TempDirOutside(t)
	claim := string(wire.Sum([]byte("it exists")))
	usage := `"usage":{"input_tokens":2,"cache_read_input_tokens":3,"output_tokens":1}`
	host := filepath.Join(scripts, "claude")
	hostRaw := multiScript(t, host, `#!/bin/sh
cat >/dev/null
result() {
  printf '{"type":"result","subtype":"success","is_error":false,"num_turns":1,"session_id":"%s","result":"%s",`+usage+`}\n' "$1" "$2"
}
case " $* " in
*" --resume claude-implement "*)
  printf '%s\n' "$*" > "`+scripts+`/resume-args"
  printf 'changed\n' > hello.txt
  result claude-implement '{\"kind\":\"BUILT\",\"summary\":\"implemented\",\"nextAction\":\"review\"}'
  ;;
*" acceptEdits "*)
  printf '%s\n' "$*" > "`+scripts+`/implement-args"
  result claude-implement '{\"kind\":\"WAIT\",\"question\":\"which greeting?\",\"summary\":\"needs input\",\"nextAction\":\"answer\"}'
  ;;
*)
  printf '%s\n' "$*" > "`+scripts+`/review-args"
  result claude-review '{\"kind\":\"REVIEW\",\"accepted\":true,\"claims\":[\"`+claim+`\"],\"summary\":\"verified\",\"nextAction\":\"integrate\"}'
  ;;
esac
`)
	core := filepath.Join(scripts, "core")
	coreRaw := multiScript(t, core, "#!/bin/sh\nprintf '{\"ok\":true,\"context\":{\"state\":\"READY\",\"revision\":\"%s\",\"freshness\":{\"state\":\"fresh\"}}}\\n' \"$(git rev-parse HEAD^{tree})\"\n")

	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("gates", wire.Array())
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(string(wire.Sum([]byte(host)))), "fileSha256", str(string(wire.Sum(hostRaw))), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER", "REVIEWER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	supervision := obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("1"),
		"program", obj("turns", str("8"), "wallClockMinutes", str("600"), "inputTokens", str("0"), "outputTokens", str("0")))
	if policyHost != "" {
		supervision.Obj.Set("host", str(policyHost))
	}
	v.Obj.Set("supervision", supervision)
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("claude-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	c := store.ProgramConfig{Profile: snapshot.SupervisedProfile, Executable: host, ExecutableSHA256: supervisor.Digest(hostRaw), Model: "pinned-model", Effort: "low", WorkRoot: fixture.TempDirOutside(t), WallSeconds: 600, CoreExecutable: core, CoreSHA256: supervisor.Digest(coreRaw), Host: supervisor.HostClaudeCode}
	return &claudeFixture{s: s, ticketID: report.Ticket, scripts: scripts, config: c}
}

// TestCALV0074_OpenWorkflowRefusesHostBeforeMutation proves a host mismatch
// is UNSUPPORTED and a missing, unpinned or digest-mismatched executable is
// CAPABILITY_UNAVAILABLE, each before any program record, worktree or lease.
func TestCALV0074_OpenWorkflowRefusesHostBeforeMutation(t *testing.T) {
	f := newClaudeFixture(t, supervisor.HostClaudeCode)
	codexPolicy := newClaudeFixture(t, "")
	unpinned := filepath.Join(f.scripts, "unpinned-claude")
	raw, err := os.ReadFile(f.config.Executable)
	if err != nil {
		t.Fatal(err)
	}
	multiScript(t, unpinned, string(raw))
	for _, tc := range []struct {
		name string
		f    *claudeFixture
		edit func(*store.ProgramConfig)
		code string
	}{
		{"codex config under claude-code policy", f, func(c *store.ProgramConfig) { c.Host = "" }, wire.CodeUnsupported},
		{"claude-code config under codex policy", codexPolicy, func(c *store.ProgramConfig) {}, wire.CodeUnsupported},
		{"unknown host", f, func(c *store.ProgramConfig) { c.Host = "opencode" }, wire.CodeUnsupported},
		{"missing executable", f, func(c *store.ProgramConfig) { c.Executable = filepath.Join(f.scripts, "absent") }, wire.CodeCapabilityUnavailable},
		{"unpinned executable", f, func(c *store.ProgramConfig) { c.Executable = unpinned }, wire.CodeCapabilityUnavailable},
		{"config digest differs", f, func(c *store.ProgramConfig) { c.ExecutableSHA256 = supervisor.Digest([]byte("other")) }, wire.CodeCapabilityUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			programs := filepath.Join(tc.f.s.repo.StateDir, "programs.json")
			attempts := filepath.Join(tc.f.s.repo.StateDir, "attempts")
			before, _ := os.ReadFile(programs)
			beforeAttempts, _ := os.ReadDir(attempts)
			c := tc.f.config
			tc.edit(&c)
			_, err := store.OpenWorkflow(context.Background(), tc.f.s.repo, operator(), "program", "/unused-self", c, tc.f.ticketID)
			if err == nil || wire.CodeOf(err) != tc.code {
				t.Fatalf("want %s, got %s %v", tc.code, wire.CodeOf(err), err)
			}
			after, _ := os.ReadFile(programs)
			afterAttempts, _ := os.ReadDir(attempts)
			if string(after) != string(before) || len(afterAttempts) != len(beforeAttempts) {
				t.Fatal("refusal changed the program or attempt inventory")
			}
			if entries, _ := os.ReadDir(c.WorkRoot); len(entries) != 0 {
				t.Fatal("refusal created work")
			}
		})
	}
}

// TestCALV0075_ClaudeCodeProgramFakeHost drives one claude-code program
// through the pinned fake host: implement asks a WAIT question, the operator
// answers, implement resumes the exact session and builds, a fresh (never
// resumed) independent review session accepts every claim, and usage is
// re-derived in the Claude Code vocabulary.
func TestCALV0075_ClaudeCodeProgramFakeHost(t *testing.T) {
	f := newClaudeFixture(t, supervisor.HostClaudeCode)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil {
		t.Fatalf("implement: %v", err)
	}
	if a.Phase != "WAITING" || a.Supervision.SessionID != "claude-implement" || a.Supervision.Question != "which greeting?" {
		t.Fatalf("wait phase %s session %q question %q", a.Phase, a.Supervision.SessionID, a.Supervision.Question)
	}
	args, err := os.ReadFile(filepath.Join(f.scripts, "implement-args"))
	if err != nil || !strings.HasPrefix(string(args), "-p --output-format json --model pinned-model --effort low ") || strings.Contains(string(args), "--resume") {
		t.Fatalf("implement argv %s %v", args, err)
	}
	if err = w.Answer(a.Supervision.QuestionID, "hello", a.TicketRevision); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if a, err = w.RunRole(ctx, "implementer", ""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if a.Phase != "BUILT" || a.CandidateTreeOid == nil {
		t.Fatalf("resume phase %s", a.Phase)
	}
	if args, err = os.ReadFile(filepath.Join(f.scripts, "resume-args")); err != nil || !strings.HasSuffix(strings.TrimSpace(string(args)), "--permission-mode acceptEdits --resume claude-implement") {
		t.Fatalf("resume argv %s %v", args, err)
	}
	if body := multiGit(t, f.s.repo.PrimaryWorktree, "show", *a.CandidateTreeOid+":hello.txt"); body != "changed" {
		t.Fatalf("candidate hello.txt %q", body)
	}
	if a, err = w.RunRole(ctx, "reviewer", ""); err != nil {
		t.Fatalf("review: %v", err)
	}
	if a.Phase != "READY_FOR_INTEGRATION" || a.Supervision.ReviewTree != *a.CandidateTreeOid {
		t.Fatalf("review phase %s question %q", a.Phase, a.Supervision.Question)
	}
	if args, err = os.ReadFile(filepath.Join(f.scripts, "review-args")); err != nil || !strings.Contains(string(args), "--permission-mode dontAsk --disallowedTools Edit,Write,NotebookEdit") || strings.Contains(string(args), "--resume") {
		t.Fatalf("review argv %s %v", args, err)
	}
	entries, err := store.ProgramRecords(ctx, f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range entries {
		if p.ID == "program" {
			found = true
			if !p.UsageKnown || p.InputTokens != 15 || p.OutputTokens != 3 {
				t.Fatalf("program usage known=%v in=%d out=%d, want 15/3 from three Claude Code turns", p.UsageKnown, p.InputTokens, p.OutputTokens)
			}
		}
	}
	if !found {
		t.Fatal("program record absent")
	}
}
