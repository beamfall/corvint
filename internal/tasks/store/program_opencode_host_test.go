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

type openCodeFixture struct {
	s        *leaseStore
	ticketID string
	scripts  string
	config   store.ProgramConfig
}

// newOpenCodeFixture is a real Git queue repository, a pinned fake OpenCode
// host and a pinned fake Core CLI under an opencode supervision policy. The
// host asks one WAIT question, implements hello.txt on the resumed session,
// then accepts every claim in an independent session. Each run is two steps
// whose disjoint counters total input 7 and output 3. A resume must pass
// --session ses_implement --fork and answers from the fork ses_fork. When the
// scripts directory holds missing, the resume fails as OpenCode does for a
// missing session; when it holds recreate, the resume answers from the same
// ID, as a host that recreated the session empty would. When it holds noisy,
// a fresh run writes more standard output than the output limit keeps.
func newOpenCodeFixture(t *testing.T) *openCodeFixture {
	t.Helper()
	s := newLeaseStore(t)
	payload := createPayload("opencode")
	effects, _ := payload.Obj.Get("effects")
	effects.Obj.Set("touchPaths", wire.Strings([]string{"hello.txt"}))
	report := mutate(t, s.repo, envelope("create-opencode", "CREATE", "", "", payload))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %+v", report)
	}
	multiCommitted(t, s.repo.PrimaryWorktree, "hello.txt", "base\n")

	scripts := fixture.TempDirOutside(t)
	claim := string(wire.Sum([]byte("it exists")))
	host := filepath.Join(scripts, "opencode")
	hostRaw := multiScript(t, host, `#!/bin/sh
cat >/dev/null
emit() {
  printf '{"type":"step_start","timestamp":1,"sessionID":"%s","part":{"type":"step-start"}}\n' "$1"
  printf '{"type":"tool_use","timestamp":2,"sessionID":"%s","part":{"type":"tool","tool":"read","state":{"status":"completed"}}}\n' "$1"
  printf '{"type":"step_finish","timestamp":3,"sessionID":"%s","part":{"type":"step-finish","reason":"tool-calls","cost":0,"tokens":{"input":2,"output":1,"reasoning":0,"cache":{"read":3,"write":0}}}}\n' "$1"
  printf '{"type":"text","timestamp":4,"sessionID":"%s","part":{"type":"text","text":"%s"}}\n' "$1" "$2"
  printf '{"type":"step_finish","timestamp":5,"sessionID":"%s","part":{"type":"step-finish","reason":"stop","cost":0,"tokens":{"input":1,"output":1,"reasoning":1,"cache":{"read":0,"write":1}}}}\n' "$1"
}
case " $* " in
*" --session ses_implement --fork "*)
  printf '%s\n' "$*" > "`+scripts+`/resume-args"
  if [ -f "`+scripts+`/missing" ]; then
    printf '{"type":"error","timestamp":1,"sessionID":"","error":{"type":"unknown","message":"Session not found"}}\n'
    exit 1
  fi
  printf 'changed\n' > hello.txt
  session=ses_fork
  if [ -f "`+scripts+`/recreate" ]; then session=ses_implement; fi
  emit "$session" '{\"kind\":\"BUILT\",\"summary\":\"implemented\",\"nextAction\":\"review\"}'
  ;;
*)
  if [ -f "`+scripts+`/noisy" ]; then
    emit ses_implement '{\"kind\":\"WAIT\",\"question\":\"which greeting?\",\"summary\":\"needs input\",\"nextAction\":\"answer\"}'
    printf '{"type":"reasoning","timestamp":6,"sessionID":"ses_implement","part":{"type":"reasoning","text":"'
    head -c 20000 /dev/zero | tr '\0' 'r'
    printf '"}}\n'
    exit 0
  fi
  case "$OPENCODE_CONFIG_CONTENT" in
  *'"edit"'*)
    printf '%s\n%s|%s|%s|%s|%s\n' "$*" "$OPENCODE_DISABLE_AUTOUPDATE" "$OPENCODE_DISABLE_PROJECT_CONFIG" "$OPENCODE_PRINT_LOGS" "$OPENCODE_LOG_LEVEL" "$OPENCODE_CONFIG_CONTENT" > "`+scripts+`/review-args"
    emit ses_review '{\"kind\":\"REVIEW\",\"accepted\":true,\"claims\":[\"`+claim+`\"],\"summary\":\"verified\",\"nextAction\":\"integrate\"}'
    ;;
  *)
    printf '%s\n%s|%s|%s|%s|%s\n' "$*" "$OPENCODE_DISABLE_AUTOUPDATE" "$OPENCODE_DISABLE_PROJECT_CONFIG" "$OPENCODE_PRINT_LOGS" "$OPENCODE_LOG_LEVEL" "$OPENCODE_CONFIG_CONTENT" > "`+scripts+`/implement-args"
    emit ses_implement '{\"kind\":\"WAIT\",\"question\":\"which greeting?\",\"summary\":\"needs input\",\"nextAction\":\"answer\"}'
    ;;
  esac
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
	v.Obj.Set("supervision", obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("1"), "host", str(supervisor.HostOpenCode),
		"program", obj("turns", str("8"), "wallClockMinutes", str("600"), "inputTokens", str("0"), "outputTokens", str("0"))))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("opencode-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	c := store.ProgramConfig{Profile: snapshot.SupervisedProfile, Executable: host, ExecutableSHA256: supervisor.Digest(hostRaw), Model: "local/probe", Effort: "low", WorkRoot: fixture.TempDirOutside(t), WallSeconds: 600, CoreExecutable: core, CoreSHA256: supervisor.Digest(coreRaw), Host: supervisor.HostOpenCode}
	return &openCodeFixture{s: s, ticketID: report.Ticket, scripts: scripts, config: c}
}

// waitThenAnswer runs the first implement stage to its WAIT question and
// answers it, returning the attempt ID.
func (f *openCodeFixture) waitThenAnswer(t *testing.T, w *store.Workflow) string {
	t.Helper()
	a, err := w.RunRole(context.Background(), "implementer", "")
	if err != nil {
		t.Fatalf("implement: %v", err)
	}
	if a.Phase != "WAITING" || a.Supervision.SessionID != "ses_implement" || a.Supervision.Question != "which greeting?" {
		t.Fatalf("wait phase %s session %q question %q", a.Phase, a.Supervision.SessionID, a.Supervision.Question)
	}
	if err = w.Answer(a.Supervision.QuestionID, "hello", a.TicketRevision); err != nil {
		t.Fatalf("answer: %v", err)
	}
	return a.AttemptID
}

// TestCALV0077_OpenCodeProgramFakeHost drives one opencode program through
// the pinned fake host: implement asks a WAIT question under the
// implement permissions, the operator answers, implement forks the recorded
// session and builds from the fork, an independent review session with edits denied
// accepts every claim, and usage is re-derived in the OpenCode vocabulary.
func TestCALV0077_OpenCodeProgramFakeHost(t *testing.T) {
	f := newOpenCodeFixture(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	f.waitThenAnswer(t, w)
	args, err := os.ReadFile(filepath.Join(f.scripts, "implement-args"))
	if err != nil || string(args) != "run --standalone --format json --model local/probe#low\n1|1|1|ERROR|{\"permission\":{\"external_directory\":\"deny\",\"task\":\"deny\"}}\n" {
		t.Fatalf("implement argv and env %q %v", args, err)
	}
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if a.Phase != "BUILT" || a.CandidateTreeOid == nil || a.Supervision.AuthorSession != "ses_fork" {
		t.Fatalf("resume phase %s author %q", a.Phase, a.Supervision.AuthorSession)
	}
	if args, err = os.ReadFile(filepath.Join(f.scripts, "resume-args")); err != nil || strings.TrimSpace(string(args)) != "run --standalone --format json --model local/probe#low --session ses_implement --fork" {
		t.Fatalf("resume argv %q %v", args, err)
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
	if args, err = os.ReadFile(filepath.Join(f.scripts, "review-args")); err != nil || string(args) != "run --standalone --format json --model local/probe#low\n1|1|1|ERROR|{\"permission\":{\"edit\":\"deny\",\"external_directory\":\"deny\",\"task\":\"deny\"}}\n" {
		t.Fatalf("review argv and env %q %v", args, err)
	}
	entries, err := store.ProgramRecords(ctx, f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range entries {
		if p.ID == "program" {
			found = true
			if !p.UsageKnown || p.InputTokens != 21 || p.OutputTokens != 9 {
				t.Fatalf("program usage known=%v in=%d out=%d, want 21/9 from three OpenCode runs", p.UsageKnown, p.InputTokens, p.OutputTokens)
			}
		}
	}
	if !found {
		t.Fatal("program record absent")
	}
}

// TestCALV0077_OpenCodeResumeRequiresFork proves a resumed WAIT stage is
// accepted only with positive evidence that the recorded session existed: a
// forked new ID. A missing session (the fork fails) and a same-ID recreation
// (the ID is unchanged) both leave the durable attempt WAITING with the
// original session as its resume target, read back independently of the
// returned error.
func TestCALV0077_OpenCodeResumeRequiresFork(t *testing.T) {
	for _, flag := range []string{"missing", "recreate"} {
		t.Run(flag, func(t *testing.T) {
			f := newOpenCodeFixture(t)
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			attempt := f.waitThenAnswer(t, w)
			if err = os.WriteFile(filepath.Join(f.scripts, flag), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			_, runErr := w.RunRole(ctx, "implementer", "")
			if args, e := os.ReadFile(filepath.Join(f.scripts, "resume-args")); e != nil || !strings.Contains(string(args), "--session ses_implement --fork") {
				t.Fatalf("resume argv %q %v", args, e)
			}
			a, err := store.AttemptRecord(ctx, f.s.repo, attempt)
			if err != nil {
				t.Fatalf("durable attempt: %v (run error %v)", err, runErr)
			}
			if a.Phase != "WAITING" || a.Supervision.SessionID != "ses_implement" || a.Supervision.Question == "" || a.Supervision.Answer != "" || a.CandidateTreeOid != nil {
				t.Fatalf("durable attempt phase %s session %q question %q answer %q (run error %v)", a.Phase, a.Supervision.SessionID, a.Supervision.Question, a.Supervision.Answer, runErr)
			}
		})
	}
}

// TestCALV0077_OpenCodeOutputLimitUsageUnknown proves output cut at the limit
// is never complete accounting: the run's standard output overflows after
// whole steps, so the program reports usage unknown rather than the partial
// sums of the steps it kept.
func TestCALV0077_OpenCodeOutputLimitUsageUnknown(t *testing.T) {
	f := newOpenCodeFixture(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.scripts, "noisy"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	a, runErr := w.RunRole(ctx, "implementer", "")
	if runErr == nil || a == nil || a.Phase != "WAITING" {
		t.Fatalf("overflowed run %+v %v", a, runErr)
	}
	entries, err := store.ProgramRecords(ctx, f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range entries {
		if p.ID == "program" {
			if p.ResultClass != "OUTPUT_LIMIT" || p.UsageKnown {
				t.Fatalf("program phase %s class %q known %v (run error %v)", p.Phase, p.ResultClass, p.UsageKnown, runErr)
			}
			return
		}
	}
	t.Fatal("program record absent")
}
