package cemchain

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/cemcandidate"
	"github.com/Beamfall/corvint/internal/taskman"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

const (
	module  = "example.invalid/chain"
	project = module + "/src"
	// specLine names every changed path and the identifiers the source hunks
	// add, so each cited hunk is anchored.
	specLine  = "src/calc.go defines Add; src/calc_test.go holds TestPass, TestFail and TestSkip; artifacts/runner-plan.json and artifacts/runner-receipt.json prove Add.\n"
	calcDraft = "package src\n\n// Add returns the sum of a and b.\nfunc Add(a, b int) int { return b + a }\n"
	calcFinal = "package src\n\n// Add returns the sum of a and b.\nfunc Add(a, b int) int { return a + b }\n"
	calcTest  = "package src\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"Add(2, 3) != 5\")\n\t}\n}\n\nfunc TestFail(t *testing.T) { t.Fatal(\"failed-receipt control\") }\n\nfunc TestSkip(t *testing.T) { t.Skip(\"skipped-receipt control\") }\n"
)

var operator = mutation.Binding{ID: "tester", Role: "OWNER"}

func obj(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func str(s string) wire.Value { return wire.String(s) }

func stamp(t *testing.T) wire.Timestamp {
	t.Helper()
	ts, err := wire.ParseTimestamp("at", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func realTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// build compiles one main package of this module, or the separate
// interop/cem01-go module, into a private directory.
func build(t *testing.T, dir, pkg, name string) string {
	t.Helper()
	out := filepath.Join(realTemp(t), name)
	command := exec.Command("go", "build", "-o", out, pkg)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	if dir != "." && strings.HasSuffix(dir, "cem01-go") {
		command.Env = append(command.Env, "GOWORK=off")
	}
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, raw)
	}
	return out
}

type checkout struct {
	t    *testing.T
	root string
	home string
}

func (c checkout) git(args ...string) string {
	c.t.Helper()
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = c.root
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "HOME="+c.home, "XDG_CONFIG_HOME="+c.home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	out, err := command.CombinedOutput()
	if err != nil {
		c.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (c checkout) put(path, data string) {
	c.t.Helper()
	fixture.Write(c.t, filepath.Join(c.root, filepath.FromSlash(path)), []byte(data))
}

// tasks is a private disposable native Tasks store whose policy declares the
// production cem-verdict gate shape against the built binaries.
type tasks struct {
	repo *intent.Repository
	code string
}

func newTasks(t *testing.T, code, corvint, consumer string) *tasks {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Init(context.Background(), repo, operator, "req-init", stamp(t)); err != nil {
		t.Fatal(err)
	}
	argv := []string{corvint, "cem", "gate-verdict", "--base", store.GateArgBaseCommit, "--candidate", store.GateArgCandidateCommit,
		"--predicate", "touched-anchored", "--touch-paths", store.GateArgTouchPaths, "--portable-consumer", consumer}
	gate := obj("gateId", str("cem-verdict"), "kind", str("COMMAND"), "argv", wire.Strings(argv), "cwd", str("WORKTREE"),
		"env", wire.Strings([]string{"PATH"}), "timeoutSeconds", str("600"), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings([]string{"output"}), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(false), "required", wire.Bool(false))
	v := fixture.PolicyValue()
	v.Obj.Set("gates", wire.Array(gate))
	v.Obj.Set("environment", obj("allowedEnvKeys", wire.Strings([]string{"PATH"})))
	v.Obj.Set("policyVersion", str("2"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	request := store.PolicyRequest{QueueID: fixture.QueueID, RequestID: "policy-chain", ExpectedPolicyVersion: wire.Size("1"), Policy: wire.EncodeFile(v)}
	if report, err := store.PolicyUpdate(context.Background(), repo, operator, request, stamp(t)); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	return &tasks{repo: repo, code: code}
}

// ticket creates one native ticket whose declared touch path is src/.
func (s *tasks) ticket(t *testing.T) string {
	t.Helper()
	payload := obj(
		"acceptanceCriteria", wire.Strings([]string{"Add returns the sum and its runner receipt passes"}),
		"body", wire.Null(), "capabilities", wire.Strings(nil), "dependencies", wire.Array(), "dueDate", wire.Null(),
		"effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings([]string{"src/"})),
		"estimateMinutes", wire.Null(), "executionClass", str("AUTONOMOUS"), "kind", str("FEATURE"), "labels", wire.Strings(nil),
		"milestone", wire.Null(), "order", str("0"), "owner", wire.Null(), "priority", str("P2"), "requiredGates", wire.Strings(nil),
		"requirementRefs", wire.Strings(nil),
		"source", obj("kind", str("NATIVE"), "sourceItemId", wire.Null(), "sourceQueueId", str(fixture.QueueID), "sourceRevisionSha256", wire.Null()),
		"supersededBy", wire.Null(), "supersedes", wire.Null(), "title", str("chain"))
	envelope := wire.EncodeFile(obj("profile", str(mutation.Profile), "requestId", str("create-chain"), "actor", obj("id", str("tester"), "role", str("OWNER")),
		"queueId", str(fixture.QueueID), "targetId", wire.Null(), "expectedRevision", wire.Null(), "operation", str(mutation.OpCreate),
		"payload", payload, "issuedAt", str("2026-09-07T12:00:00Z")))
	report, err := store.Mutate(context.Background(), s.repo, operator, envelope, stamp(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v %v", report, err)
	}
	return report.Ticket
}

func (s *tasks) lease(t *testing.T, requestID string, l transaction.LeaseRequest) *store.Report {
	t.Helper()
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.code, Lease: l}
	report, err := store.Lease(context.Background(), s.repo, operator, choice, stamp(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("lease %s: %+v %v", requestID, report, err)
	}
	return report
}

func (s *tasks) gate(requestID string, claim *store.Report) (*store.Report, error) {
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.code,
		Lease: transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: claim.AttemptID, Generation: claim.Generation, Gate: "cem-verdict"}}
	return store.GateRun(context.Background(), s.repo, operator, choice, s.code, time.Now)
}

// result is the attempt's latest cem-verdict result and its output evidence.
func (s *tasks) result(t *testing.T, attemptID string) (*snapshot.GateResult, []byte) {
	t.Helper()
	evidence := func(digest string) []byte {
		raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", digest))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", attemptID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		t.Fatal(err)
	}
	var last *snapshot.GateResult
	for _, d := range a.GateResults {
		g, err := snapshot.DecodeGateResult(evidence(d))
		if err != nil {
			t.Fatal(err)
		}
		if g.GateID == "cem-verdict" {
			last = g
		}
	}
	if last == nil || len(last.Evidence) != 1 {
		t.Fatalf("no cem-verdict result with one output: %+v", last)
	}
	return last, evidence(string(last.Evidence[0].Sha256))
}

// verdictDoc is the part of the gate document this test reads.
type verdictDoc struct {
	Outcome string  `json:"outcome"`
	Code    *string `json:"code"`
	Reason  *string `json:"reason"`
	Verdict *struct {
		Outcome, Stage, Sidecar, VerdictSha256 string
		IssueCodes                             []string
	} `json:"verdict"`
	Portable *struct {
		Outcome, Stage, VerdictSha256 string
	} `json:"portable"`
	Paths []struct {
		Path      string
		Tier      string
		Satisfied bool
	} `json:"paths"`
}

func decodeVerdict(t *testing.T, raw []byte) verdictDoc {
	t.Helper()
	var d verdictDoc
	if err := json.Unmarshal(bytes.TrimSpace(raw), &d); err != nil {
		t.Fatalf("gate document: %v\n%s", err, raw)
	}
	return d
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// runner drives the trusted runner CLI: plan, then run with the plan's
// approval, under the independently pinned go executable.
type runner struct {
	t        *testing.T
	bin, dir string
	identity []string
}

func newRunner(t *testing.T, bin string) runner {
	t.Helper()
	goExe, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go unavailable")
	}
	if goExe, err = filepath.EvalSymlinks(goExe); err != nil {
		t.Fatal(err)
	}
	tool, err := os.ReadFile(goExe)
	if err != nil {
		t.Fatal(err)
	}
	return runner{t: t, bin: bin, dir: realTemp(t), identity: []string{"--executable", goExe, "--executable-sha256", tr.Digest(tool)}}
}

// run plans and executes one selected test of the checkout's src package over
// the current worktree bytes and returns the plan and receipt bytes.
func (r runner) run(name, root, test string, wantExit int) ([]byte, []byte) {
	r.t.Helper()
	inputs := map[string]string{}
	for _, path := range []string{"go.mod", "src/calc.go", "src/calc_test.go"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			r.t.Fatal(err)
		}
		inputs[path] = tr.Digest(raw)
	}
	selector := []string{project + "::" + test}
	request := tr.Request{Runner: "go-test", Root: root, Project: project, ReportDir: filepath.Join(r.dir, name+"-reports"), TimeoutSeconds: 300, InputFiles: inputs, ExpectedTests: selector, Selectors: selector}
	raw, err := json.Marshal(request)
	if err != nil {
		r.t.Fatal(err)
	}
	requestPath := filepath.Join(r.dir, name+"-request.json")
	if err := os.WriteFile(requestPath, raw, 0o600); err != nil {
		r.t.Fatal(err)
	}
	plan, err := exec.Command(r.bin, append([]string{"plan", "--request", requestPath}, r.identity...)...).Output()
	if err != nil {
		r.t.Fatalf("runner plan %s: %v", name, err)
	}
	var p tr.PlanDocument
	if err := tr.DecodeDocument(plan, &p); err != nil {
		r.t.Fatal(err)
	}
	planPath, receiptPath := filepath.Join(r.dir, name+"-plan.json"), filepath.Join(r.dir, name+"-receipt.json")
	if err := os.WriteFile(planPath, plan, 0o600); err != nil {
		r.t.Fatal(err)
	}
	command := exec.Command(r.bin, append([]string{"run", "--out", receiptPath, "--plan", planPath, "--approve", tr.Identity(p), "--experimental", "--trusted-local"}, r.identity...)...)
	out, _ := command.CombinedOutput()
	if command.ProcessState.ExitCode() != wantExit {
		r.t.Fatalf("runner run %s exit %d, want %d: %s", name, command.ProcessState.ExitCode(), wantExit, out)
	}
	receipt, err := os.ReadFile(receiptPath)
	if err != nil {
		r.t.Fatal(err)
	}
	return plan, receipt
}

// TestCEMV1286TasksRunnerProducerVerifyChain drives the bounded chain on one
// private disposable Tasks store and one fixture checkout: a native claim, the
// trusted runner's plan and receipt, the plain producer with runner
// admission, and the Tasks cem-verdict gate running the built corvint CLI,
// which recomputes the verdict natively and through the independently
// authored portable consumer built from interop/cem01-go. Failed, skipped,
// stale and tampered receipts are each refused by the producer with their
// registered codes before any bundle exists (CEM-V1-286); a runner artifact
// tampered after production is refused by the gate, by native and portable
// recomputation, and by Tasks itself (CEM-V1-287).
func TestCEMV1286TasksRunnerProducerVerifyChain(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the cem-verdict gate and stable process containment are only supported on Darwin/Linux")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	corvint := build(t, repoRoot, "./cmd/corvint", "corvint")
	runnerBin := build(t, repoRoot, "./cmd/corvint-test-runner", "corvint-test-runner")
	consumer := build(t, filepath.Join(repoRoot, "interop", "cem01-go"), ".", "cem01-go")
	run := newRunner(t, runnerBin)

	code := checkout{t: t, root: filepath.Join(realTemp(t), "code"), home: realTemp(t)}
	if err := os.Mkdir(code.root, 0o755); err != nil {
		t.Fatal(err)
	}
	code.git("init", "-q", "-b", "main")
	code.put("go.mod", "module "+module+"\n\ngo 1.27.1\n")
	code.put("docs/spec.md", "# Calc\n\n"+specLine)
	code.git("add", ".")
	code.git("commit", "-qm", "base")
	base := code.git("rev-parse", "HEAD")

	// Tasks: one native ticket claimed on the base; the attempt's base is it.
	s := newTasks(t, code.root, corvint, consumer)
	id := s.ticket(t)
	claim := s.lease(t, "claim-chain", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: id, Holder: "agent-1", LeaseMinutes: "60",
		Scope: []string{".corvint/changes/", "artifacts/", "src/"}})

	// Runner: a passing receipt over a draft that later changes is stale;
	// the final sources yield the passing, failed and skipped receipts.
	code.put("src/calc.go", calcDraft)
	code.put("src/calc_test.go", calcTest)
	stalePlan, staleReceipt := run.run("stale", code.root, "TestPass", 0)
	code.put("src/calc.go", calcFinal)
	passPlan, passReceipt := run.run("pass", code.root, "TestPass", 0)
	failPlan, failReceipt := run.run("fail", code.root, "TestFail", 1)
	skipPlan, skipReceipt := run.run("skip", code.root, "TestSkip", 0)
	code.put("artifacts/runner-plan.json", string(passPlan))
	code.put("artifacts/runner-receipt.json", string(passReceipt))
	code.git("add", ".")
	code.git("commit", "-qm", "calc: add Add with its runner receipt")
	target := code.git("rev-parse", "HEAD")

	// Producer map: every hunk cites the base spec line naming its path.
	session, err := workflow.Open(code.root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	prepared, err := os.ReadFile(filepath.Join(code.root, filepath.FromSlash(cw.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	draft, err := cw.ParseStable(prepared)
	if err != nil || len(draft.Change.Hunks) != 4 {
		t.Fatalf("prepared map: %v %+v", err, draft)
	}
	if session, err = workflow.Open(code.root); err != nil {
		t.Fatal(err)
	}
	var hunks []string
	for _, h := range draft.Change.Hunks {
		hunks = append(hunks, h.ID)
		if _, err := session.Cite(ctx, workflow.CiteOptions{MapPath: cw.ExcludedCEMPath, Hunk: h.ID, EvidencePath: "docs/spec.md", Lines: "3:3", Relation: "specification"}); err != nil {
			t.Fatalf("cite %s: %v", h.Path, err)
		}
	}
	sourceBytes, err := os.ReadFile(filepath.Join(code.root, filepath.FromSlash(cw.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	source, err := cw.ParseStable(sourceBytes)
	if err != nil || len(source.Change.Evidence) != 1 {
		t.Fatalf("source map: %v", err)
	}
	inputs := realTemp(t)
	ref := func(name string, raw []byte) cemcandidate.FileRef {
		t.Helper()
		path := filepath.Join(inputs, name)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return cemcandidate.FileRef{Path: path, Sha256: tr.Digest(raw)}
	}
	evidence := source.Change.Evidence[0].ID
	request := func(name string, plan, receipt []byte) cemcandidate.PlainRequest {
		return cemcandidate.PlainRequest{Profile: cemcandidate.PlainAssemblyProfile, Repository: code.root, ExpectedBase: base, Target: target,
			SourceMap: ref(name+"-source.json", sourceBytes), RunnerPlan: ref(name+"-plan.json", plan), RunnerReceipt: ref(name+"-receipt.json", receipt),
			Criteria:        []cemcandidate.PlainCriterion{{CriterionEvidenceID: evidence, HunkIDs: hunks, EvidenceIDs: []string{evidence}}},
			RunnerAdmission: cemcandidate.RunnerAdmissionProfile}
	}

	// Producer controls: each refused with its code, nothing written.
	for _, c := range []struct {
		name, code    string
		plan, receipt []byte
	}{
		{"failed", "runner-receipt-failed", failPlan, failReceipt},
		{"skipped", "runner-receipt-skipped", skipPlan, skipReceipt},
		{"stale", "runner-receipt-stale", stalePlan, staleReceipt},
		{"tampered", "runner-receipt-tampered", failPlan, passReceipt},
	} {
		out := filepath.Join(realTemp(t), "bundle")
		_, err := cemcandidate.AssemblePlain(ctx, request(c.name, c.plan, c.receipt), out)
		if got := cemcode.CodeOf(err); got != c.code {
			t.Fatalf("%s receipt: code %q (%v), want %q", c.name, got, err, c.code)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("%s receipt left a bundle: %v", c.name, err)
		}
	}
	// Without admission the producer keeps CEM-V1-240's opaque files: the
	// failed receipt is then accepted, so the refusals above are admission's.
	opaque := request("opaque", failPlan, failReceipt)
	opaque.RunnerAdmission = ""
	if result, err := cemcandidate.AssemblePlain(ctx, opaque, filepath.Join(realTemp(t), "bundle")); err != nil || result.Verification.Outcome != "ACCEPT" {
		t.Fatalf("opaque failed receipt: %+v %v", result.Verification, err)
	}

	bundle := filepath.Join(realTemp(t), "bundle")
	produced, err := cemcandidate.AssemblePlain(ctx, request("pass", passPlan, passReceipt), bundle)
	if err != nil || produced.Verification.Outcome != "ACCEPT" {
		t.Fatalf("admitted receipt: %+v %v", produced.Verification, err)
	}
	stable, err := os.ReadFile(produced.StablePath)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{"runner-plan.json": passPlan, "runner-receipt.json": passReceipt} {
		if got, err := os.ReadFile(filepath.Join(bundle, "artifacts", name)); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("bundle %s is not the runner's bytes: %v", name, err)
		}
	}

	// Bind and seal, then submit the sealed tree to the attempt.
	if err := os.WriteFile(filepath.Join(code.root, filepath.FromSlash(cw.ExcludedCEMPath)), stable, 0o644); err != nil {
		t.Fatal(err)
	}
	code.git("add", "-f", cw.ExcludedCEMPath)
	code.git("commit", "-qm", "chore: bind change evidence")
	bind := code.git("rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(code.root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	code.git("mv", cw.ExcludedCEMPath, ".corvint/changes/"+bind+".cem.json")
	code.git("commit", "-qm", "chore: seal change evidence")
	seal := code.git("rev-parse", "HEAD")
	s.lease(t, "submit-seal", transaction.LeaseRequest{Verb: transaction.LeaseSubmit, AttemptID: claim.AttemptID, Generation: claim.Generation, Tree: code.git("rev-parse", "HEAD^{tree}")})

	// Tasks gate: GRANT with native and portable ACCEPT and src/ anchored.
	if report, err := s.gate("gate-seal", claim); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("gate: %+v %v", report, err)
	}
	g, out := s.result(t, claim.AttemptID)
	doc := decodeVerdict(t, out)
	if g.State != "PASSED" || doc.Outcome != "GRANT" || doc.Verdict == nil || doc.Verdict.Outcome != "ACCEPT" || doc.Verdict.Sidecar != "EXACT" ||
		doc.Portable == nil || doc.Portable.Outcome != "ACCEPT" || doc.Portable.VerdictSha256 != doc.Verdict.VerdictSha256 || len(doc.Paths) == 0 {
		t.Fatalf("sealed chain not granted: state %s\n%s", g.State, out)
	}
	for _, p := range doc.Paths {
		if !p.Satisfied {
			t.Fatalf("governed path %s at tier %s is unsatisfied", p.Path, p.Tier)
		}
	}

	// Post-production tamper in the worktree: Tasks refuses the dirty
	// checkout, and the gate's native and portable recomputations both
	// reject the artifact digest, so the verdict is not granted.
	code.put("artifacts/runner-receipt.json", strings.Replace(string(passReceipt), `"state":"PASSED"`, `"state":"PASSED" `, 1))
	if _, err := s.gate("gate-dirty", claim); wire.CodeOf(err) != wire.CodeDirtyWorktree {
		t.Fatalf("dirty worktree gate: %v", err)
	}
	direct, err := taskman.EvaluateCEMVerdictGate(ctx, taskman.CEMVerdictInput{Root: code.root, BaseCommit: base, CandidateCommit: seal,
		TouchPaths: []string{"src/"}, Predicate: taskman.PredicateTouchedAnchored, PortableConsumer: consumer})
	if err != nil {
		t.Fatal(err)
	}
	tampered := decodeVerdict(t, direct.Document)
	if direct.Granted || direct.Code != taskman.CEMVerdictUnverifiable || tampered.Verdict == nil || tampered.Verdict.Outcome != "REJECT" ||
		tampered.Verdict.Stage != "artifacts" || !slices.Contains(tampered.Verdict.IssueCodes, "artifact-digest-mismatch") ||
		tampered.Portable == nil || tampered.Portable.Outcome != "REJECT" || tampered.Portable.Stage != "artifacts" ||
		tampered.Portable.VerdictSha256 != tampered.Verdict.VerdictSha256 {
		t.Fatalf("tampered artifact not refused by both verifiers: %s", direct.Document)
	}
	if r, status := verify.Stable(ctx, stable, verify.StableOptions{Repository: code.root, ExpectedBase: base, Target: bind, ArtifactRoot: code.root}); status == 0 || r.Outcome != "REJECT" {
		t.Fatalf("native verify accepted the tampered artifact: %+v", r)
	}

	// Committed tamper: the candidate is no longer the sealed target with
	// only its sidecar moved, so the Tasks gate fails.
	code.git("commit", "-qam", "tamper with the runner receipt")
	s.lease(t, "submit-tamper", transaction.LeaseRequest{Verb: transaction.LeaseSubmit, AttemptID: claim.AttemptID, Generation: claim.Generation, Tree: code.git("rev-parse", "HEAD^{tree}")})
	if report, err := s.gate("gate-tamper", claim); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("tamper gate: %+v %v", report, err)
	}
	g, out = s.result(t, claim.AttemptID)
	doc = decodeVerdict(t, out)
	if g.State != "FAILED" || doc.Outcome != "REFUSE" || deref(doc.Code) != taskman.CEMVerdictProducerMismatch || deref(doc.Reason) != "sealed-tree-mismatch" {
		t.Fatalf("committed tamper not refused: state %s\n%s", g.State, out)
	}
}
