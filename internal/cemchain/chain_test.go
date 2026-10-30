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
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/cemcandidate"
	"github.com/Beamfall/corvint/internal/taskman"
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

func obj(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func str(s string) wire.Value { return wire.String(s) }

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
	write(c.t, filepath.Join(c.root, filepath.FromSlash(path)), []byte(data))
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// queueID is the private store's queue; tickets are minted under it.
const queueID = "queue:acme:main"

// queueValue is a native taskman-queue/0 manifest for the private store.
func queueValue() wire.Value {
	return obj("profile", str("taskman-queue/0"), "queueId", str(queueID), "repositoryAuthorityId", str("repo:acme"), "prefix", str("AT"),
		"nextSerial", str("2"), "schemaVersion", str("0"), "canonicalWriter", str("NATIVE"), "foreignAdapterId", wire.Null(),
		"intentBranch", str("main"), "fixture", wire.Bool(true), "executionCutover", wire.Null(), "importMapSha256", wire.Null(),
		"writeBarrier", obj("reason", str("NONE"), "since", wire.Null()))
}

// policyValue is the store's initial taskman-policy/0 (version 1) with one
// required COMMAND gate `verify`; newTasks replaces it through the CLI.
func policyValue() wire.Value {
	gate := obj("gateId", str("verify"), "kind", str("COMMAND"), "argv", wire.Strings([]string{"make", "verify"}), "cwd", str("WORKTREE"),
		"env", wire.Strings(nil), "timeoutSeconds", str("1800"), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings(nil), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(true), "required", wire.Bool(true))
	return obj("profile", str("taskman-policy/0"), "policyVersion", str("1"), "roles", wire.ObjectValue(wire.NewObject()),
		"capacity", obj("maxActiveAttempts", str("1"), "maxWorkersTotal", str("4"), "classes", wire.Array()),
		"budgets", obj(
			"lane", obj("inputTokens", str("2000000"), "cacheCreationTokens", str("1000000"), "cacheReadTokens", str("100000000"),
				"outputTokens", str("400000"), "turns", str("400"), "wallClockMinutes", str("90")),
			"ticketMultiplier", str("4"), "requireEnforcedFields", wire.Strings([]string{"turns", "wallClockMinutes"})),
		"retries", obj("admissionsPerRevision", str("3"), "repairRounds", str("2"), "malformedReviewRetry", str("1"),
			"gateRerunOnStale", str("1"), "reconcileAttempts", str("3")),
		"retention", obj("evidenceDays", str("90")), "gates", wire.Array(gate), "serialFallback", str("BLOCK"),
		"integrationRequiredKinds", wire.Strings(nil), "allowEmptyObligationsKinds", wire.Strings([]string{"CHORE"}),
		"reviewLane", obj("required", wire.Bool(true)), "docsLane", obj("required", wire.Bool(false)),
		"cemRequired", wire.Bool(false), "ocmRequired", wire.Bool(false), "runtimes", wire.Array(),
		"environment", obj("allowedEnvKeys", wire.Strings(nil)))
}

// tasks drives a private disposable native Tasks store only through the
// built corvint-tasks CLI: Core imports no Tasks package but wire. The store
// is initialized in the fixture checkout, which is on the intent branch, and
// its intent directory is excluded from Git there, so the checkout's commits,
// diffs and worktree status never contain it. Its policy declares the
// production cem-verdict gate shape against the built binaries.
type tasks struct {
	t         *testing.T
	bin, root string
	stateDir  string
}

// envelope is the part of a taskman-command-result/0 this test reads.
type envelope struct {
	Outcome  string            `json:"outcome"`
	Codes    []string          `json:"codes"`
	Items    []json.RawMessage `json:"items"`
	Warnings []string          `json:"warnings"`
}

// item is the part of a mutation or lease result this test reads.
type item struct {
	Outcome    string   `json:"outcome"`
	TicketID   string   `json:"ticketId"`
	AttemptID  string   `json:"attemptId"`
	Generation string   `json:"generation"`
	StateDir   string   `json:"stateDir"`
	Results    []string `json:"gateResults"`
}

// run executes one corvint-tasks command in the checkout and decodes its
// envelope.
func (s *tasks) run(args ...string) (envelope, []byte) {
	s.t.Helper()
	command := exec.Command(s.bin, args...)
	command.Dir = s.root
	command.Env = append(os.Environ(), "CORVINT_TASKS_ACTOR=tester", "ATM_ACTOR=tester")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	var e envelope
	if err := json.Unmarshal(stdout.Bytes(), &e); err != nil {
		s.t.Fatalf("corvint-tasks %v: %v (%v)\n%s\n%s", args, err, runErr, stdout.Bytes(), stderr.Bytes())
	}
	return e, stdout.Bytes()
}

// item decodes the envelope's single item.
func (s *tasks) item(e envelope, raw []byte) item {
	s.t.Helper()
	var it item
	if len(e.Items) != 1 || json.Unmarshal(e.Items[0], &it) != nil {
		s.t.Fatalf("not one result item: %s", raw)
	}
	return it
}

// mutate runs one write command and requires an OK envelope whose item
// committed (outcome COMPLETED), as the in-process store reported it.
func (s *tasks) mutate(args ...string) item {
	s.t.Helper()
	e, raw := s.run(append(args, "--role", "OWNER")...)
	it := s.item(e, raw)
	if e.Outcome != "OK" || it.Outcome != "COMPLETED" {
		s.t.Fatalf("corvint-tasks %v not completed: %s", args, raw)
	}
	return it
}

func newTasks(t *testing.T, bin, root, corvint, consumer string) *tasks {
	t.Helper()
	s := &tasks{t: t, bin: bin, root: root}
	exclude, err := os.OpenFile(filepath.Join(root, ".git", "info", "exclude"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, err = exclude.WriteString("/.taskman/\n")
		if closeErr := exclude.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ".taskman", "queue.json"), wire.EncodeFile(queueValue()))
	write(t, filepath.Join(root, ".taskman", "policy.json"), wire.EncodeFile(policyValue()))
	e, raw := s.run("init", "--role", "OWNER", "--request-id", "req-init")
	if s.stateDir = s.item(e, raw).StateDir; e.Outcome != "OK" || s.stateDir == "" {
		t.Fatalf("init: %s", raw)
	}
	argv := []string{corvint, "cem", "gate-verdict", "--base", "{attempt.baseCommit}", "--candidate", "{attempt.candidateCommit}",
		"--predicate", "touched-anchored", "--touch-paths", "{claim.touchPaths}", "--portable-consumer", consumer}
	gate := obj("gateId", str("cem-verdict"), "kind", str("COMMAND"), "argv", wire.Strings(argv), "cwd", str("WORKTREE"),
		"env", wire.Strings([]string{"PATH"}), "timeoutSeconds", str("600"), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings([]string{"output"}), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(false), "required", wire.Bool(false))
	v := policyValue()
	v.Obj.Set("gates", wire.Array(gate))
	v.Obj.Set("environment", obj("allowedEnvKeys", wire.Strings([]string{"PATH"})))
	v.Obj.Set("policyVersion", str("2"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	path := filepath.Join(realTemp(t), "policy-2.json")
	write(t, path, wire.EncodeFile(v))
	s.mutate("policy", "update", "--request-id", "policy-chain", "--expected-policy-version", "1", "--file", path)
	return s
}

// ticket creates one native ticket whose declared touch paths cover the
// claimed scope; a claim cannot widen past them (ATR-V0-210). It requires the
// cem-verdict gate, so completion needs that gate's current PASSED result.
func (s *tasks) ticket() string {
	s.t.Helper()
	payload := obj(
		"acceptanceCriteria", wire.Strings([]string{"Add returns the sum and its runner receipt passes"}),
		"body", wire.Null(), "capabilities", wire.Strings(nil), "dependencies", wire.Array(), "dueDate", wire.Null(),
		"effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings([]string{".corvint/changes/", "artifacts/", "src/"})),
		"estimateMinutes", wire.Null(), "executionClass", str("AUTONOMOUS"), "kind", str("FEATURE"), "labels", wire.Strings(nil),
		"milestone", wire.Null(), "order", str("0"), "owner", wire.Null(), "priority", str("P2"), "requiredGates", wire.Strings([]string{"cem-verdict"}),
		"requirementRefs", wire.Strings(nil),
		"source", obj("kind", str("NATIVE"), "sourceItemId", wire.Null(), "sourceQueueId", str(queueID), "sourceRevisionSha256", wire.Null()),
		"supersededBy", wire.Null(), "supersedes", wire.Null(), "title", str("chain"))
	created := s.mutate("ticket", "create", "--request-id", "create-chain", "--issued-at", "2026-09-07T12:00:00Z",
		"--payload", string(bytes.TrimSpace(wire.EncodeFile(payload))))
	return created.TicketID
}

// claim claims the ticket on the checkout's HEAD, the base.
func (s *tasks) claim(ticketID string, scope ...string) item {
	s.t.Helper()
	args := append([]string{"claim", ticketID, "--holder", "agent-1", "--request-id", "claim-chain", "--lease-minutes", "60", "--scope"}, scope...)
	return s.mutate(args...)
}

// submit records a tree as the attempt's candidate.
func (s *tasks) submit(requestID string, claim item, tree string) {
	s.t.Helper()
	s.mutate("submit", "--attempt", claim.AttemptID, "--generation", claim.Generation, "--request-id", requestID, "--tree", tree)
}

// gate runs the attempt's cem-verdict gate in the checkout.
func (s *tasks) gate(requestID string, claim item) (envelope, []byte) {
	s.t.Helper()
	return s.run("gate", "run", "--attempt", claim.AttemptID, "--generation", claim.Generation, "--request-id", requestID,
		"--gate", "cem-verdict", "--worktree", s.root, "--role", "OWNER")
}

// complete asks Tasks to complete the attempt at an intent-branch commit.
func (s *tasks) complete(requestID string, claim item, commit string) (envelope, []byte) {
	s.t.Helper()
	return s.run("complete", "--attempt", claim.AttemptID, "--generation", claim.Generation, "--request-id", requestID,
		"--commit", commit, "--role", "OWNER")
}

// refused requires a Tasks command to have been refused with code.
func refused(t *testing.T, step string, e envelope, raw []byte, code string) {
	t.Helper()
	if e.Outcome == "OK" || !slices.Contains(e.Codes, code) {
		t.Fatalf("%s: want refusal %s, got %s", step, code, raw)
	}
}

// gateRan requires a gate run to have committed its result.
func (s *tasks) gateRan(requestID string, claim item) {
	s.t.Helper()
	e, raw := s.gate(requestID, claim)
	if it := s.item(e, raw); e.Outcome != "OK" || it.Outcome != "COMPLETED" {
		s.t.Fatalf("gate %s: %s", requestID, raw)
	}
}

// gateResult is the part of a taskman-gate-result/0 this test reads.
type gateResult struct {
	GateID   string `json:"gateId"`
	State    string `json:"state"`
	Evidence []struct {
		Sha256 string `json:"sha256"`
	} `json:"evidence"`
}

// result is the attempt's latest cem-verdict result and its output evidence:
// `attempt show` lists the result digests, read from the store's
// content-addressed evidence directory.
func (s *tasks) result(attemptID string) (*gateResult, []byte) {
	s.t.Helper()
	evidence := func(digest string) []byte {
		raw, err := os.ReadFile(filepath.Join(s.stateDir, "evidence", digest))
		if err != nil {
			s.t.Fatal(err)
		}
		return raw
	}
	e, raw := s.run("attempt", "show", attemptID)
	a := s.item(e, raw)
	if e.Outcome != "OK" || a.AttemptID != attemptID {
		s.t.Fatalf("attempt show: %s", raw)
	}
	var last *gateResult
	for _, d := range a.Results {
		var g gateResult
		if err := json.Unmarshal(evidence(d), &g); err != nil {
			s.t.Fatal(err)
		}
		if g.GateID == "cem-verdict" {
			last = &g
		}
	}
	if last == nil || len(last.Evidence) != 1 {
		s.t.Fatalf("no cem-verdict result with one output: %+v", last)
	}
	return last, evidence(last.Evidence[0].Sha256)
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

// run plans and executes the selected tests of the checkout's src package over
// the current worktree bytes and returns the plan and receipt bytes.
func (r runner) run(name, root string, wantExit int, tests ...string) ([]byte, []byte) {
	r.t.Helper()
	inputs := map[string]string{}
	for _, path := range []string{"go.mod", "src/calc.go", "src/calc_test.go"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			r.t.Fatal(err)
		}
		inputs[path] = tr.Digest(raw)
	}
	var selector []string
	for _, test := range tests {
		selector = append(selector, project+"::"+test)
	}
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

// newCheckout creates the fixture checkout on main whose base commit holds
// go.mod and the one-line spec every hunk cites.
func newCheckout(t *testing.T, spec string) (checkout, string) {
	t.Helper()
	code := checkout{t: t, root: filepath.Join(realTemp(t), "code"), home: realTemp(t)}
	if err := os.Mkdir(code.root, 0o755); err != nil {
		t.Fatal(err)
	}
	code.git("init", "-q", "-b", "main")
	code.put("go.mod", "module "+module+"\n\ngo 1.27.1\n")
	code.put("docs/spec.md", "# Calc\n\n"+spec)
	code.git("add", ".")
	code.git("commit", "-qm", "base")
	return code, code.git("rev-parse", "HEAD")
}

// plainRequests prepares the base..target producer map, cites the base spec
// line from each of its wantHunks hunks, and returns a constructor of
// admitted plain-assembly requests over that one source map.
func plainRequests(t *testing.T, code checkout, base, target string, wantHunks int) func(name string, plan, receipt []byte) cemcandidate.PlainRequest {
	t.Helper()
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
	if err != nil || len(draft.Change.Hunks) != wantHunks {
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
	return func(name string, plan, receipt []byte) cemcandidate.PlainRequest {
		return cemcandidate.PlainRequest{Profile: cemcandidate.PlainAssemblyProfile, Repository: code.root, ExpectedBase: base, Target: target,
			SourceMap: ref(name+"-source.json", sourceBytes), RunnerPlan: ref(name+"-plan.json", plan), RunnerReceipt: ref(name+"-receipt.json", receipt),
			Criteria:        []cemcandidate.PlainCriterion{{CriterionEvidenceID: evidence, HunkIDs: hunks, EvidenceIDs: []string{evidence}}},
			RunnerAdmission: json.RawMessage(strconv.Quote(cemcandidate.RunnerAdmissionProfile))}
	}
}

// TestCEMV1286TasksRunnerProducerVerifyChain drives the bounded chain on one
// private disposable Tasks store, through the built corvint-tasks CLI, and one
// fixture checkout: a native claim, the
// trusted runner's plan and receipt, the plain producer with runner
// admission, and the Tasks cem-verdict gate running the built corvint CLI,
// which recomputes the verdict natively and through the independently
// authored portable consumer built from interop/cem01-go. Failed, skipped,
// stale and tampered receipts are each refused by the producer with their
// registered codes before any bundle exists (CEM-V1-286); a runner artifact
// tampered after production is refused by the gate, by native and portable
// recomputation, and by Tasks itself (CEM-V1-287). A GRANT superseded by
// another submitted tree never completes the ticket, which completes only on
// a fresh GRANT at its current candidate (CEM-V1-038).
func TestCEMV1286TasksRunnerProducerVerifyChain(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the cem-verdict gate and stable process containment are only supported on Darwin/Linux")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	corvint := build(t, repoRoot, "./cmd/corvint", "corvint")
	tasksBin := build(t, repoRoot, "./cmd/corvint-tasks", "corvint-tasks")
	runnerBin := build(t, repoRoot, "./cmd/corvint-test-runner", "corvint-test-runner")
	consumer := build(t, filepath.Join(repoRoot, "interop", "cem01-go"), ".", "cem01-go")
	run := newRunner(t, runnerBin)

	code, base := newCheckout(t, specLine)

	// Tasks: one native ticket claimed on the base; the attempt's base is it.
	s := newTasks(t, tasksBin, code.root, corvint, consumer)
	claim := s.claim(s.ticket(), ".corvint/changes/", "artifacts/", "src/")

	// Runner: a passing receipt over a draft that later changes is stale;
	// the final sources yield the passing, failed and skipped receipts.
	code.put("src/calc.go", calcDraft)
	code.put("src/calc_test.go", calcTest)
	stalePlan, staleReceipt := run.run("stale", code.root, 0, "TestPass")
	code.put("src/calc.go", calcFinal)
	passPlan, passReceipt := run.run("pass", code.root, 0, "TestPass")
	failPlan, failReceipt := run.run("fail", code.root, 1, "TestFail")
	skipPlan, skipReceipt := run.run("skip", code.root, 0, "TestSkip")
	code.put("artifacts/runner-plan.json", string(passPlan))
	code.put("artifacts/runner-receipt.json", string(passReceipt))
	code.git("add", ".")
	code.git("commit", "-qm", "calc: add Add with its runner receipt")
	target := code.git("rev-parse", "HEAD")

	// Producer map: every hunk cites the base spec line naming its path.
	ctx := context.Background()
	request := plainRequests(t, code, base, target, 4)
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
	opaque.RunnerAdmission = nil
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
	s.submit("submit-seal", claim, code.git("rev-parse", "HEAD^{tree}"))

	// Tasks gate: GRANT with native and portable ACCEPT and src/ anchored.
	s.gateRan("gate-seal", claim)
	g, out := s.result(claim.AttemptID)
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
	if e, raw := s.gate("gate-dirty", claim); e.Outcome == "OK" || !slices.Contains(e.Codes, wire.CodeDirtyWorktree) {
		t.Fatalf("dirty worktree gate: %s", raw)
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
	tamper := code.git("rev-parse", "HEAD")
	s.submit("submit-tamper", claim, code.git("rev-parse", "HEAD^{tree}"))

	// Historical authority (V1-2018, CEM-V1-038): the GRANT above was
	// recorded for the sealed tree, and submitting another tree supersedes
	// it. Tasks refuses completion at the superseded seal commit, which is
	// no longer the candidate, and at the new candidate, where the only
	// cem-verdict result last ran at another tree.
	e, raw := s.complete("complete-superseded-seal", claim, seal)
	refused(t, "completion at the superseded seal", e, raw, wire.CodeStaleTree)
	e, raw = s.complete("complete-historical-grant", claim, tamper)
	refused(t, "completion on the historical GRANT", e, raw, wire.CodeGateStale)

	s.gateRan("gate-tamper", claim)
	g, out = s.result(claim.AttemptID)
	doc = decodeVerdict(t, out)
	if g.State != "FAILED" || doc.Outcome != "REFUSE" || deref(doc.Code) != taskman.CEMVerdictProducerMismatch || deref(doc.Reason) != "sealed-tree-mismatch" {
		t.Fatalf("committed tamper not refused: state %s\n%s", g.State, out)
	}
	e, raw = s.complete("complete-failed", claim, tamper)
	refused(t, "completion on the failed verdict", e, raw, wire.CodeGateFailed)

	// Current authority: reverting the tamper restores the sealed tree, and
	// resubmitting it leaves the failed result at another tree, so
	// completion waits for a gate run at the current candidate, whose fresh
	// GRANT then completes the ticket.
	code.git("revert", "--no-edit", "HEAD")
	reseal := code.git("rev-parse", "HEAD")
	if tree := code.git("rev-parse", "HEAD^{tree}"); tree != code.git("rev-parse", seal+"^{tree}") {
		t.Fatalf("revert did not restore the sealed tree: %s", tree)
	}
	s.submit("submit-reseal", claim, code.git("rev-parse", "HEAD^{tree}"))
	e, raw = s.complete("complete-before-regate", claim, reseal)
	refused(t, "completion before re-running the gate", e, raw, wire.CodeGateStale)
	s.gateRan("gate-reseal", claim)
	if g, out = s.result(claim.AttemptID); g.State != "PASSED" || decodeVerdict(t, out).Outcome != "GRANT" {
		t.Fatalf("resealed chain not granted: state %s\n%s", g.State, out)
	}
	s.mutate("complete", "--attempt", claim.AttemptID, "--generation", claim.Generation, "--request-id", "complete-current", "--commit", reseal)
}

const (
	composeSpec  = "src/calc.go defines Add and Mul; src/calc_test.go holds TestAdd and TestMul; artifacts/runner-plan.json and artifacts/runner-receipt.json prove both repairs.\n"
	composeDraft = "package src\n\nfunc Add(a, b int) int { return a - b }\n\nfunc Mul(a, b int) int { return a + b }\n"
	composeAdd   = "package src\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Mul(a, b int) int { return a + b }\n"
	composeMul   = "package src\n\nfunc Add(a, b int) int { return a - b }\n\nfunc Mul(a, b int) int { return a * b }\n"
	composeFinal = "package src\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Mul(a, b int) int { return a * b }\n"
	composeTest  = "package src\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"Add(2, 3) != 5\")\n\t}\n}\n\nfunc TestMul(t *testing.T) {\n\tif Mul(2, 3) != 6 {\n\t\tt.Fatal(\"Mul(2, 3) != 6\")\n\t}\n}\n"
)

// TestCEMV1039RunnerRepairsComposeOnOneBoundSource drives two independent
// repairs of one draft through the trusted runner and the plain producer
// with admission (V1-2018, CEM-V1-039). Each isolated repair's passing
// receipt observes only its own fix over a source the composed target does
// not contain, so the producer refuses it as runner-receipt-stale. One plan
// over the composed source, selecting both repaired tests, yields the one
// admitted receipt, which observes both tests PASSED and is bound into the
// map as the runner's exact bytes.
func TestCEMV1039RunnerRepairsComposeOnOneBoundSource(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the trusted runner's process containment is only supported on Darwin/Linux")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	run := newRunner(t, build(t, repoRoot, "./cmd/corvint-test-runner", "corvint-test-runner"))
	code, base := newCheckout(t, composeSpec)

	// The draft fails both tests; each repair alone passes its own test.
	code.put("src/calc_test.go", composeTest)
	code.put("src/calc.go", composeDraft)
	run.run("draft", code.root, 1, "TestAdd", "TestMul")
	code.put("src/calc.go", composeAdd)
	addPlan, addReceipt := run.run("repair-add", code.root, 0, "TestAdd")
	code.put("src/calc.go", composeMul)
	mulPlan, mulReceipt := run.run("repair-mul", code.root, 0, "TestMul")
	code.put("src/calc.go", composeFinal)
	plan, receipt := run.run("composed", code.root, 0, "TestAdd", "TestMul")
	code.put("artifacts/runner-plan.json", string(plan))
	code.put("artifacts/runner-receipt.json", string(receipt))
	code.git("add", ".")
	code.git("commit", "-qm", "calc: compose the Add and Mul repairs")
	target := code.git("rev-parse", "HEAD")

	ctx := context.Background()
	request := plainRequests(t, code, base, target, 4)
	for _, c := range []struct {
		name          string
		plan, receipt []byte
	}{{"repair-add", addPlan, addReceipt}, {"repair-mul", mulPlan, mulReceipt}} {
		out := filepath.Join(realTemp(t), "bundle")
		_, err := cemcandidate.AssemblePlain(ctx, request(c.name, c.plan, c.receipt), out)
		if got := cemcode.CodeOf(err); got != "runner-receipt-stale" {
			t.Fatalf("isolated %s receipt: code %q (%v), want runner-receipt-stale", c.name, got, err)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("isolated %s receipt left a bundle: %v", c.name, err)
		}
	}

	bundle := filepath.Join(realTemp(t), "bundle")
	produced, err := cemcandidate.AssemblePlain(ctx, request("composed", plan, receipt), bundle)
	if err != nil || produced.Verification.Outcome != "ACCEPT" {
		t.Fatalf("composed receipt: %+v %v", produced.Verification, err)
	}
	bound, err := os.ReadFile(filepath.Join(bundle, "artifacts", "runner-receipt.json"))
	if err != nil || !bytes.Equal(bound, receipt) {
		t.Fatalf("bundle receipt is not the composed runner's bytes: %v", err)
	}
	var doc tr.ReceiptDocument
	if err := tr.DecodeDocument(bound, &doc); err != nil {
		t.Fatal(err)
	}
	passed := map[string]bool{}
	for _, test := range doc.Observation.Tests {
		passed[test.Name] = test.State == tr.Passed
	}
	if !doc.Observation.Complete || len(doc.Observation.Tests) != 2 || !passed["TestAdd"] || !passed["TestMul"] {
		t.Fatalf("composed receipt does not observe both repairs passing: %+v", doc.Observation)
	}
}
