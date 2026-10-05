//go:build darwin || linux

package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// continuationOptions shape one checkpointed continuation fixture.
// continuations "" omits the policy key; laneTokens and programTokens are the
// lane and program input/output token caps ("0" disables them).
type continuationOptions struct {
	host          string
	continuations string
	laneTokens    string
	laneTurns     string
	programTokens string
	programTurns  string
	programWall   string
	wallSeconds   int
}

type continuationFixture struct {
	s        *leaseStore
	ticketID string
	scripts  string
	config   store.ProgramConfig
	self     string
}

// codexContinuationHost is a fake Codex host. While "stall-implement" (or
// "stall-integrate") exists in the scripts directory, a fresh implement (or
// integrate) run writes hello.txt "partial", reports its session, and never
// finishes, so only the stage wall stops it; it execs its sleep, so the
// supervisor's drain, which stops the newest group member first, leaves no
// shell to finish the stage after the wall. A resume records its argv, its
// prompt, the hello.txt it finds and its working directory, then (unless
// "stuck" exists, which makes it never finish) completes the stage.
func codexContinuationHost(scripts, claim string) string {
	return `#!/bin/sh
input="$(cat)"
S="` + scripts + `"
case " $* " in
*" resume "*)
  printf '%s\n' "$*" >> "$S/resume-args"
  printf '%s\n' "$input" >> "$S/resume-prompt"
  pwd >> "$S/resume-pwd"
  case " $* " in
  *" resume integrate-session "*)
    echo '{"type":"thread.started","thread_id":"integrate-session"}'
    echo '{"type":"turn.started"}'
    if [ -f "$S/stuck" ]; then exec sleep 120; fi
    echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"HANDOFF\",\"summary\":\"verified\",\"nextAction\":\"integrate\"}"}}'
    ;;
  *)
    cat hello.txt >> "$S/seen"
    echo '{"type":"thread.started","thread_id":"implement-session"}'
    echo '{"type":"turn.started"}'
    if [ -f "$S/stuck" ]; then exec sleep 120; fi
    printf 'changed\n' > hello.txt
    echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"summary\":\"continued\",\"nextAction\":\"review\"}"}}'
    ;;
  esac
  ;;
*)
  case "$input" in
  *"Read-only integration verification"*)
    echo '{"type":"thread.started","thread_id":"integrate-session"}'
    echo '{"type":"turn.started"}'
    if [ -f "$S/stall-integrate" ]; then : > "$S/started-integrate"; exec sleep 120; fi
    echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"HANDOFF\",\"summary\":\"verified\",\"nextAction\":\"integrate\"}"}}'
    ;;
  *)
    case " $* " in
    *" workspace-write "*)
      pwd > "$S/first-pwd"
      printf 'partial\n' > hello.txt
      echo '{"type":"thread.started","thread_id":"implement-session"}'
      echo '{"type":"turn.started"}'
      if [ -f "$S/stall-implement" ]; then : > "$S/started"; exec sleep 120; fi
      printf 'changed\n' > hello.txt
      echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"summary\":\"built\",\"nextAction\":\"review\"}"}}'
      ;;
    *)
      echo '{"type":"thread.started","thread_id":"review-session"}'
      echo '{"type":"turn.started"}'
      echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"REVIEW\",\"accepted\":true,\"claims\":[\"` + claim + `\"],\"summary\":\"verified\",\"nextAction\":\"integrate\"}"}}'
      ;;
    esac
    ;;
  esac
  ;;
esac
echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
`
}

// openCodeContinuationHost is a fake OpenCode host whose fresh implement run
// writes hello.txt "partial", reports session ses_implement in its first
// step, and never finishes. A resume must fork ses_implement; it records its
// argv and the hello.txt it finds and builds from the fork ses_fork.
func openCodeContinuationHost(scripts string) string {
	return `#!/bin/sh
cat >/dev/null
S="` + scripts + `"
emit() {
  printf '{"type":"step_start","timestamp":1,"sessionID":"%s","part":{"type":"step-start"}}\n' "$1"
  printf '{"type":"text","timestamp":4,"sessionID":"%s","part":{"type":"text","text":"%s"}}\n' "$1" "$2"
  printf '{"type":"step_finish","timestamp":5,"sessionID":"%s","part":{"type":"step-finish","reason":"stop","cost":0,"tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}}\n' "$1"
}
case " $* " in
*" --session ses_implement --fork "*)
  printf '%s\n' "$*" >> "$S/resume-args"
  cat hello.txt >> "$S/seen"
  printf 'changed\n' > hello.txt
  emit ses_fork '{\"kind\":\"BUILT\",\"summary\":\"continued\",\"nextAction\":\"review\"}'
  ;;
*)
  printf 'partial\n' > hello.txt
  printf '{"type":"step_start","timestamp":1,"sessionID":"ses_implement","part":{"type":"step-start"}}\n'
  : > "$S/started"
  exec sleep 120
  ;;
esac
`
}

// newContinuationFixture is a real Git queue repository with a pinned fake
// host and a pinned fake Core CLI under a supervision policy shaped by o.
func newContinuationFixture(t *testing.T, o continuationOptions) *continuationFixture {
	t.Helper()
	for _, d := range []*string{&o.laneTokens, &o.programTokens} {
		if *d == "" {
			*d = "0"
		}
	}
	if o.programTurns == "" {
		o.programTurns = "8"
	}
	if o.programWall == "" {
		o.programWall = "600"
	}
	if o.wallSeconds == 0 {
		o.wallSeconds = 4
	}
	s := newLeaseStore(t)
	payload := createPayload("continuation")
	effects, _ := payload.Obj.Get("effects")
	effects.Obj.Set("touchPaths", wire.Strings([]string{"hello.txt"}))
	report := mutate(t, s.repo, envelope("create-continuation", "CREATE", "", "", payload))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %+v", report)
	}
	multiCommitted(t, s.repo.PrimaryWorktree, "hello.txt", "base\n")
	scripts := fixture.TempDirOutside(t)
	claim := string(wire.Sum([]byte("it exists")))
	name, body, model := "codex", codexContinuationHost(scripts, claim), "pinned-model"
	switch o.host {
	case supervisor.HostOpenCode:
		name, body, model = "opencode", openCodeContinuationHost(scripts), "local/probe"
	case supervisor.HostClaudeCode:
		name, body = "claude", "#!/bin/sh\nexit 3\n"
	}
	host := filepath.Join(scripts, name)
	hostRaw := multiScript(t, host, body)
	core := filepath.Join(scripts, "core")
	coreRaw := multiScript(t, core, "#!/bin/sh\nprintf '{\"ok\":true,\"context\":{\"state\":\"READY\",\"revision\":\"%s\",\"freshness\":{\"state\":\"fresh\"}}}\\n' \"$(git rev-parse HEAD^{tree})\"\n")

	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("gates", wire.Array())
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	lane, _ := budgets.Obj.Get("lane")
	lane.Obj.Set("inputTokens", str(o.laneTokens))
	lane.Obj.Set("outputTokens", str(o.laneTokens))
	if o.laneTurns != "" {
		lane.Obj.Set("turns", str(o.laneTurns))
	}
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(string(wire.Sum([]byte(host)))), "fileSha256", str(string(wire.Sum(hostRaw))), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER", "REVIEWER", "VERIFIER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	supervision := obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("1"),
		"program", obj("turns", str(o.programTurns), "wallClockMinutes", str(o.programWall), "inputTokens", str(o.programTokens), "outputTokens", str(o.programTokens)))
	if o.host != "" {
		supervision.Obj.Set("host", str(o.host))
	}
	if o.continuations != "" {
		supervision.Obj.Set("continuations", str(o.continuations))
	}
	v.Obj.Set("supervision", supervision)
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("continuation-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := store.ProgramConfig{Profile: snapshot.SupervisedProfile, Executable: host, ExecutableSHA256: supervisor.Digest(hostRaw), Model: model, Effort: "low", WorkRoot: fixture.TempDirOutside(t), WallSeconds: o.wallSeconds, CoreExecutable: core, CoreSHA256: supervisor.Digest(coreRaw), Host: o.host}
	return &continuationFixture{s: s, ticketID: report.Ticket, scripts: scripts, config: c, self: self}
}

func (f *continuationFixture) open(t *testing.T) *store.Workflow {
	t.Helper()
	w, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", f.self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return w
}

func (f *continuationFixture) touch(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(f.scripts, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// lines reads one fake-host record file as lines; an absent file has none.
func (f *continuationFixture) lines(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.scripts, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// requestDrain records a DRAIN control beside the running owner. An unlocked
// journal read that races the owner's staged write can fail MALFORMED
// "unassigned stage slot" (a suspected reader defect reported separately from
// CAL-V0-089), so the request repeats while the control is not yet recorded
// and that error alone is ignored once it is.
func (f *continuationFixture) requestDrain() error {
	for i := 0; ; i++ {
		err := store.RequestProgramControl(context.Background(), f.s.repo, operator(), "program", "DRAIN")
		if err == nil || !strings.Contains(err.Error(), "unassigned stage slot") || i == 20 {
			return err
		}
		entries, e := store.ProgramRecords(context.Background(), f.s.repo)
		for _, p := range entries {
			if e == nil && p.ID == "program" && p.Control == "DRAIN" {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (f *continuationFixture) program(t *testing.T) snapshot.Program {
	t.Helper()
	entries, err := store.ProgramRecords(context.Background(), f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range entries {
		if p.ID == "program" {
			return p
		}
	}
	t.Fatal("program record absent")
	return snapshot.Program{}
}

// assertWaitingCheckpoint checks that a stage stopped at its wall left the
// attempt waiting, unanswered, on its preserved session and worktree, with no
// live worker, proved quiescence, and the owner released.
func (f *continuationFixture) assertWaitingCheckpoint(t *testing.T, id, session string, turns int64) *snapshot.Attempt {
	t.Helper()
	a := f.s.attempt(t, id)
	if a.Phase != "WAITING" || a.Supervision.Answer != "" || a.Supervision.SessionID != session || a.Supervision.Worker || a.Quiescence != "PROVED" || a.WorktreePath == nil {
		t.Fatalf("checkpoint phase %s answer %q session %q worker %v quiescence %s", a.Phase, a.Supervision.Answer, a.Supervision.SessionID, a.Supervision.Worker, a.Quiescence)
	}
	if a.Supervision.Turns.Int() != turns {
		t.Fatalf("turns %d, want %d", a.Supervision.Turns.Int(), turns)
	}
	if p := f.program(t); p.Phase != "FINISHED" || !p.OwnerReleased {
		t.Fatalf("program phase %s ownerReleased %v", p.Phase, p.OwnerReleased)
	}
	return a
}

// TestCALV0089_CodexContinuationResumesPreservedSession drives one Codex
// program whose implement and integrate stages each reach their stage wall:
// each continues the recorded session in the same preserved worktree, the
// wall-time checkpoint stays as a per-turn candidate ref, the continuation
// prompt names the continuation, and integration lands exactly once.
func TestCALV0089_CodexContinuationResumesPreservedSession(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{continuations: "3"})
	f.config.OwnIntegrationCheckout = true
	f.touch(t, "stall-implement")
	w := f.open(t)
	ctx := context.Background()
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	if a.Supervision.Turns.Int() != 2 {
		t.Fatalf("implement turns %d, want 2", a.Supervision.Turns.Int())
	}
	args := f.lines(t, "resume-args")
	if len(args) != 1 || !strings.Contains(" "+args[0]+" ", " exec resume implement-session ") {
		t.Fatalf("resume argv %q", args)
	}
	if seen := f.lines(t, "seen"); len(seen) != 1 || seen[0] != "partial" {
		t.Fatalf("continuation found hello.txt %q, want the preserved partial work", seen)
	}
	first, resumed := f.lines(t, "first-pwd"), f.lines(t, "resume-pwd")
	if len(first) != 1 || len(resumed) != 1 || first[0] != resumed[0] {
		t.Fatalf("continuation worktree %q, want the preserved %q", resumed, first)
	}
	if prompt, _ := os.ReadFile(filepath.Join(f.scripts, "resume-prompt")); !strings.Contains(string(prompt), "checkpointed continuation 1 of 3") {
		t.Fatalf("continuation prompt %s", prompt)
	}
	refs := strings.Fields(multiGit(t, f.s.repo.PrimaryWorktree, "for-each-ref", "--format=%(refname)", "refs/corvint/tasks/program/"))
	if len(refs) != 2 {
		t.Fatalf("per-turn candidate refs %q, want 2", refs)
	}
	for i, want := range []string{"partial", "changed"} {
		if body := multiGit(t, f.s.repo.PrimaryWorktree, "show", refs[i]+":hello.txt"); body != want {
			t.Fatalf("turn %d checkpoint hello.txt %q, want %q", i+1, body, want)
		}
	}

	a, err = w.RunRole(ctx, "reviewer", "")
	if err != nil || a.Phase != "READY_FOR_INTEGRATION" {
		t.Fatalf("review: %+v %v", a, err)
	}
	rec := f.s.record(t, f.ticketID)
	grant := obj("grantId", str("g"), "actor", str("tester"), "operation", str("INTEGRATE"), "targetRevision", str(string(rec.AcceptanceRevision)), "scope", wire.Strings([]string{transaction.IntegrationScope(a.BaseCommit, *a.CandidateTreeOid, "main")}))
	if r := mutate(t, f.s.repo, envelope("grant-integrate", mutation.OpGrantApproval, f.ticketID, string(rec.Revision), grant)); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("grant: %+v", r)
	}
	base := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD")
	f.touch(t, "stall-integrate")
	if _, err = w.RunRole(ctx, "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.scripts, "started-integrate")); err != nil {
		t.Fatalf("integrate stage did not stall: %v", err)
	}
	if args = f.lines(t, "resume-args"); len(args) != 2 || !strings.Contains(" "+args[1]+" ", " exec resume integrate-session ") {
		t.Fatalf("integrate resume argv %q", args)
	}
	if n := multiGit(t, f.s.repo.PrimaryWorktree, "rev-list", "--count", base+"..HEAD"); n != "1" {
		t.Fatalf("queue checkout gained %s commits, want exactly 1", n)
	}
	if head := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD"); head != f.program(t).CandidateCommit {
		t.Fatalf("queue HEAD %s, want candidate %s", head, f.program(t).CandidateCommit)
	}
	if stored := f.s.attempt(t, a.AttemptID); stored.Phase != "COMPLETED" {
		t.Fatalf("attempt phase %s, want COMPLETED", stored.Phase)
	}
}

// TestCALV0089_OpenCodeContinuationForksPreservedSession proves the
// OpenCode adapter continues a stage stopped at its wall by forking the
// session its first step reported, in the preserved worktree.
func TestCALV0089_OpenCodeContinuationForksPreservedSession(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{host: supervisor.HostOpenCode, continuations: "2"})
	a, err := f.open(t).RunRole(context.Background(), "implementer", "")
	if err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	if _, err := os.Stat(filepath.Join(f.scripts, "started")); err != nil {
		t.Fatalf("implement stage did not stall: %v", err)
	}
	if args := f.lines(t, "resume-args"); len(args) != 1 || !strings.Contains(args[0], "--session ses_implement --fork") {
		t.Fatalf("resume argv %q", args)
	}
	if seen := f.lines(t, "seen"); len(seen) != 1 || seen[0] != "partial" {
		t.Fatalf("continuation found hello.txt %q, want the preserved partial work", seen)
	}
	if a.Supervision.Turns.Int() != 2 || f.program(t).SessionID != "ses_fork" {
		t.Fatalf("turns %d session %q", a.Supervision.Turns.Int(), f.program(t).SessionID)
	}
}

// TestCALV0089_ContinuationBoundThenOperatorRestart proves the policy bound
// stops automatic continuation with the attempt waiting, unanswered, on its
// preserved session and worktree with the owner released; a restarted
// supervisor continues it only after an explicit operator retry, in the same
// worktree, without starting a new session.
func TestCALV0089_ContinuationBoundThenOperatorRestart(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{continuations: "1"})
	f.touch(t, "stall-implement", "stuck")
	a, err := f.open(t).RunRole(context.Background(), "implementer", "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded continuation error %v, want the stage wall", err)
	}
	a = f.assertWaitingCheckpoint(t, a.AttemptID, "implement-session", 2)
	if args := f.lines(t, "resume-args"); len(args) != 1 {
		t.Fatalf("resumes %q, want exactly the one allowed continuation", args)
	}

	// Reopening as a restarted supervisor takes ownership without answering
	// the checkpoint or touching its session and worktree.
	w := f.open(t)
	if r := f.s.attempt(t, a.AttemptID); r.Phase != "WAITING" || r.Supervision.Answer != "" || r.Supervision.SessionID != "implement-session" || r.Supervision.Turns.Int() != 2 || *r.WorktreePath != *a.WorktreePath {
		t.Fatalf("reopened checkpoint phase %s answer %q session %q turns %d", r.Phase, r.Supervision.Answer, r.Supervision.SessionID, r.Supervision.Turns.Int())
	}
	if err = os.Remove(filepath.Join(f.scripts, "stuck")); err != nil {
		t.Fatal(err)
	}
	if err = w.Resume(true); err != nil {
		t.Fatalf("operator retry: %v", err)
	}
	if a, err = w.RunRole(context.Background(), "implementer", ""); err != nil || a.Phase != "BUILT" || a.Supervision.Turns.Int() != 3 {
		t.Fatalf("restarted continuation: %+v %v", a, err)
	}
	args := f.lines(t, "resume-args")
	if len(args) != 2 || !strings.Contains(" "+args[1]+" ", " exec resume implement-session ") {
		t.Fatalf("restart resume argv %q", args)
	}
	first, resumed := f.lines(t, "first-pwd"), f.lines(t, "resume-pwd")
	if len(resumed) != 2 || resumed[0] != first[0] || resumed[1] != first[0] {
		t.Fatalf("resume worktrees %q, want the preserved %q", resumed, first)
	}
	if seen := f.lines(t, "seen"); len(seen) != 2 || seen[1] != "partial" {
		t.Fatalf("restart found hello.txt %q, want the preserved partial work", seen)
	}
}

// programCapRefusal reports the native SPAWNING refusal of the shared
// program turn/wall cap.
func programCapRefusal(err error) bool {
	return err != nil && strings.Contains(err.Error(), wire.CodeLimitExceeded) && strings.Contains(err.Error(), "shared program turn/wall cap")
}

// TestCALV0089_TurnCapsBoundContinuation proves the lane turn cap and the
// shared program turn cap each end continuation before the policy bound,
// leaving the checkpoint unanswered rather than consuming a turn the cap
// refuses, and a later operator retry is refused by that cap rather than run.
func TestCALV0089_TurnCapsBoundContinuation(t *testing.T) {
	for name, tc := range map[string]struct {
		o       continuationOptions
		refused func(error) bool
	}{
		"program": {continuationOptions{continuations: "3", programTurns: "2"}, programCapRefusal},
		"lane": {continuationOptions{continuations: "3", laneTurns: "2"}, func(err error) bool {
			return err != nil && strings.Contains(err.Error(), wire.CodeLimitExceeded) && strings.Contains(err.Error(), "turn cap reached")
		}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newContinuationFixture(t, tc.o)
			f.touch(t, "stall-implement", "stuck")
			a, err := f.open(t).RunRole(context.Background(), "implementer", "")
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("capped continuation error %v, want the stage wall", err)
			}
			f.assertWaitingCheckpoint(t, a.AttemptID, "implement-session", 2)
			w := f.open(t)
			if err = w.Resume(true); err != nil {
				t.Fatalf("operator retry: %v", err)
			}
			if _, err = w.RunRole(context.Background(), "implementer", ""); !tc.refused(err) {
				t.Fatalf("retry past the %s turn cap: %v", name, err)
			}
			if args := f.lines(t, "resume-args"); len(args) != 1 {
				t.Fatalf("resumes %q, want one", args)
			}
		})
	}
}

// TestCALV0089_DrainStopsContinuation proves a program drain stops
// continuation both when it is requested while a stage runs, which stops the
// stage before its wall, and when it arrives as the stage stops at its wall,
// which leaves the checkpoint waiting unanswered.
func TestCALV0089_DrainStopsContinuation(t *testing.T) {
	t.Run("while running", func(t *testing.T) {
		f := newContinuationFixture(t, continuationOptions{continuations: "3", wallSeconds: 20})
		f.touch(t, "stall-implement")
		w := f.open(t)
		drained := make(chan error, 1)
		go func() {
			started := filepath.Join(f.scripts, "started")
			for i := 0; i < 150; i++ {
				if _, err := os.Stat(started); err == nil {
					drained <- f.requestDrain()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			drained <- errors.New("implement stage never started")
		}()
		begin := time.Now()
		a, err := w.RunRole(context.Background(), "implementer", "")
		if e := <-drained; e != nil {
			t.Fatalf("drain request: %v", e)
		}
		if err == nil || errors.Is(err, context.DeadlineExceeded) || time.Since(begin) >= 20*time.Second {
			t.Fatalf("drained stage error %v after %v, want a drain before the stage wall", err, time.Since(begin))
		}
		if args := f.lines(t, "resume-args"); len(args) != 0 {
			t.Fatalf("drained stage continued: %q", args)
		}
		if stored := f.s.attempt(t, a.AttemptID); stored.Supervision.Turns.Int() != 1 || stored.Phase == "BUILT" {
			t.Fatalf("drained attempt phase %s turns %d", stored.Phase, stored.Supervision.Turns.Int())
		}
	})
	t.Run("at the wall", func(t *testing.T) {
		f := newContinuationFixture(t, continuationOptions{continuations: "3"})
		f.touch(t, "stall-implement")
		w := f.open(t)
		// The request waits for the owner to retire, so it runs beside the
		// owner, which proceeds once the control is recorded.
		var drained chan error
		defer store.SetRunFaultForTest(func(point string) error {
			if point != "stage-finished" || drained != nil {
				return nil
			}
			drained = make(chan error, 1)
			go func() {
				drained <- f.requestDrain()
			}()
			for i := 0; i < 150; i++ {
				if f.program(t).Control == "DRAIN" {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
			return errors.New("drain was never recorded")
		})()
		a, err := w.RunRole(context.Background(), "implementer", "")
		if drained == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("drain at the wall error %v", err)
		}
		if e := <-drained; e != nil {
			t.Fatalf("drain request: %v", e)
		}
		f.assertWaitingCheckpoint(t, a.AttemptID, "implement-session", 1)
		if args := f.lines(t, "resume-args"); len(args) != 0 {
			t.Fatalf("stage drained at its wall continued: %q", args)
		}
	})
}

// TestCALV0089_IntegrateCheckpointRestartKeepsGrant proves an integrate
// stage left waiting at its wall after the continuation bound integrates
// nothing, keeps its recorded grant, and after an explicit operator retry
// resumes under that grant (a different grant is refused) and lands exactly
// once.
func TestCALV0089_IntegrateCheckpointRestartKeepsGrant(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{continuations: "1"})
	f.config.OwnIntegrationCheckout = true
	w := f.open(t)
	ctx := context.Background()
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	a, err := w.RunRole(ctx, "reviewer", "")
	if err != nil || a.Phase != "READY_FOR_INTEGRATION" {
		t.Fatalf("review: %+v %v", a, err)
	}
	rec := f.s.record(t, f.ticketID)
	grant := obj("grantId", str("g"), "actor", str("tester"), "operation", str("INTEGRATE"), "targetRevision", str(string(rec.AcceptanceRevision)), "scope", wire.Strings([]string{transaction.IntegrationScope(a.BaseCommit, *a.CandidateTreeOid, "main")}))
	if r := mutate(t, f.s.repo, envelope("grant-integrate", mutation.OpGrantApproval, f.ticketID, string(rec.Revision), grant)); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("grant: %+v", r)
	}
	base := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD")
	f.touch(t, "stall-integrate", "stuck")
	if _, err = w.RunRole(ctx, "integrator", "g"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded integrate continuation error %v", err)
	}
	stored := f.assertWaitingCheckpoint(t, a.AttemptID, "integrate-session", 4)
	if stored.Stage != "integrate" || stored.Supervision.IntegrationGrant != "g" || len(stored.PendingEffects) != 0 {
		t.Fatalf("integrate checkpoint stage %s grant %q effects %v", stored.Stage, stored.Supervision.IntegrationGrant, stored.PendingEffects)
	}
	if head := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD"); head != base {
		t.Fatal("an interrupted integrate stage moved the queue checkout")
	}
	if err = os.Remove(filepath.Join(f.scripts, "stuck")); err != nil {
		t.Fatal(err)
	}
	w = f.open(t)
	if err = w.Resume(true); err != nil {
		t.Fatalf("operator retry: %v", err)
	}
	if _, err = w.RunRole(ctx, "integrator", "other"); wire.CodeOf(err) != wire.CodeApprovalMissing {
		t.Fatalf("a different grant on the integrate checkpoint: %v", err)
	}
	if _, err = w.RunRole(ctx, "integrator", ""); err != nil {
		t.Fatalf("restarted integrate: %v", err)
	}
	if args := f.lines(t, "resume-args"); len(args) != 2 || !strings.Contains(" "+args[1]+" ", " exec resume integrate-session ") {
		t.Fatalf("integrate resume argv %q", args)
	}
	if n := multiGit(t, f.s.repo.PrimaryWorktree, "rev-list", "--count", base+"..HEAD"); n != "1" {
		t.Fatalf("queue checkout gained %s commits, want exactly 1", n)
	}
	if stored = f.s.attempt(t, a.AttemptID); stored.Phase != "COMPLETED" {
		t.Fatalf("attempt phase %s, want COMPLETED", stored.Phase)
	}
}

// TestCALV0089_ProgramWallExpiryEndsContinuation proves a stage stopped by
// the shared program wall, not its own stage wall, does not continue, and a
// later operator retry is refused by the expired program wall.
func TestCALV0089_ProgramWallExpiryEndsContinuation(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{continuations: "3", programWall: "1", wallSeconds: 600})
	f.touch(t, "stall-implement")
	a, err := f.open(t).RunRole(context.Background(), "implementer", "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("program wall error %v", err)
	}
	f.assertWaitingCheckpoint(t, a.AttemptID, "implement-session", 1)
	if args := f.lines(t, "resume-args"); len(args) != 0 {
		t.Fatalf("stage stopped by the program wall continued: %q", args)
	}
	w := f.open(t)
	if err = w.Resume(true); err != nil {
		t.Fatalf("operator retry: %v", err)
	}
	if _, err = w.RunRole(context.Background(), "implementer", ""); !programCapRefusal(err) {
		t.Fatalf("retry after the program wall: %v", err)
	}
}

// TestCALV0089_UnsupportedContinuationRefusedBeforeMutation proves a policy
// with continuations is refused UNSUPPORTED before any program record or
// worktree exists when the host reports its session only in its final result
// (Claude Code), or when a lane or program token cap is set, since no host
// reports an interrupted turn's usage.
func TestCALV0089_UnsupportedContinuationRefusedBeforeMutation(t *testing.T) {
	for name, tc := range map[string]struct {
		o    continuationOptions
		want string
	}{
		"claude-code":   {continuationOptions{host: supervisor.HostClaudeCode, continuations: "2"}, "reports its session only in its final result"},
		"lane tokens":   {continuationOptions{continuations: "2", laneTokens: "2000000"}, "token cap"},
		"program token": {continuationOptions{host: supervisor.HostOpenCode, continuations: "2", programTokens: "1000"}, "token cap"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newContinuationFixture(t, tc.o)
			programs := filepath.Join(f.s.repo.StateDir, "programs.json")
			before, _ := os.ReadFile(programs)
			_, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", f.self, f.config, f.ticketID)
			if wire.CodeOf(err) != wire.CodeUnsupported || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want UNSUPPORTED %q, got %v", tc.want, err)
			}
			if after, _ := os.ReadFile(programs); string(after) != string(before) {
				t.Fatal("refusal changed the program inventory")
			}
			if entries, _ := os.ReadDir(f.config.WorkRoot); len(entries) != 0 {
				t.Fatal("refusal created work")
			}
		})
	}
	// Without continuations each host and token cap stays admitted.
	f := newContinuationFixture(t, continuationOptions{host: supervisor.HostClaudeCode, laneTokens: "2000000"})
	if _, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", f.self, f.config, f.ticketID); err != nil {
		t.Fatalf("claude-code without continuations: %v", err)
	}
}
