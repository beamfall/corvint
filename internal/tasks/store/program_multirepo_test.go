//go:build darwin || linux

package store_test

import (
	"context"
	"os"
	"os/exec"
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

func multiGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func multiCommitted(t *testing.T, dir, file, body string) {
	t.Helper()
	multiGit(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	multiGit(t, dir, "add", file)
	multiGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
}

func multiScript(t *testing.T, path, body string) []byte {
	t.Helper()
	raw := []byte(body)
	if err := os.WriteFile(path, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return raw
}

type multiFixture struct {
	s        *leaseStore
	ticketID string
	extra    string
	scripts  string
	config   store.ProgramConfig
}

// newMultiFixture is a real Git queue repository plus one extra repository
// "docs", a pinned fake Codex host and a pinned fake Core CLI. The host edits
// hello.txt in the queue worktree and note.txt in the docs sibling worktree,
// then (in a read-only stage) accepts every claim.
func newMultiFixture(t *testing.T, touch ...string) *multiFixture {
	t.Helper()
	return buildProgramFixture(t, false, true, nil, touch...)
}

// buildProgramFixture is newMultiFixture; keepGates retains the fixture
// policy's required `verify` gate, and without multi it declares no
// extra repository and the fake host edits only the queue worktree. A
// non-nil gates replaces the policy's gates with one required gate per id,
// each a command that exits 0. While the file "escape" exists in scripts,
// the implement stage leaves a setsid process holding the host output past
// the drain, so the stage stops without proved quiescence; while
// "review-escape" exists, the review stage does the same after editing the
// candidate worktree; while "slow" exists, the implement stage runs for three
// seconds; while "docs-unchanged" exists, it leaves the docs repository
// unchanged; while "no-docs-context" exists, the fake Core refuses a query
// from a docs worktree. The implement stage records its argv in
// "implement-args" and its prompt in "implement-prompt".
func buildProgramFixture(t *testing.T, keepGates, multi bool, gates []string, touch ...string) *multiFixture {
	t.Helper()
	s := newLeaseStore(t)
	payload := createPayload("multi")
	effects, _ := payload.Obj.Get("effects")
	if len(touch) == 0 {
		touch = []string{"@docs/", "hello.txt"}
		if !multi {
			touch = []string{"hello.txt"}
		}
	}
	effects.Obj.Set("touchPaths", wire.Strings(touch))
	report := mutate(t, s.repo, envelope("create-multi", "CREATE", "", "", payload))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %+v", report)
	}
	multiCommitted(t, s.repo.PrimaryWorktree, "hello.txt", "base\n")
	scripts := fixture.TempDirOutside(t)
	extra, docsEdit := "", ""
	if multi {
		extra = filepath.Join(fixture.TempDirOutside(t), "docs")
		if err := os.Mkdir(extra, 0o755); err != nil {
			t.Fatal(err)
		}
		multiCommitted(t, extra, "note.txt", "base\n")
		docsEdit = "  if [ ! -f \"" + scripts + "/docs-unchanged\" ]; then printf 'changed\\n' > \"$PWD@docs/note.txt\"; fi\n"
	}

	claim := string(wire.Sum([]byte("it exists")))
	codex := filepath.Join(scripts, "codex")
	codexRaw := multiScript(t, codex, `#!/bin/sh
input="$(cat)"
case "$input" in
*"Read-only integration verification"*)
  echo '{"type":"thread.started","thread_id":"integrate-session"}'
  echo '{"type":"turn.started"}'
  echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"HANDOFF\",\"summary\":\"verified\",\"nextAction\":\"integrate\"}"}}'
  echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
  exit 0
  ;;
esac
case " $* " in
*" workspace-write "*)
  printf '%s\n' "$*" > "`+scripts+`/implement-args"
  printf '%s\n' "$input" > "`+scripts+`/implement-prompt"
  if [ -f "`+scripts+`/escape" ]; then perl -e 'use POSIX; POSIX::setsid(); sleep 5' & fi
  if [ -f "`+scripts+`/slow" ]; then sleep 3; fi
  printf 'changed\n' > hello.txt
`+docsEdit+`  echo '{"type":"thread.started","thread_id":"implement-session"}'
  echo '{"type":"turn.started"}'
  echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"summary\":\"edited both repositories\",\"nextAction\":\"review\"}"}}'
  ;;
*)
  printf '%s\n' "$*" > "`+scripts+`/review-args"
  if [ -f "`+scripts+`/review-escape" ]; then perl -e 'use POSIX; POSIX::setsid(); sleep 5' & printf 'reviewed\n' > hello.txt; fi
  echo '{"type":"thread.started","thread_id":"review-session"}'
  echo '{"type":"turn.started"}'
  echo '{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"REVIEW\",\"accepted\":true,\"claims\":[\"`+claim+`\"],\"summary\":\"verified\",\"nextAction\":\"integrate\"}"}}'
  ;;
esac
echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
`)
	core := filepath.Join(scripts, "core")
	coreRaw := multiScript(t, core, "#!/bin/sh\ncase \"$PWD\" in *@docs) if [ -f \""+scripts+"/no-docs-context\" ]; then exit 3; fi;; esac\nprintf '{\"ok\":true,\"context\":{\"state\":\"READY\",\"revision\":\"%s\",\"freshness\":{\"state\":\"fresh\"}}}\\n' \"$(git rev-parse HEAD^{tree})\"\n")

	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	if !keepGates {
		v.Obj.Set("gates", wire.Array())
	}
	if gates != nil {
		base, _ := v.Obj.Get("gates")
		list := []wire.Value{}
		for _, id := range gates {
			g := wire.ObjectValue(wire.NewObject())
			for _, k := range base.Arr[0].Obj.Keys {
				member, _ := base.Arr[0].Obj.Get(k)
				g.Obj.Set(k, member)
			}
			g.Obj.Set("gateId", str(id))
			g.Obj.Set("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}))
			list = append(list, g)
		}
		v.Obj.Set("gates", wire.Array(list...))
	}
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(string(wire.Sum([]byte(codex)))), "fileSha256", str(string(wire.Sum(codexRaw))), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER", "REVIEWER", "VERIFIER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	supervision := obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("1"),
		"program", obj("turns", str("8"), "wallClockMinutes", str("600"), "inputTokens", str("0"), "outputTokens", str("0")))
	if multi {
		supervision.Obj.Set("repositories", obj("docs", obj("pathSha256", str(string(wire.Sum([]byte(extra)))))))
	}
	v.Obj.Set("supervision", supervision)
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("multi-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	c := store.ProgramConfig{Profile: snapshot.SupervisedProfile, Executable: codex, ExecutableSHA256: supervisor.Digest(codexRaw), Model: "pinned-model", Effort: "low", WorkRoot: fixture.TempDirOutside(t), WallSeconds: 600, CoreExecutable: core, CoreSHA256: supervisor.Digest(coreRaw)}
	if multi {
		c.Repositories = []store.ProgramRepository{{Name: "docs", Checkout: extra}}
	}
	return &multiFixture{s: s, ticketID: report.Ticket, extra: extra, scripts: scripts, config: c}
}

// TestCALV0071_MultiRepositoryProgramFakeHost drives one supervised program
// over the queue repository and one declared extra repository through the
// pinned fake host: implement edits both, the candidate binds a composite
// tree whose gitlink names the extra repository's preserved commit, review
// binds to that composite, and integration of the changed extra repository
// without an integration designation is refused UNSUPPORTED before a grant is
// recorded or any checkout moves (CAL-V0-087).
func TestCALV0071_MultiRepositoryProgramFakeHost(t *testing.T) {
	t.Parallel()
	f := newMultiFixture(t)
	f.config.OwnIntegrationCheckout = true
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	extraBase := multiGit(t, f.extra, "rev-parse", "HEAD")
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil {
		t.Fatalf("implement: %v", err)
	}
	if a.Phase != "BUILT" || a.CandidateTreeOid == nil {
		t.Fatalf("implement phase %s", a.Phase)
	}
	args, err := os.ReadFile(filepath.Join(f.scripts, "implement-args"))
	if err != nil || !strings.Contains(string(args), `sandbox_workspace_write.writable_roots=["`) || !strings.Contains(string(args), `@docs"]`) {
		t.Fatalf("implement argv lacks the extra writable root: %s %v", args, err)
	}
	refs := multiGit(t, f.extra, "for-each-ref", "--format=%(objectname)", "refs/corvint/tasks/")
	if refs == "" || strings.Contains(refs, "\n") {
		t.Fatalf("extra repository candidate refs %q", refs)
	}
	if parent := multiGit(t, f.extra, "rev-parse", refs+"^"); parent != extraBase {
		t.Fatalf("extra candidate parent %s, want base %s", parent, extraBase)
	}
	if body := multiGit(t, f.extra, "show", refs+":note.txt"); body != "changed" {
		t.Fatalf("extra candidate note.txt %q", body)
	}
	if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != extraBase {
		t.Fatal("extra checkout HEAD moved")
	}
	listing := multiGit(t, f.s.repo.PrimaryWorktree, "ls-tree", *a.CandidateTreeOid)
	if !strings.Contains(listing, "160000 commit "+refs+"\tdocs") || !strings.Contains(listing, "040000 tree ") || !strings.Contains(listing, "\t.queue") {
		t.Fatalf("composite tree %s", listing)
	}
	if body := multiGit(t, f.s.repo.PrimaryWorktree, "show", *a.CandidateTreeOid+":.queue/hello.txt"); body != "changed" {
		t.Fatalf("composite queue hello.txt %q", body)
	}

	a, err = w.RunRole(ctx, "reviewer", "")
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if a.Phase != "READY_FOR_INTEGRATION" || a.Supervision.ReviewTree != *a.CandidateTreeOid {
		t.Fatalf("review phase %s tree %s", a.Phase, a.Supervision.ReviewTree)
	}
	if args, err = os.ReadFile(filepath.Join(f.scripts, "review-args")); err != nil || strings.Contains(string(args), "writable_roots") {
		t.Fatalf("review argv widened writable roots: %s %v", args, err)
	}
	queueBase := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD")
	if _, err = w.RunRole(ctx, "integrator", "grant"); wire.CodeOf(err) != wire.CodeUnsupported || !strings.Contains(err.Error(), "repository docs changed but has no integration designation") {
		t.Fatalf("undesignated extra repository integration did not fail closed: %v", err)
	}
	if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != extraBase {
		t.Fatal("extra checkout HEAD moved after integration refusal")
	}
	if head := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD"); head != queueBase {
		t.Fatal("queue checkout HEAD moved after integration refusal")
	}
	if stored := f.s.attempt(t, a.AttemptID); stored.Supervision.IntegrationGrant != "" || len(stored.PendingEffects) != 0 {
		t.Fatalf("refusal recorded grant %q effects %v", stored.Supervision.IntegrationGrant, stored.PendingEffects)
	}
}

// TestCALV0071_UndeclaredRepositoryRefusedBeforeMutation proves a program
// naming an extra repository the policy does not pin, or a checkout whose
// path differs from the pin, is refused before any program record or
// worktree exists.
func TestCALV0071_UndeclaredRepositoryRefusedBeforeMutation(t *testing.T) {
	t.Parallel()
	f := newMultiFixture(t)
	programs := filepath.Join(f.s.repo.StateDir, "programs.json")
	before, _ := os.ReadFile(programs)
	for want, repos := range map[string][]store.ProgramRepository{
		"is not declared by policy":        {{Name: "site", Checkout: f.extra}},
		"differs from the policy path pin": {{Name: "docs", Checkout: f.scripts}},
	} {
		name := want
		c := f.config
		c.Repositories = repos
		_, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", "/unused-self", c, f.ticketID)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("want refusal %q, got %v", want, err)
		}
		after, _ := os.ReadFile(programs)
		if string(after) != string(before) {
			t.Fatalf("%s refusal changed the program inventory", name)
		}
		if entries, _ := os.ReadDir(c.WorkRoot); len(entries) != 0 {
			t.Fatalf("%s refusal created work", name)
		}
	}
}

// TestCALV0071_ExtraRepositoryPathsAreScoped proves an extra repository's
// edits are scope-checked under "@name/": a ticket that declares only the
// queue repository path blocks a candidate that also edits docs.
func TestCALV0071_ExtraRepositoryPathsAreScoped(t *testing.T) {
	t.Parallel()
	f := newMultiFixture(t, "hello.txt")
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
	if a.Phase != "BLOCKED_RECOVERY" || a.ScopeCheck != "OUT_OF_SCOPE" || a.CandidateTreeOid != nil {
		t.Fatalf("out-of-scope extra edit reached phase %s scope %s", a.Phase, a.ScopeCheck)
	}
}

// TestCALV0072_MultiRepositoryGatesFailClosed proves required gates of a
// multi-repository program run against, and bind, the whole composite
// candidate: an extra repository sibling worktree that is dirtied before its
// gate runs refuses the gate, records no result and never reaches
// READY_FOR_INTEGRATION (CAL-V0-072). The passing composite gate is proved by
// TestCALV0087_DesignatedMultiRepositoryIntegration.
func TestCALV0072_MultiRepositoryGatesFailClosed(t *testing.T) {
	t.Parallel()
	f := buildProgramFixture(t, true, true, []string{"verify"})
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %v", err)
	}
	restore := store.SetRunFaultForTest(f.s.repo.StateDir, func(point string) error {
		if point == "gate:verify" {
			return os.WriteFile(*w.Attempt().WorktreePath+"@docs/stray.txt", []byte("stray\n"), 0o644)
		}
		return nil
	})
	a, err := w.RunRole(ctx, "reviewer", "")
	restore()
	if wire.CodeOf(err) != wire.CodeDirtyWorktree {
		t.Fatalf("dirty extra repository worktree did not refuse its gate: %v", err)
	}
	if a.Phase == "READY_FOR_INTEGRATION" || len(a.GateResults) != 0 {
		t.Fatalf("refused program reached phase %s with %d gate results", a.Phase, len(a.GateResults))
	}
	if stored := f.s.attempt(t, a.AttemptID); stored.Phase == "READY_FOR_INTEGRATION" || len(stored.GateResults) != 0 {
		t.Fatalf("stored attempt reached phase %s with %d gate results", stored.Phase, len(stored.GateResults))
	}
}

// TestCALV0071_RetargetedCheckoutRefusedBeforeWrite proves a declared
// checkout that is re-cloned, or retargeted by a symlink, after admission
// keeps its path pin but is refused before Git registers a worktree in, or
// writes a ref into, the wrong repository (CAL-V0-071).
func TestCALV0071_RetargetedCheckoutRefusedBeforeWrite(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"reclone", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			f := newMultiFixture(t)
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			moved := f.extra + "-original"
			if err = os.Rename(f.extra, moved); err != nil {
				t.Fatal(err)
			}
			other := f.extra
			if mode == "symlink" {
				other = filepath.Join(fixture.TempDirOutside(t), "other")
			}
			multiGit(t, filepath.Dir(other), "clone", "-q", moved, other)
			if mode == "symlink" {
				if err = os.Symlink(other, f.extra); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = w.RunRole(ctx, "implementer", ""); err == nil || !strings.Contains(err.Error(), "checkout identity differs") {
				t.Fatalf("retargeted checkout not refused: %v", err)
			}
			for _, repo := range []string{moved, other} {
				if list := multiGit(t, repo, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
					t.Fatalf("%s gained a worktree registration:\n%s", repo, list)
				}
				if refs := multiGit(t, repo, "for-each-ref", "refs/corvint/"); refs != "" {
					t.Fatalf("%s gained candidate refs %q", repo, refs)
				}
			}
		})
	}
}
