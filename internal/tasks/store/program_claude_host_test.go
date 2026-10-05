//go:build unix

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
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
	version  int
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

	f := &claudeFixture{s: s, ticketID: report.Ticket, scripts: scripts, version: 2}
	f.setPolicy(t, host, policyHost)
	c := store.ProgramConfig{Profile: snapshot.SupervisedProfile, Executable: host, ExecutableSHA256: supervisor.Digest(hostRaw), Model: "pinned-model", Effort: "low", WorkRoot: fixture.TempDirOutside(t), WallSeconds: 600, CoreExecutable: core, CoreSHA256: supervisor.Digest(coreRaw), Host: supervisor.HostClaudeCode}
	f.config = c
	return f
}

// setPolicy replaces the queue policy with one pinning executable (its path,
// the bytes and permission bits it resolves to) as the only supervised
// runtime, under supervised host policyHost ("" is Codex).
func (f *claudeFixture) setPolicy(t *testing.T, executable, policyHost string) {
	t.Helper()
	raw, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str(strconv.Itoa(f.version+1)))
	v.Obj.Set("gates", wire.Array())
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(string(wire.Sum([]byte(executable)))), "fileSha256", str(string(wire.Sum(raw))), "mode", str(fmt.Sprintf("%04o", st.Mode().Perm()))), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER", "REVIEWER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	supervision := obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("1"),
		"program", obj("turns", str("8"), "wallClockMinutes", str("600"), "inputTokens", str("0"), "outputTokens", str("0")))
	if policyHost != "" {
		supervision.Obj.Set("host", str(policyHost))
	}
	v.Obj.Set("supervision", supervision)
	id := fmt.Sprintf("claude-policy-%d", f.version+1)
	rep, e := store.PolicyUpdate(context.Background(), f.s.repo, operator(), policyRequest(id, strconv.Itoa(f.version), wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	f.version++
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
		{"unknown host", f, func(c *store.ProgramConfig) { c.Host = "gemini-cli" }, wire.CodeUnsupported},
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

// inventory is the program record and attempt count a refusal must leave
// unchanged.
func (f *claudeFixture) inventory(t *testing.T) string {
	t.Helper()
	programs, _ := os.ReadFile(filepath.Join(f.s.repo.StateDir, "programs.json"))
	attempts, _ := os.ReadDir(filepath.Join(f.s.repo.StateDir, "attempts"))
	return fmt.Sprintf("%d:%s", len(attempts), programs)
}

func (f *claudeFixture) program(t *testing.T, id string) snapshot.Program {
	t.Helper()
	entries, err := store.ProgramRecords(context.Background(), f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range entries {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("program %s absent", id)
	return snapshot.Program{}
}

// TestCALV0074_AdmissionRunsLaunchCheck proves admission refuses, before any
// record, claim or lease, a pinned runtime that launch would refuse (a
// symlink or a file without an execute bit), and that a launch refusal after
// admission settles the dispatched stage as NO_EXEC and cancels the attempt,
// leaving no live claim or reservation.
func TestCALV0074_AdmissionRunsLaunchCheck(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, want string
		make       func(f *claudeFixture) string
	}{
		{"symlink pinned by its path", "pin its target", func(f *claudeFixture) string {
			link := filepath.Join(f.scripts, "claude-link")
			if err := os.Symlink(f.config.Executable, link); err != nil {
				t.Fatal(err)
			}
			return link
		}},
		{"pinned file without execute bit", "not regular executable", func(f *claudeFixture) string {
			plain := filepath.Join(f.scripts, "claude-plain")
			raw, err := os.ReadFile(f.config.Executable)
			if err != nil || os.WriteFile(plain, raw, 0o644) != nil || os.Chmod(plain, 0o644) != nil {
				t.Fatal(err)
			}
			return plain
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaudeFixture(t, supervisor.HostClaudeCode)
			c := f.config
			c.Executable = tc.make(f)
			f.setPolicy(t, c.Executable, supervisor.HostClaudeCode)
			before := f.inventory(t)
			_, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", "/unused-self", c, f.ticketID)
			if wire.CodeOf(err) != wire.CodeCapabilityUnavailable || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want CAPABILITY_UNAVAILABLE %q, got %s %v", tc.want, wire.CodeOf(err), err)
			}
			if f.inventory(t) != before {
				t.Fatal("refusal changed the program or attempt inventory")
			}
			if entries, _ := os.ReadDir(c.WorkRoot); len(entries) != 0 {
				t.Fatal("refusal created work")
			}
		})
	}

	t.Run("launch refused after admission", func(t *testing.T) {
		f := newClaudeFixture(t, supervisor.HostClaudeCode)
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if err = os.Chmod(f.config.Executable, 0o644); err != nil {
			t.Fatal(err)
		}
		a, err := w.RunRole(ctx, "implementer", "")
		if wire.CodeOf(err) != wire.CodeCapabilityUnavailable {
			t.Fatalf("want CAPABILITY_UNAVAILABLE, got %s %v", wire.CodeOf(err), err)
		}
		p := f.program(t, "program")
		if p.Phase != "FINISHED" || p.ResultClass != "NO_EXEC" || !p.OwnerReleased || p.Quiescence != "PROVED" {
			t.Fatalf("program left %s class %s released %v quiescence %s", p.Phase, p.ResultClass, p.OwnerReleased, p.Quiescence)
		}
		if a == nil || a.Supervision == nil || a.Supervision.Worker || a.Phase != "CANCELLED" {
			t.Fatalf("attempt not released: %+v", a)
		}
		// No live claim or reservation remains: with the pinned mode
		// restored, another program claims the same ticket.
		if err = os.Chmod(f.config.Executable, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err = store.OpenWorkflow(ctx, f.s.repo, operator(), "next", self, f.config, f.ticketID); err != nil {
			t.Fatalf("reclaim after launch refusal: %v", err)
		}
		if next := f.program(t, "next"); next.CurrentAttempt != a.AttemptID || next.CurrentGeneration == string(a.Generation) {
			t.Fatalf("ticket not reclaimed: %+v", next)
		}
	})
}

// TestCALV0074_NoExecCancelBeforeRelease proves that a launch refused after
// admission cancels the attempt while its owner still holds the program. In
// the window before the cancel the program is FINISHED with proved quiescence
// but unreleased, so a competing owner is refused as long as this owner is
// live and cannot fence the cancel; the owner is released only after the
// cancel has released the claim.
func TestCALV0074_NoExecCancelBeforeRelease(t *testing.T) {
	ctx := context.Background()
	f := newClaudeFixture(t, supervisor.HostClaudeCode)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err = os.Chmod(f.config.Executable, 0o644); err != nil {
		t.Fatal(err)
	}
	windows := map[string]snapshot.Program{}
	phases := map[string]string{}
	var competing error
	restore := store.CALTestNoExecHook(func(point string) error {
		windows[point] = f.program(t, "program")
		attempts, e := store.ProgramAttempts(ctx, f.s.repo)
		if e != nil {
			return e
		}
		if a := attempts["program"]; a != nil {
			phases[point] = a.Phase
		}
		if point != "cancel" {
			return nil
		}
		other := windows[point]
		other.OwnerPID = os.Getppid()
		if other.OwnerStarted, competing = supervisor.ProcessIdentity(other.OwnerPID); competing == nil {
			var r *store.Report
			r, competing = store.ProgramTransition(ctx, f.s.repo, operator(), "queue:acme:main", "competing-owner", other)
			if competing == nil && (r == nil || r.Outcome.Outcome != mutation.OutcomeCompleted) {
				competing = fmt.Errorf("refused: %+v", r)
			}
		}
		return nil
	})
	_, err = w.RunRole(ctx, "implementer", "")
	restore()
	if err == nil {
		t.Fatal("refused launch reported success")
	}
	for _, point := range []string{"cancel", "release"} {
		p := windows[point]
		if p.OwnerReleased || p.Phase != "FINISHED" || p.Quiescence != "PROVED" || p.ResultClass != "NO_EXEC" {
			t.Fatalf("%s window program %+v", point, p)
		}
	}
	if phases["cancel"] != "WAITING" || phases["release"] != "CANCELLED" {
		t.Fatalf("attempt phases by window: %v", phases)
	}
	if competing == nil || !strings.Contains(competing.Error(), "live owner") {
		t.Fatalf("competing owner not refused as live-owned: %v", competing)
	}
	if p := f.program(t, "program"); !p.OwnerReleased || p.Phase != "FINISHED" {
		t.Fatalf("owner not released after cancel: %+v", p)
	}
	if err = os.Chmod(f.config.Executable, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err = store.OpenWorkflow(ctx, f.s.repo, operator(), "next", self, f.config, f.ticketID); err != nil {
		t.Fatalf("reclaim after cancel: %v", err)
	}
}

// noExecCrashEnv names the interruption point for the child process of
// TestCALV0074_NoExecCrashTakeover.
const noExecCrashEnv = "CORVINT_TEST_NOEXEC_CRASH"

// TestCALV0074_NoExecCrashChild is the owner process of
// TestCALV0074_NoExecCrashTakeover. It opens the program, makes the pinned
// runtime unlaunchable and exits at the named NO_EXEC point without any
// further write, standing in for an owner crash. It is skipped otherwise.
func TestCALV0074_NoExecCrashChild(t *testing.T) {
	point := os.Getenv(noExecCrashEnv)
	if point == "" {
		t.Skip("child of TestCALV0074_NoExecCrashTakeover")
	}
	var c store.ProgramConfig
	if err := json.Unmarshal([]byte(os.Getenv(noExecCrashEnv+"_CONFIG")), &c); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv(noExecCrashEnv + "_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.OpenWorkflow(context.Background(), repo, operator(), "program", self, c, os.Getenv(noExecCrashEnv+"_TICKET"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err = os.Chmod(c.Executable, 0o644); err != nil {
		t.Fatal(err)
	}
	store.CALTestNoExecHook(func(at string) error {
		if at == point {
			os.Exit(3)
		}
		return nil
	})
	_, err = w.RunRole(context.Background(), "implementer", "")
	t.Fatalf("owner was not interrupted at %s: %v", point, err)
}

// TestCALV0074_NoExecCrashTakeover proves that an owner dying inside a
// cancelling NO_EXEC settlement leaves a program that a genuinely different
// process takes over after the original owner exits: before the cancel the
// replacement cancels the stopped attempt and the ticket is reclaimable;
// after the cancel but before the release the replacement reassigns the
// program and claims the ticket again. It also pins the known limit: an owner
// dying after the attempt stops and before the program is recorded FINISHED
// leaves a STOPPING program that a replacement is refused, keeping the claim.
func TestCALV0074_NoExecCrashTakeover(t *testing.T) {
	for _, point := range []string{"stopped", "cancel", "release"} {
		t.Run(point, func(t *testing.T) {
			ctx := context.Background()
			f := newClaudeFixture(t, supervisor.HostClaudeCode)
			config, err := json.Marshal(f.config)
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(os.Args[0], "-test.run=^TestCALV0074_NoExecCrashChild$", "-test.count=1")
			child.Env = append(os.Environ(), noExecCrashEnv+"="+point, noExecCrashEnv+"_CONFIG="+string(config), noExecCrashEnv+"_ROOT="+f.s.repo.PrimaryWorktree, noExecCrashEnv+"_TICKET="+f.ticketID)
			out, err := child.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 3 {
				t.Fatalf("child did not stop at %s: %v\n%s", point, err, out)
			}
			p := f.program(t, "program")
			if p.OwnerPID != child.Process.Pid || p.OwnerReleased {
				t.Fatalf("interrupted program %+v", p)
			}
			attempts, err := store.ProgramAttempts(ctx, f.s.repo)
			if err != nil {
				t.Fatal(err)
			}
			old := attempts["program"]
			want := map[string]string{"stopped": "WAITING", "cancel": "WAITING", "release": "CANCELLED"}[point]
			if old == nil || old.Supervision == nil || old.Supervision.Worker || old.Quiescence != "PROVED" || old.Phase != want {
				t.Fatalf("interrupted attempt %+v", old)
			}
			if err = os.Chmod(f.config.Executable, 0o755); err != nil {
				t.Fatal(err)
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			again, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if point == "stopped" {
				// Known limit (CAL-V0-074): STOPPING is not a safe takeover
				// phase and the stopped attempt has no worker to recover.
				if err == nil || !strings.Contains(err.Error(), "not proved stopped") {
					t.Fatalf("takeover of a STOPPING program: %v", err)
				}
				if q := f.program(t, "program"); q.Phase != "STOPPING" || q.OwnerPID != p.OwnerPID || q.Epoch != p.Epoch {
					t.Fatalf("refused takeover changed the program: %+v", q)
				}
				if _, err = store.OpenWorkflow(ctx, f.s.repo, operator(), "next", self, f.config, f.ticketID); err == nil || !strings.Contains(err.Error(), "ATTEMPT_LIVE") {
					t.Fatalf("the stopped attempt's claim was not held: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("takeover: %v", err)
			}
			q := f.program(t, "program")
			if q.OwnerPID != os.Getpid() || q.Epoch != p.Epoch+1 || p.Phase != "FINISHED" || p.Quiescence != "PROVED" {
				t.Fatalf("takeover did not bind this process: %+v", q)
			}
			if point == "cancel" {
				if err = again.Cancel(); err != nil {
					t.Fatalf("cancel after takeover: %v", err)
				}
				if again.Attempt().Phase != "CANCELLED" {
					t.Fatalf("attempt not cancelled: %+v", again.Attempt())
				}
				if _, err = store.OpenWorkflow(ctx, f.s.repo, operator(), "next", self, f.config, f.ticketID); err != nil {
					t.Fatalf("reclaim after takeover: %v", err)
				}
				return
			}
			a := again.Attempt()
			if a == nil || a.AttemptID == old.AttemptID && a.Generation == old.Generation || !a.Live() {
				t.Fatalf("takeover did not reassign the released ticket: %+v", a)
			}
		})
	}
}

// TestCALV0074_HostSwitchAndRollback proves the documented rollback with
// distinct Claude Code and Codex executables. Every program is cancelled, a
// drained one included, with its original config and pins before the policy
// host and runtime pin change; afterwards an original config is refused
// before any mutation, the old config cannot be edited in place, and a new
// Codex program claims the released tickets. A program that was only drained
// keeps its claim and cannot be reopened under the Codex pin; restoring the
// Claude Code pin recovers bounded cancel access without a stage launch.
func TestCALV0074_HostSwitchAndRollback(t *testing.T) {
	ctx := context.Background()
	f := newClaudeFixture(t, supervisor.HostClaudeCode)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"second", "third"} {
		payload := createPayload("claude-" + name)
		effects, _ := payload.Obj.Get("effects")
		effects.Obj.Set("touchPaths", wire.Strings([]string{name + ".txt"}))
		if r := mutate(t, f.s.repo, envelope("create-"+name, "CREATE", "", "", payload)); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("create %+v", r)
		}
	}
	waiting := map[string]*store.Workflow{}
	for _, id := range []string{"done", "late", "stranded"} {
		w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), id, self, f.config, "")
		if err != nil {
			t.Fatalf("open %s: %v", id, err)
		}
		if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "WAITING" {
			t.Fatalf("%s implement: %v", id, err)
		}
		waiting[id] = w
	}
	if _, err = store.OpenWorkflow(ctx, f.s.repo, operator(), "idle", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
		t.Fatalf("idle open: %v", err)
	}
	// Rollback step 1: with the claude-code policy and its pin in force,
	// cancel every program; a drained program is cancelled after its drain.
	if err = waiting["done"].Cancel(); err != nil {
		t.Fatalf("cancel before switch: %v", err)
	}
	if err = waiting["late"].Drain(); err != nil {
		t.Fatalf("drain before switch: %v", err)
	}
	if err = waiting["late"].Cancel(); err != nil {
		t.Fatalf("cancel after drain: %v", err)
	}
	// The step this test leaves out: "stranded" is drained, not cancelled.
	if err = waiting["stranded"].Drain(); err != nil {
		t.Fatalf("drain stranded: %v", err)
	}
	stranded := waiting["stranded"].Attempt()

	// Step 2: remove the policy host and pin a distinct Codex runtime.
	codexExe := filepath.Join(f.scripts, "codex")
	codexRaw := multiScript(t, codexExe, "#!/bin/sh\ncat >/dev/null\nexit 1\n")
	codex := f.config
	codex.Host = ""
	codex.Executable = codexExe
	codex.ExecutableSHA256 = supervisor.Digest(codexRaw)
	f.setPolicy(t, codexExe, "")

	for _, id := range []string{"idle", "done", "late", "stranded"} {
		before := f.inventory(t)
		_, err = store.OpenWorkflow(ctx, f.s.repo, operator(), id, self, f.config, "")
		if wire.CodeOf(err) != wire.CodeCapabilityUnavailable {
			t.Fatalf("reopen %s under the Codex pin: want CAPABILITY_UNAVAILABLE, got %s %v", id, wire.CodeOf(err), err)
		}
		if f.inventory(t) != before {
			t.Fatalf("reopen %s changed the program or attempt inventory", id)
		}
	}
	if _, err = store.OpenWorkflow(ctx, f.s.repo, operator(), "done", self, codex, ""); err == nil || !strings.Contains(err.Error(), "program config differs") {
		t.Fatalf("edited config: %v", err)
	}

	// Step 3: a new Codex program claims a released ticket; the drained
	// program's ticket is still claimed.
	cw, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "codex", self, codex, "")
	if err != nil {
		t.Fatalf("new codex program: %v", err)
	}
	if a := cw.Attempt(); a == nil || a.TicketID == stranded.TicketID {
		t.Fatalf("new codex program claimed %+v", a)
	}

	// Recovery of the stranded program: re-pin its Claude Code runtime
	// (host still Codex); its live attempt reopens for cancel only.
	f.setPolicy(t, f.config.Executable, "")
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "stranded", self, f.config, "")
	if err != nil {
		t.Fatalf("reopen stranded: %v", err)
	}
	before := f.inventory(t)
	if _, err = w.RunRole(ctx, "implementer", ""); wire.CodeOf(err) != wire.CodeUnsupported {
		t.Fatalf("stage under switched host: want UNSUPPORTED, got %s %v", wire.CodeOf(err), err)
	}
	if f.inventory(t) != before {
		t.Fatal("refused stage changed the program or attempt inventory")
	}
	if err = w.Cancel(); err != nil {
		t.Fatalf("cancel stranded: %v", err)
	}
	if a := w.Attempt(); a == nil || a.Phase != "CANCELLED" {
		t.Fatalf("stranded attempt %+v", a)
	}
}

// TestCALV0074_SpawnedFailureIsNotNoExec proves a failure after the lane
// leader is spawned is never settled as NO_EXEC, even when Run returns an
// empty outcome class: a leader whose boot identity is forged is refused
// after the spawn, its output stays open past the drain, and the stage keeps
// the unproved cleanup (BLOCKED_RECOVERY) instead of a clean release.
func TestCALV0074_SpawnedFailureIsNotNoExec(t *testing.T) {
	ctx := context.Background()
	f := newClaudeFixture(t, supervisor.HostClaudeCode)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Setenv("CORVINT_TEST_LEADER_FAULT", "identity")
	a, err := w.RunRole(ctx, "implementer", "")
	if err == nil || wire.CodeOf(err) == wire.CodeCapabilityUnavailable {
		t.Fatalf("spawned failure: want a non-launch-refusal error, got %s %v", wire.CodeOf(err), err)
	}
	p := f.program(t, "program")
	if p.Phase != "BLOCKED_RECOVERY" || p.ResultClass == "NO_EXEC" || p.OwnerReleased || p.LeaderPID <= 0 {
		t.Fatalf("spawned failure settled as released: phase %s class %q quiescence %s released %v leader %d", p.Phase, p.ResultClass, p.Quiescence, p.OwnerReleased, p.LeaderPID)
	}
	if a == nil || a.Phase == "CANCELLED" || a.Phase == "WAITING" || a.Quiescence == "PROVED" {
		t.Fatalf("attempt released despite unproved cleanup: %+v", a)
	}
}
