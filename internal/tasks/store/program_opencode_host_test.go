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
// whose disjoint counters total input 7 and output 3. When the scripts
// directory holds new-session, a resumed run answers from a fresh session.
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
*" --session ses_implement "*)
  printf '%s\n' "$*" > "`+scripts+`/resume-args"
  printf 'changed\n' > hello.txt
  session=ses_implement
  if [ -f "`+scripts+`/new-session" ]; then session=ses_fresh; fi
  emit "$session" '{\"kind\":\"BUILT\",\"summary\":\"implemented\",\"nextAction\":\"review\"}'
  ;;
*)
  case "$OPENCODE_CONFIG_CONTENT" in
  *'"edit"'*)
    printf '%s\n%s|%s|%s\n' "$*" "$OPENCODE_DISABLE_AUTOUPDATE" "$OPENCODE_DISABLE_PROJECT_CONFIG" "$OPENCODE_CONFIG_CONTENT" > "`+scripts+`/review-args"
    emit ses_review '{\"kind\":\"REVIEW\",\"accepted\":true,\"claims\":[\"`+claim+`\"],\"summary\":\"verified\",\"nextAction\":\"integrate\"}'
    ;;
  *)
    printf '%s\n%s|%s|%s\n' "$*" "$OPENCODE_DISABLE_AUTOUPDATE" "$OPENCODE_DISABLE_PROJECT_CONFIG" "$OPENCODE_CONFIG_CONTENT" > "`+scripts+`/implement-args"
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
// answers it.
func (f *openCodeFixture) waitThenAnswer(t *testing.T, w *store.Workflow) {
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
}

// TestCALV0077_OpenCodeProgramFakeHost drives one opencode program through
// the pinned fake host: implement asks a WAIT question under the
// implement permissions, the operator answers, implement continues the exact
// session and builds, an independent review session with edits denied
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
	if err != nil || string(args) != "run --standalone --format json --model local/probe#low\n1|1|{\"permission\":{\"external_directory\":\"deny\",\"task\":\"deny\"}}\n" {
		t.Fatalf("implement argv and env %q %v", args, err)
	}
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if a.Phase != "BUILT" || a.CandidateTreeOid == nil {
		t.Fatalf("resume phase %s", a.Phase)
	}
	if args, err = os.ReadFile(filepath.Join(f.scripts, "resume-args")); err != nil || strings.TrimSpace(string(args)) != "run --standalone --format json --model local/probe#low --session ses_implement" {
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
	if args, err = os.ReadFile(filepath.Join(f.scripts, "review-args")); err != nil || string(args) != "run --standalone --format json --model local/probe#low\n1|1|{\"permission\":{\"edit\":\"deny\",\"external_directory\":\"deny\",\"task\":\"deny\"}}\n" {
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

// TestCALV0077_OpenCodeResumeRefusesFreshSession proves a resumed WAIT stage
// answered from a session other than the recorded one is not accepted as
// the build, because --session creates a session that does not exist.
func TestCALV0077_OpenCodeResumeRefusesFreshSession(t *testing.T) {
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
	if err = os.WriteFile(filepath.Join(f.scripts, "new-session"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := w.RunRole(ctx, "implementer", "")
	if err == nil && a.Phase == "BUILT" {
		t.Fatalf("fresh session accepted: phase %s session %q", a.Phase, a.Supervision.SessionID)
	}
}
