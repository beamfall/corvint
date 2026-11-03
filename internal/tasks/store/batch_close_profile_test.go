// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestV14315_BatchCloseProfile is the opt-in V1-4315 before/after measurement
// of gated `ticket batch-close` on a large store (TCX-V0-970 to TCX-V0-976),
// not a timing assertion. It builds one historyStoreAt fixture of
// CORVINT_TASKS_BATCH_CLOSE_TICKETS tickets and CORVINT_TASKS_BATCH_CLOSE_RECEIPTS
// receipts inside a real Git repository with a required local gate, creates
// CORVINT_TASKS_BATCH_CLOSE_ROWS closable tickets delivered by
// CORVINT_TASKS_BATCH_CLOSE_BATCHES batch merges (one batch PR each, the rows
// split evenly), keeps a pristine copy, and then, for each `label=path`
// corvint-tasks binary in CORVINT_TASKS_BATCH_CLOSE_BINARIES and each of
// CORVINT_TASKS_BATCH_CLOSE_REPS repetitions, restores the fixture to the
// same path (a copied store refuses relocation), times `ticket batch-close
// --dry-run` of the first batch (the planning cost) and the gated close of
// every batch, and checks that every row closed VERIFIED and that `receipt
// audit` still agrees. One JSON line per (binary, rep) is appended to
// CORVINT_TASKS_BATCH_CLOSE_PROFILE; a pair already recorded there for the
// same sizes is skipped, so the same command can be rerun until every pair is
// recorded. CORVINT_TASKS_BATCH_CLOSE_DIR names a fixed working directory
// whose pristine fixture a later invocation with the same sizes reuses.
// CORVINT_TASKS_BATCH_CLOSE_BUDGET_SECONDS bounds one invocation: once it is
// exceeded no further batch is started, the pair's progress is kept in
// `inflight.json` in the directory, and the next invocation of the same
// command continues that pair from its next batch. A slow baseline over 50
// rows is measured in several invocations that way, each under the lane's
// per-command bound, and its wall time is the sum of its batches.
func TestV14315_BatchCloseProfile(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_BATCH_CLOSE_PROFILE")
	if output == "" {
		t.Skip("set CORVINT_TASKS_BATCH_CLOSE_PROFILE to a report path")
	}
	envInt := func(name string, def int) int {
		if raw := os.Getenv(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				t.Fatalf("%s=%q", name, raw)
			}
			return n
		}
		return def
	}
	type binary struct{ label, path string }
	var bins []binary
	for _, item := range strings.Split(os.Getenv("CORVINT_TASKS_BATCH_CLOSE_BINARIES"), ":") {
		label, path, ok := strings.Cut(item, "=")
		if !ok || label == "" || path == "" {
			t.Fatalf("CORVINT_TASKS_BATCH_CLOSE_BINARIES wants label=path[:label=path...], got %q", item)
		}
		bins = append(bins, binary{label, path})
	}
	tickets := envInt("CORVINT_TASKS_BATCH_CLOSE_TICKETS", 4000)
	receipts := envInt("CORVINT_TASKS_BATCH_CLOSE_RECEIPTS", 19200)
	rows := envInt("CORVINT_TASKS_BATCH_CLOSE_ROWS", 50)
	reps := envInt("CORVINT_TASKS_BATCH_CLOSE_REPS", 1)
	batches := envInt("CORVINT_TASKS_BATCH_CLOSE_BATCHES", 1)
	budget := time.Duration(envInt("CORVINT_TASKS_BATCH_CLOSE_BUDGET_SECONDS", 0)) * time.Second
	if receipts <= tickets+1 || rows < 1 || batches < 1 || batches > rows || reps < 1 {
		t.Fatalf("receipts %d must exceed tickets+1 (%d); rows %d, batches %d, reps %d must be positive with batches <= rows", receipts, tickets, rows, batches, reps)
	}
	started := time.Now()
	overBudget := func() bool { return budget > 0 && time.Since(started) > budget }
	dir := os.Getenv("CORVINT_TASKS_BATCH_CLOSE_DIR")
	if dir == "" {
		base, err := os.MkdirTemp("", "atm-batch-close-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
		dir = base
	}
	var err error
	if dir, err = filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	root, pristine, stampPath, inflightPath := filepath.Join(dir, "repo"), filepath.Join(dir, "pristine"), filepath.Join(dir, "fixture.json"), filepath.Join(dir, "inflight.json")
	type stamp struct {
		Tickets, Receipts, Rows, Batches int
		Merges                           []string
		Prs, Runs                        string
		SetupMS                          float64
	}
	var fx stamp
	if raw, err := os.ReadFile(stampPath); err == nil {
		if json.Unmarshal(raw, &fx) != nil || fx.Tickets != tickets || fx.Receipts != receipts || fx.Rows != rows || fx.Batches != batches {
			fx = stamp{}
		}
	}
	if len(fx.Merges) == 0 {
		os.RemoveAll(root)
		os.RemoveAll(pristine)
		os.Remove(inflightPath)
		setup := time.Now()
		fx = stamp{Tickets: tickets, Receipts: receipts, Rows: rows, Batches: batches}
		fx.Merges, fx.Prs, fx.Runs = batchCloseFixture(t, root, tickets, receipts, rows, batches)
		fx.SetupMS = ms(time.Since(setup))
		run(t, "cp", "-a", root, pristine)
		raw, _ := json.Marshal(fx)
		if err := os.WriteFile(stampPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("fixture: %d tickets, %d receipts, %d rows in %d batches, built in %.0f ms", tickets, receipts, rows, batches, fx.SetupMS)
	}
	type result struct {
		Label                            string
		Tickets, Receipts, Rows, Batches int
		Rep                              int
		Host, GoVersion                  string
		NProc                            int
		LoadBefore, LoadAfter            string
		DryRunMS                         float64
		CloseMS, CloseCPUMS              float64
		BatchMS                          []float64
		PerRowMS                         float64
		Closed, Verified                 int
		ExitCodes                        []int
		Codes                            []string
		AuditMS                          float64
		AuditOK                          bool
		AuditVerified                    string
		SetupMS                          float64
		BinarySha256                     string
		Invocations                      int
	}
	// inflight is one pair's progress across invocations.
	type inflight struct {
		Label string
		Rep   int
		Next  int // next batch index to close
		Res   result
	}
	recorded := map[string]bool{}
	if raw, err := os.ReadFile(output); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			var r result
			if json.Unmarshal([]byte(line), &r) == nil && r.Tickets == tickets && r.Receipts == receipts && r.Rows == rows && r.Batches == batches {
				recorded[r.Label+"#"+strconv.Itoa(r.Rep)] = true
			}
		}
	}
	var state *inflight
	if raw, err := os.ReadFile(inflightPath); err == nil {
		state = &inflight{}
		if json.Unmarshal(raw, state) != nil {
			state = nil
		}
	}
	saveState := func() {
		raw, _ := json.Marshal(state)
		if err := os.WriteFile(inflightPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	host, _ := os.Hostname()
	report, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer report.Close()
	closeArgs := func(batch int) []string {
		return []string{"ticket", "batch-close", "--pr", strconv.Itoa(50 + batch), "--prs", fx.Prs, "--runs", fx.Runs}
	}
	for rep := 1; rep <= reps; rep++ {
		for _, b := range bins {
			if recorded[b.label+"#"+strconv.Itoa(rep)] {
				continue
			}
			if state == nil || state.Label != b.label || state.Rep != rep {
				if overBudget() {
					t.Logf("budget exceeded before %s rep %d; rerun to continue", b.label, rep)
					return
				}
				run(t, "rm", "-rf", root)
				run(t, "cp", "-a", pristine, root)
				r := result{Label: b.label, Tickets: tickets, Receipts: receipts, Rows: rows, Batches: batches, Rep: rep, Host: host, GoVersion: goVersion(), NProc: nproc(), SetupMS: fx.SetupMS, BinarySha256: binarySha256(t, b.path)}
				r.LoadBefore = hostLoad()
				dry := batchCloseRun(t, b.path, root, append(closeArgs(0), "--dry-run")...)
				r.DryRunMS = dry.wallMS
				state = &inflight{Label: b.label, Rep: rep, Res: r}
				saveState()
			}
			r := &state.Res
			r.Invocations++
			for state.Next < batches {
				if overBudget() {
					saveState()
					t.Logf("%s rep %d: budget exceeded after batch %d of %d (%.0f ms so far); rerun the same command to continue", b.label, rep, state.Next, batches, r.CloseMS)
					return
				}
				full := batchCloseRun(t, b.path, root, closeArgs(state.Next)...)
				r.CloseMS += full.wallMS
				r.CloseCPUMS += full.cpuMS
				r.BatchMS = append(r.BatchMS, full.wallMS)
				r.ExitCodes = append(r.ExitCodes, full.code)
				for _, c := range full.res.Codes {
					r.Codes = append(r.Codes, c)
				}
				if len(full.res.Items) == 1 {
					for _, row := range field(full.res.Items[0], "tickets").Arr {
						if field(row, "disposition").Str == "CLOSED" {
							r.Closed++
							if field(row, "completionKind").Str == "VERIFIED" {
								r.Verified++
							}
						}
					}
				}
				state.Next++
				saveState()
				t.Logf("%s rep %d: batch %d of %d closed in %.0f ms (exit %d)", b.label, rep, state.Next, batches, full.wallMS, full.code)
			}
			if r.Closed > 0 {
				r.PerRowMS = (r.CloseMS - float64(batches)*r.DryRunMS) / float64(r.Closed)
			}
			audit := batchCloseRun(t, b.path, root, "receipt", "audit")
			r.AuditMS = audit.wallMS
			if len(audit.res.Items) == 1 {
				item := audit.res.Items[0]
				r.AuditOK = audit.code == 0 && field(item, "structuralConsistency").Str == "CONSISTENT" && field(item, "projectionAgreement").Str == "AGREES"
				r.AuditVerified = field(field(item, "completions"), "verified").Str
			}
			r.LoadAfter = hostLoad()
			line, _ := json.Marshal(r)
			if _, err := fmt.Fprintf(report, "%s\n", line); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s rep %d: dry-run %.0f ms, close %.0f ms over %d batches (cpu %.0f ms), %d closed / %d verified of %d, %.0f ms per row, exits %v %v, audit %.0f ms ok=%v verified=%s, load %s -> %s, invocations %d",
				b.label, rep, r.DryRunMS, r.CloseMS, batches, r.CloseCPUMS, r.Closed, r.Verified, rows, r.PerRowMS, r.ExitCodes, r.Codes, r.AuditMS, r.AuditOK, r.AuditVerified, r.LoadBefore, r.LoadAfter, r.Invocations)
			if r.Closed != rows || r.Verified != rows || !r.AuditOK {
				t.Errorf("%s rep %d: %d closed, %d verified of %d rows, audit ok %v", b.label, rep, r.Closed, r.Verified, rows, r.AuditOK)
			}
			recorded[b.label+"#"+strconv.Itoa(rep)] = true
			state = nil
			os.Remove(inflightPath)
		}
	}
}

// batchCloseFixture builds the V1-4315 fixture at root: a Git repository on
// main with a base commit, one lane commit per batch touching src/a<k>.go,
// each merged into main in order, a historyStoreAt store of tickets and
// receipts (sealed into segments as a live store of that length is), a policy
// with the required local gate `verify`, rows closable tickets (undeclared
// scope, so each batch merge is its rows' candidate) split evenly over the
// batches, and the batch and run exports batch-close reads: lane PR 40+k and
// batch PR 50+k for batch k. It returns the merge commits and the two
// export paths.
func batchCloseFixture(t *testing.T, root string, tickets, receipts, rows, batches int) (merges []string, prs, runs string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	fixture.Write(t, filepath.Join(root, "src", "keep.txt"), []byte("keep\n"))
	git("add", "-A", "--", "src")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	lanes := make([]string, batches)
	for k := 0; k < batches; k++ {
		git("checkout", "-q", "-b", "lane-"+strconv.Itoa(k), base)
		fixture.Write(t, filepath.Join(root, "src", "a"+strconv.Itoa(k)+".go"), []byte("package src\n"))
		git("add", "-A", "--", "src")
		git("commit", "-q", "-m", "lane "+strconv.Itoa(k))
		lanes[k] = git("rev-parse", "HEAD")
	}
	git("checkout", "-q", "main")
	for k := 0; k < batches; k++ {
		git("merge", "-q", "--no-ff", "-m", "Batch "+strconv.Itoa(k), lanes[k])
		merges = append(merges, git("rev-parse", "HEAD"))
	}

	repo := historyStoreAt(t, root, tickets, receipts)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	policy := fixture.PolicyValue()
	policy.Obj.Set("policyVersion", wire.String("2"))
	policy.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String("4"), "maxWorkersTotal", wire.String("4"), "classes", wire.Array()))
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	expected := wire.NewObject().Set("exitCode", wire.String("0")).Set("reducer", wire.Null())
	verify := wire.NewObject().Set("gateId", wire.String("verify")).Set("kind", wire.String("COMMAND")).
		Set("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"})).Set("cwd", wire.String("WORKTREE")).Set("env", wire.Strings(nil)).
		Set("timeoutSeconds", wire.String("30")).Set("expected", wire.ObjectValue(expected)).Set("evidence", wire.Strings(nil)).
		Set("inputs", wire.Strings(nil)).Set("sharedResource", wire.Null()).Set("reusable", wire.Bool(false)).Set("required", wire.Bool(true))
	policy.Obj.Set("gates", wire.Array(wire.ObjectValue(verify)))
	report, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "batch-close-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(policy)}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	locals := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		id := fmt.Sprintf("batch-close-create-%d", i)
		env, err := wire.Parse(historyCreate(id, "batch close row "+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		payload, _ := env.Obj.Get("payload")
		payload.Obj.Set("effects", historyObject("coverage", wire.String("INCOMPLETE"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings([]string{"elsewhere/"})))
		payload.Obj.Set("requiredGates", wire.Strings([]string{"verify"}))
		rep, err := Mutate(context.Background(), repo, historyActor, wire.EncodeFile(env), WallClock())
		if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("create row %d: %+v %v", i, rep, err)
		}
		locals = append(locals, rep.Ticket[strings.LastIndex(rep.Ticket, ":")+1:])
	}
	var prEntries, runEntries []string
	for k := 0; k < batches; k++ {
		lo, hi := k*rows/batches, (k+1)*rows/batches
		refs := make([]string, 0, hi-lo)
		for _, local := range locals[lo:hi] {
			refs = append(refs, "Refs "+local)
		}
		body, _ := json.Marshal(strings.Join(refs, "\n"))
		lanePR, batchPR := strconv.Itoa(40+k), strconv.Itoa(50+k)
		prEntries = append(prEntries,
			`{"number":`+lanePR+`,"state":"MERGED","title":"lane `+strconv.Itoa(k)+`","body":`+string(body)+`,"headRefOid":"`+lanes[k]+`","labels":[]}`,
			`{"number":`+batchPR+`,"state":"MERGED","title":"Batch `+strconv.Itoa(k)+`: #`+lanePR+`","body":"- #`+lanePR+`","headRefOid":"`+lanes[k]+`","mergeCommit":{"oid":"`+merges[k]+`"},"mergedAt":"2026-11-02T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/`+batchPR+`","labels":[]}`)
		runEntries = append(runEntries, `{"databaseId":`+strconv.Itoa(7+k)+`,"headSha":"`+merges[k]+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/`+strconv.Itoa(7+k)+`"}`)
	}
	prs = filepath.Join(filepath.Dir(root), "prs.json")
	runs = filepath.Join(filepath.Dir(root), "runs.json")
	fixture.Write(t, prs, []byte("[\n"+strings.Join(prEntries, ",\n")+"]\n"))
	fixture.Write(t, runs, []byte("[\n"+strings.Join(runEntries, ",\n")+"]\n"))
	return merges, prs, runs
}

type batchCloseResult struct {
	wallMS, cpuMS float64
	code          int
	stdout        []byte
	res           *wire.Result
}

// batchCloseRun runs one corvint-tasks command in root and decodes its result.
func batchCloseRun(t *testing.T, bin, root string, args ...string) batchCloseResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CORVINT_TASKS_ACTOR=tester", "ATM_ACTOR=tester")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	start := time.Now()
	out, err := cmd.Output()
	wall := time.Since(start)
	r := batchCloseResult{wallMS: ms(wall), stdout: out}
	if cmd.ProcessState != nil {
		r.cpuMS = ms(cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime())
		r.code = cmd.ProcessState.ExitCode()
	}
	if err != nil && cmd.ProcessState == nil {
		t.Fatalf("%s %v: %v\n%s", bin, args, err, stderr.String())
	}
	res, decodeErr := wire.DecodeResult(out)
	if decodeErr != nil {
		t.Fatalf("%s %v: envelope does not decode: %v\n%s\n%s", bin, args, decodeErr, out, stderr.String())
	}
	r.res = res
	return r
}

func field(v wire.Value, key string) wire.Value {
	if v.Kind != wire.KindObject {
		return wire.Null()
	}
	got, ok := v.Obj.Get(key)
	if !ok {
		return wire.Null()
	}
	return got
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func goVersion() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return "NOT_OBSERVED"
	}
	return strings.TrimSpace(string(out))
}

func nproc() int {
	out, err := exec.Command("nproc").Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func binarySha256(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(wire.Sum(raw))
}
