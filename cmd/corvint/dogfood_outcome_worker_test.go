package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func canonicalWorkerTest(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return append(wire.CanonicalValue(parsed), '\n')
}

func TestAggregateWorkerNativeProduceVerify(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native Owner unavailable")
	}
	runner := portableDogfoodRunner(t)
	root, _ := portableDogfoodRepo(t)
	base := cemGit(t, root, "rev-parse", "HEAD")
	for i := 0; i < 201; i++ {
		cemWrite(t, root, fmt.Sprintf("aggregate/f%03d.go", i), fmt.Sprintf("package aggregate\nconst V%d = %d\n", i, i))
	}
	cemGit(t, root, "add", "aggregate")
	cemGit(t, root, "commit", "-qm", "aggregate source")
	expected := tracerecordrepo.AggregateExpected{ObjectFormat: "sha1", Base: base, Target: cemGit(t, root, "rev-parse", "HEAD"), Tree: cemGit(t, root, "rev-parse", "HEAD^{tree}"), AdmissionPolicy: tracerecordrepo.AggregateAdmissionPolicy, Task: "Local completion " + strings.Repeat("a", 64), Verification: []string{"go test ./aggregate"}, Outcome: "passed"}
	request := dogfoodflow.AggregateWorkerRequest{Profile: dogfoodflow.AggregateWorkerProfile, Mode: "produce", Expected: expected, RemainingGitOperations: 56, RemainingWorkMilliseconds: 60000, Receipt: json.RawMessage("null")}
	invoke := func(raw []byte, failure string) []byte {
		t.Helper()
		command := exec.Command(runner.binary, "dogfood-outcome-worker", root)
		command.Env = runner.env
		command.Stdin = bytes.NewReader(raw)
		command.WaitDelay = 2 * time.Second
		var out, stderr bytes.Buffer
		command.Stdout = &out
		command.Stderr = &stderr
		owner, err := groupreap.Start(command)
		if err != nil {
			t.Fatal(err)
		}
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		select {
		case <-owner.Exited():
		case <-timer.C:
			owner.Stop()
		}
		deadline := time.Now().Add(20 * time.Second)
		result := owner.FinishBounded(groupreap.RetirementBound{Expired: func() bool { return !time.Now().Before(deadline) }})
		if result.State != groupreap.Released {
			t.Fatalf("worker state=%s exit=%v cleanup=%v stderr=%s", result.State, result.WaitErr, result.Err, stderr.String())
		}
		if failure != "" {
			var exit *exec.ExitError
			var refusal struct {
				Code  string `json:"code"`
				Error string `json:"error"`
				OK    bool   `json:"ok"`
			}
			if !errors.As(result.WaitErr, &exit) || exit.ExitCode() != 2 || out.Len() != 0 || json.Unmarshal(stderr.Bytes(), &refusal) != nil || refusal.Code != failure || refusal.OK {
				t.Fatalf("typed worker refusal: exit=%v stdout=%s stderr=%s", result.WaitErr, &out, &stderr)
			}
			return stderr.Bytes()
		}
		if result.WaitErr != nil {
			t.Fatalf("worker exit=%v stderr=%s", result.WaitErr, &stderr)
		}
		if stderr.Len() != 0 {
			t.Fatalf("unexpected stderr=%s", stderr.String())
		}
		return out.Bytes()
	}
	raw := invoke(canonicalWorkerTest(t, request), "")
	var response map[string]json.RawMessage
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if len(response) != 5 {
		t.Fatalf("response fields=%d", len(response))
	}
	var consumed int
	if err := json.Unmarshal(response["consumedGitOperations"], &consumed); err != nil || consumed < 1 || consumed > 56 {
		t.Fatalf("consumed=%d err=%v", consumed, err)
	}
	receipt, err := tracerecordrepo.ParseAggregateOutcome(append(bytes.Clone(response["receipt"]), '\n'))
	if err != nil || receipt.AdmittedCount != 201 {
		t.Fatalf("receipt count=%d err=%v", receipt.AdmittedCount, err)
	}
	t.Logf("native produce consumed %d/56 physical Git operations", consumed)
	request.Mode = "verify"
	request.Receipt = response["receipt"]
	raw = invoke(canonicalWorkerTest(t, request), "")
	response = nil
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if len(response) != 4 {
		t.Fatalf("verify response fields=%d", len(response))
	}
	var envelope string
	if err := json.Unmarshal(response["verifiedEnvelopeSha256"], &envelope); err != nil || envelope != receipt.EnvelopeSHA256 {
		t.Fatal("verification mismatch", err)
	}
	command := exec.Command(runner.binary, "dogfood-outcome-worker", root)
	command.Env = runner.env
	command.Stdin = bytes.NewReader(canonicalWorkerTest(t, request))
	out, err := command.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("aggregate-worker-invalid")) {
		t.Fatalf("uncontained worker accepted: %s %v", out, err)
	}
	for i := 201; i < 513; i++ {
		cemWrite(t, root, fmt.Sprintf("aggregate/f%03d.go", i), fmt.Sprintf("package aggregate\nconst V%d = %d\n", i, i))
	}
	cemGit(t, root, "add", "aggregate")
	cemGit(t, root, "commit", "-qm", "over aggregate admission")
	request.Mode = "produce"
	request.Receipt = json.RawMessage("null")
	request.Expected.Target = cemGit(t, root, "rev-parse", "HEAD")
	request.Expected.Tree = cemGit(t, root, "rev-parse", "HEAD^{tree}")
	invoke(canonicalWorkerTest(t, request), "aggregate-admitted-limit")
}

// This uses genuine enrollment, frozen verification and an actual legacy
// recorder failure before selecting the separate aggregate outcome profile.
func TestAggregateFinishNativeForeignRepository(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native Owner unavailable")
	}
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	for i := 0; i < 200; i++ {
		cemWrite(t, root, fmt.Sprintf("aggregate/f%03d.go", i), fmt.Sprintf("package aggregate\nconst V%d = %d\n", i, i))
	}
	cemGit(t, root, "add", "aggregate")
	cemGit(t, root, "commit", "-qm", "aggregate source")
	key := localcompletion.HashSession(t.Name())
	plan, _ := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"intent.md"}, Checks: []localcompletion.Check{{ID: "actual-test", Argv: []string{"go", "test", "./fixture", "./aggregate"}, TimeoutSeconds: 60}}})
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	run.ok(t, root, "dogfood", "begin", "--plan", planPath, "--session-key", key)
	run.ok(t, root, "cem", "prepare", "--base", base, "--target", "HEAD")
	for i := 1; i <= 201; i++ {
		run.ok(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", fmt.Sprint(i), "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind evidence")
	run.ok(t, root, "dogfood", "verify", "--check", "actual-test", "--session-key", key)
	code, out, stderr := run.exec(t, root, nil, "dogfood", "finish", "--session-key", key)
	var evaluation struct {
		Policy struct {
			ReportSetDigest string `json:"reportSetDigest"`
			Satisfied       bool   `json:"satisfied"`
		} `json:"policy"`
	}
	if err := json.Unmarshal([]byte(out), &evaluation); err != nil || code != 1 || evaluation.Policy.ReportSetDigest == "" {
		t.Fatalf("legacy finish: %d %s %s %v", code, out, stderr, err)
	}
	run.ok(t, root, "dogfood", "review", "--report-set", evaluation.Policy.ReportSetDigest, "--session-key", key)
	// Review precedes the coordinator; only this second legacy finish actually
	// invokes the recorder and retains its admission-limit refusal.
	code, out, stderr = run.exec(t, root, nil, "dogfood", "finish", "--session-key", key)
	if code == 0 {
		t.Fatalf("legacy recorder unexpectedly accepted aggregate: %s %s", out, stderr)
	}

	if faultBinary := os.Getenv("CORVINT_AGGREGATE_FAULT_TEST_BINARY"); faultBinary != "" {
		aggregateNativeRecoveryFaults(t, run, root, key, faultBinary)
	}
	if os.Getenv("CORVINT_QUAL_NATIVE_PHASE") == "1" {
		aggregateQualificationNativeSignals(t, run, root, key)
	}
	code, out, stderr = run.exec(t, root, nil, "dogfood", "finish", "--session-key", key, "--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile)
	if code != 0 {
		_ = filepath.WalkDir(filepath.Join(root, ".git", "corvint"), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && (strings.Contains(path, "legacy-failure") || strings.HasSuffix(path, "coordination-time.stderr") || strings.HasSuffix(path, "state.json")) {
				raw, _ := os.ReadFile(path)
				t.Logf("%s: %s", path, raw)
			}
			return nil
		})
		raw, _ := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
		t.Logf("report: %s", raw)
		t.Fatalf("aggregate finish exit=%d stdout=%s stderr=%s", code, out, stderr)
	}
	if err := json.Unmarshal([]byte(out), &evaluation); err != nil || !evaluation.Policy.Satisfied {
		t.Fatalf("aggregate finish: %s %v", out, err)
	}
	report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = dogfoodflow.ParseAggregateReport(report); err != nil {
		t.Fatal(err)
	}
	// Read-only consumers and the success fast path must not consume a history
	// ordinal or rewrite any enrolled evidence, even though they revalidate it.
	evidenceBytes := func() map[string][32]byte {
		result := map[string][32]byte{}
		for _, prefix := range []string{".git/corvint/local-completion", ".corvint"} {
			err := filepath.WalkDir(filepath.Join(root, prefix), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, _ := filepath.Rel(root, path)
				result[relative] = sha256.Sum256(raw)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	before := evidenceBytes()
	status := run.ok(t, root, "dogfood", "status", "--session-key", key)
	if err := json.Unmarshal([]byte(status), &evaluation); err != nil || !evaluation.Policy.Satisfied {
		t.Fatalf("fresh status: %s %v", status, err)
	}
	run.ok(t, root, "dogfood", "finish", "--session-key", key, "--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile)
	if after := evidenceBytes(); !reflect.DeepEqual(before, after) {
		t.Fatal("status or identical finish mutated enrollment/history")
	}
	again, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if err != nil || !bytes.Equal(report, again) {
		t.Fatalf("identical finish rewrote report: %v", err)
	}
	if _, err = os.Stat(filepath.Join(root, ".context-corvint", "traces")); !os.IsNotExist(err) {
		t.Fatalf("aggregate opened trace store: %v", err)
	}
	if helper := os.Getenv("CORVINT_QUAL_CAPACITY_TEST_BINARY"); helper != "" {
		command := exec.Command(helper, "-test.run=^TestAggregateQualificationCommittedMixedCapacityHelper$", "-test.count=1", "-test.timeout=30s", "-test.v")
		command.Env = append(os.Environ(), "CORVINT_QUAL_COMMITTED_ROOT="+root, "CORVINT_QUAL_COMMITTED_KEY="+key)
		command.WaitDelay = time.Second
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("actual committed capacity: %v %s", err, out)
		}
		t.Logf("%s", out)
	}
	if os.Getenv("CORVINT_QUAL_NATIVE_PHASE") == "1" {
		aggregateQualificationStaleCommitted(t, run, root, key)
		return
	}
	t.Run("predecessor-and-pending-dispatch", func(t *testing.T) {
		predecessor := os.Getenv("CORVINT_AGGREGATE_PREDECESSOR_TEST_BINARY")
		if predecessor == "" {
			t.Skip("separate qualification supplies a hashed immutable predecessor binary")
		}
		old := portableRun{binary: predecessor, env: run.env}
		statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
		committed, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.WriteFile(statePath, committed, 0600); err != nil {
				t.Error(err)
			}
		}()
		invokeEvent := func(binary string) (int, []byte, []byte) {
			args := append([]string{"dogfood", "event"}, dogfoodEventArguments("stop")...)
			command := exec.Command(binary, args...)
			command.Dir = root
			command.Env = run.env
			command.Stdin = strings.NewReader(`{"sessionIdSha256":"` + key + `"}`)
			command.WaitDelay = 2 * time.Second
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			code := 0
			var exit *exec.ExitError
			if err != nil {
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			return code, stdout.Bytes(), stderr.Bytes()
		}
		for _, phase := range []string{"PREPARED", "OUTCOME_PUBLISHED", "REPORT_PUBLISHED", "COMMITTED"} {
			var value map[string]any
			if err := json.Unmarshal(committed, &value); err != nil {
				t.Fatal(err)
			}
			value["aggregateOutcome"].(map[string]any)["phase"] = phase
			if phase != "COMMITTED" {
				value["lifecycle"] = "active"
				value["terminal"] = nil
			}
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(statePath, append(raw, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			// These lower-bound phase cases retain genuinely issued and captured
			// bytes from the binary run; they are dispatch tests, not crash witnesses.
			before := evidenceBytes()
			for _, action := range []string{"status", "finish"} {
				code, out, stderr := old.exec(t, root, nil, "dogfood", action, "--session-key", key)
				if code != 2 || out != "" {
					t.Fatalf("predecessor %s/%s accepted: %d %s %s", phase, action, code, out, stderr)
				}
			}
			code, out, stderr := invokeEvent(predecessor)
			if code != 2 || len(out) != 0 {
				t.Fatalf("predecessor event %s accepted: %d %s %s", phase, code, out, stderr)
			}
			if phase != "COMMITTED" {
				out := run.ok(t, root, "dogfood", "status", "--session-key", key)
				if err := json.Unmarshal([]byte(out), &evaluation); err != nil || evaluation.Policy.Satisfied {
					t.Fatalf("pending status %s: %s %v", phase, out, err)
				}
				code, out, stderr := run.exec(t, root, nil, "dogfood", "finish", "--session-key", key)
				if code != 2 || !strings.Contains(stderr, "aggregate-enrollment-required") {
					t.Fatalf("pending selector omission %s: %d %s %s", phase, code, out, stderr)
				}
			}
			if after := evidenceBytes(); !reflect.DeepEqual(before, after) {
				t.Fatalf("dispatch mutated evidence at %s", phase)
			}
		}
	})
}

// The optional binary is built with a retained Go overlay of only the existing
// private fault seam. Production source exposes no fault environment switch.
func aggregateNativeRecoveryFaults(t *testing.T, run portableRun, root, key, faultBinary string) {
	t.Helper()
	type retained struct {
		raw  []byte
		mode os.FileMode
	}
	baseline := map[string]retained{}
	roots := []string{filepath.Join(root, ".git/corvint"), filepath.Join(root, ".corvint")}
	for _, directory := range roots {
		err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("nonregular fixture %s", path)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			baseline[path] = retained{raw, info.Mode().Perm()}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	restore := func() {
		// Independent cases reset only this disposable fixture to the same actual
		// pre-aggregate enrollment. Recovery within a case never deletes history.
		for _, directory := range roots {
			if err := os.RemoveAll(directory); err != nil {
				t.Fatal(err)
			}
		}
		for path, value := range baseline {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, value.raw, value.mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	points := []string{"reservation-installed", "state-before-closed", "reservation-state-saved", "snapshot-manifest-installed", "snapshot-sealed-acknowledged"}
	for _, role := range []string{"check-capture", "check-stderr", "check-stdout", "legacy-failure", "legacy-stderr", "legacy-stdout", "prior-outcome", "prior-report", "state-before"} {
		// Absent prior check captures have no payload-close boundary on the first
		// attempt; their later replacement is covered by the storage process tests.
		if strings.HasPrefix(role, "check-") {
			continue
		}
		points = append(points, "payload-closed:"+role)
	}
	for _, role := range []string{"outcome", "report", "check-report"} {
		for _, boundary := range []string{"prepared-closed:", "stage-closed:", "issued-saved:", "artifact-renamed:", "phase-acknowledged:"} {
			points = append(points, boundary+role)
		}
	}
	for _, name := range []string{"check.stdout", "check.stderr", "check-report.prepared", "check.capture.json"} {
		points = append(points, "check-capture-closed:"+name)
	}
	points = append(points, "before-terminal-state", "terminal-state-committed")
	for _, point := range points {
		t.Run("native-recovery/"+point, func(t *testing.T) {
			restore()
			marker := filepath.Join(t.TempDir(), "fault-observed")
			faultRun := portableRun{binary: faultBinary, env: run.env}
			code, out, stderr := faultRun.exec(t, root, []string{"CORVINT_AGGREGATE_TEST_FAULT=" + point, "CORVINT_AGGREGATE_TEST_FAULT_MARKER=" + marker}, "dogfood", "finish", "--session-key", key, "--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile)
			marked, err := os.ReadFile(marker)
			if err != nil || string(marked) != point || code == 0 {
				t.Fatalf("fault not observed: point=%s code=%d marker=%s err=%v out=%s stderr=%s", point, code, marked, err, out, stderr)
			}
			statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			status := run.ok(t, root, "dogfood", "status", "--session-key", key)
			var evaluation struct {
				Policy struct {
					Satisfied bool `json:"satisfied"`
				} `json:"policy"`
			}
			if err = json.Unmarshal([]byte(status), &evaluation); err != nil {
				t.Fatal(err)
			}
			if evaluation.Policy.Satisfied != (point == "terminal-state-committed") {
				t.Fatalf("wrong pending satisfaction at %s: %s", point, status)
			}
			after, err := os.ReadFile(statePath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("pending status wrote state: %v", err)
			}
			out = run.ok(t, root, "dogfood", "finish", "--session-key", key, "--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile)
			if err = json.Unmarshal([]byte(out), &evaluation); err != nil || !evaluation.Policy.Satisfied {
				t.Fatalf("new-process real recovery failed: %v %s", err, out)
			}
		})
	}
	restore()
}

// The existing native fixture establishes real enrollment/legacy refusal.
// These extra cells are opt-in so ordinary unit runs do not repeat the campaign.
func aggregateQualificationNativeSignals(t *testing.T, run portableRun, root, key string) {
	t.Helper()
	binaryRaw, binaryErr := os.ReadFile(run.binary)
	if binaryErr != nil {
		t.Fatal(binaryErr)
	}
	t.Logf("native candidate sha256=%x", sha256.Sum256(binaryRaw))
	type heldFile struct {
		raw  []byte
		mode os.FileMode
	}
	baseline := map[string]heldFile{}
	prefixes := []string{filepath.Join(root, ".git/corvint"), filepath.Join(root, ".corvint")}
	for _, prefix := range prefixes {
		if err := filepath.WalkDir(prefix, func(path string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() {
				info, err := e.Info()
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				baseline[path] = heldFile{raw, info.Mode().Perm()}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	restore := func() {
		for _, prefix := range prefixes {
			if err := os.RemoveAll(prefix); err != nil {
				t.Fatal(err)
			}
		}
		for path, value := range baseline {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, value.raw, value.mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	proofRoot := os.Getenv("CORVINT_QUAL_NATIVE_EVIDENCE")
	if proofRoot == "" {
		t.Fatal("missing private evidence destination")
	}
	for _, mode := range []string{"INT", "TERM", "KILL"} {
		t.Run("qualification-signal/"+mode, func(t *testing.T) {
			restore()
			bin := t.TempDir()
			marker := filepath.Join(bin, "ready")
			quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
			script := "#!/bin/sh\nparent=$(/bin/ps -p \"$PPID\" -o command=)\ncase \"$parent\" in *dogfood-verifier-worker*) trap '' TERM; /bin/sleep 60 & child=$!; printf '%s %s %s\\n' \"$PPID\" \"$$\" \"$child\" > " + quote(marker) + "; wait;; esac\nexec " + quote(realGit) + " \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(run.binary, "dogfood", "finish", "--session-key", key, "--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile)
			command.Dir = root
			command.Env = append(run.env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			command.WaitDelay = 2 * time.Second
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			joined := false
			group := "" // Only a validated owned group may populate this field.
			var cleanupGroup func() error
			retirementObserved := false
			defer func() {
				if !joined {
					_ = command.Process.Kill()
					select {
					case <-done:
						joined = true
					case <-time.After(5 * time.Second):
						t.Error("parent failed to join")
					}
				}
				if cleanupGroup != nil {
					_ = cleanupGroup()
				}
				if !retirementObserved {
					// Joining the CLI does not establish retirement of its separate worker.
					t.Logf("worker retirement NOT_OBSERVED; validated cleanup authority=%t", cleanupGroup != nil)
					dir := filepath.Join(proofRoot, mode)
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Error(err)
						return
					}
					raw, _ := json.Marshal(map[string]any{"workerRetirement": "NOT_OBSERVED", "validatedCleanupAuthority": cleanupGroup != nil, "parentJoined": joined})
					if err := os.WriteFile(filepath.Join(dir, "cleanup-uncertainty.json"), raw, 0600); err != nil {
						t.Error(err)
					}
				}
			}()
			deadline := time.Now().Add(45 * time.Second)
			var pids []string
			for {
				raw, err := os.ReadFile(marker)
				if err == nil {
					pids = strings.Fields(string(raw))
					if len(pids) == 3 {
						break
					}
				}
				select {
				case err := <-done:
					joined = true
					t.Fatalf("parent exited before verifier barrier: %v %s %s", err, &stdout, &stderr)
				default:
				}
				if !time.Now().Before(deadline) {
					t.Fatal("native verifier barrier timeout")
				}
				time.Sleep(10 * time.Millisecond)
			}
			pgRaw, err := exec.Command("/bin/ps", "-p", pids[0], "-o", "pgid=").Output()
			if err != nil {
				t.Fatal(err)
			}
			observedGroup := strings.TrimSpace(string(pgRaw))
			var forbidden []string
			for _, pid := range []int{os.Getpid(), command.Process.Pid} {
				raw, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "pgid=").Output()
				if err != nil {
					t.Fatal("caller group unavailable; cleanup not armed", err)
				}
				forbidden = append(forbidden, strings.TrimSpace(string(raw)))
			}
			cleanupGroup, err = aggregateQualificationOwnedCleanup(pids[0], observedGroup, forbidden, func(owned string) error {
				return exec.Command("/bin/kill", "-KILL", "-"+owned).Run()
			})
			if err != nil {
				t.Fatal(err)
			}
			group = observedGroup
			statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
			started := 0
			_ = filepath.WalkDir(filepath.Dir(statePath), func(path string, e os.DirEntry, err error) error {
				if err == nil && !e.IsDir() && e.Name() == "check.started.json" {
					started++
				}
				return err
			})
			if started == 0 {
				t.Fatal("verifier reached without actual check-start evidence")
			}
			signalAt := time.Now()
			switch mode {
			case "INT":
				err = command.Process.Signal(os.Interrupt)
			case "TERM":
				err = command.Process.Signal(syscall.SIGTERM)
			case "KILL":
				err = command.Process.Kill()
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
				joined = true
				if err == nil {
					t.Fatal("interrupted CLI succeeded")
				}
			case <-time.After(25 * time.Second):
				t.Fatal("parent exceeded original retirement allowance")
			}
			var state struct {
				Terminal  any            `json:"terminal"`
				Aggregate map[string]any `json:"aggregateOutcome"`
			}
			raw, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &state); err != nil || state.Terminal != nil {
				t.Fatalf("success terminal after signal: %v", err)
			}
			lock := filepath.Join(root, ".git/corvint/local-completion/operation.lock")
			if mode == "KILL" {
				if _, err := os.Stat(lock); err != nil {
					t.Fatal("KILL lost retained lock", err)
				}
				before, _ := os.ReadFile(statePath)
				code, out, stderr := run.exec(t, root, nil, "dogfood", "finish", "--session-key", key, "--aggregate-outcome-profile", tracerecordrepo.AggregateOutcomeProfile)
				if code != 2 || !strings.Contains(stderr, "operation-in-progress") {
					t.Fatalf("KILL lock stolen: %d %s %s", code, out, stderr)
				}
				after, _ := os.ReadFile(statePath)
				if !bytes.Equal(before, after) {
					t.Fatal("KILL refusal changed state")
				}
			}
			destination := filepath.Join(proofRoot, mode)
			if err := os.MkdirAll(destination, 0700); err != nil {
				t.Fatal(err)
			}
			for _, prefix := range prefixes {
				if err := filepath.WalkDir(prefix, func(path string, e os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if e.IsDir() {
						return nil
					}
					relative, err := filepath.Rel(root, path)
					if err != nil {
						return err
					}
					out := filepath.Join(destination, relative)
					if err = os.MkdirAll(filepath.Dir(out), 0700); err != nil {
						return err
					}
					raw, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					return os.WriteFile(out, raw, 0600)
				}); err != nil {
					t.Fatal(err)
				}
			}
			observation := map[string]any{"mode": mode, "parentPid": command.Process.Pid, "workerPid": pids[0], "wrapperPid": pids[1], "descendantPid": pids[2], "ownedGroup": group, "elapsedAfterSignal": time.Since(signalAt).Seconds(), "startedCaptures": started, "state": state, "stdout": stdout.String(), "stderr": stderr.String()}
			if mode == "KILL" {
				observation["qualification"] = "retained refusal only; external captured-group cleanup follows"
				if err := cleanupGroup(); err != nil {
					t.Logf("external cleanup returned %v", err)
				}
			}
			deadline = time.Now().Add(5 * time.Second)
			for {
				rows, err := exec.Command("/bin/ps", "-axo", "pid=,pgid=").Output()
				if err != nil {
					t.Fatal(err)
				}
				live := []string{}
				for _, line := range strings.Split(string(rows), "\n") {
					fields := strings.Fields(line)
					if len(fields) == 2 && fields[1] == group {
						live = append(live, line)
					}
				}
				if len(live) == 0 {
					observation["survivors"] = live
					break
				}
				if !time.Now().Before(deadline) {
					observation["survivors"] = live
					encoded, _ := json.MarshalIndent(observation, "", "  ")
					_ = os.WriteFile(filepath.Join(destination, "observation.json"), encoded, 0600)
					t.Fatalf("owned descendants survived %s: %v", mode, live)
				}
				time.Sleep(20 * time.Millisecond)
			}
			group = "" // absence observed; never signal this numeric group again.
			cleanupGroup = nil
			retirementObserved = true
			if mode != "KILL" {
				if _, err := os.Stat(lock); !os.IsNotExist(err) {
					t.Fatalf("retired CLI kept lock: %v", err)
				}
			}
			encoded, _ := json.MarshalIndent(observation, "", "  ")
			if err := os.WriteFile(filepath.Join(destination, "observation.json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
	restore()
}

func aggregateQualificationStaleCommitted(t *testing.T, run portableRun, root, key string) {
	t.Helper()
	statePath := filepath.Join(root, ".git/corvint/local-completion", key, "state.json")
	original, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	target := cemGit(t, root, "rev-parse", "HEAD")
	for _, kind := range []string{"target", "check-observation", "review-binding"} {
		t.Run("qualification-stale/"+kind, func(t *testing.T) {
			defer func() {
				if kind == "target" {
					cemGit(t, root, "reset", "--hard", target)
				}
				if err := os.WriteFile(statePath, original, 0600); err != nil {
					t.Error(err)
				}
			}()
			var value map[string]any
			if err := json.Unmarshal(original, &value); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "target":
				cemWrite(t, root, "aggregate/f000.go", "package aggregate\nconst Changed=1\n")
				cemGit(t, root, "add", "aggregate/f000.go")
				cemGit(t, root, "commit", "-qm", "stale committed target")
			case "check-observation":
				observations := value["observations"].([]any)
				if len(observations) == 0 {
					t.Fatal("missing actual check observation")
				}
				observations[0].(map[string]any)["checkDigest"] = strings.Repeat("f", 64)
			case "review-binding":
				value["review"] = strings.Repeat("f", 64)
			}
			if kind != "target" {
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(statePath, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(statePath)
			code, out, stderr := run.exec(t, root, nil, "dogfood", "status", "--session-key", key)
			if code == 0 {
				var evaluation struct {
					Policy struct {
						Satisfied bool `json:"satisfied"`
					} `json:"policy"`
				}
				if err := json.Unmarshal([]byte(out), &evaluation); err != nil || evaluation.Policy.Satisfied {
					t.Fatalf("stale committed accepted: %v %s", err, out)
				}
			} else if code != 2 {
				t.Fatalf("unexpected stale read: %d %s %s", code, out, stderr)
			}
			after, _ := os.ReadFile(statePath)
			if !bytes.Equal(before, after) {
				t.Fatal("stale reader changed immutable state")
			}
		})
	}
}

// Observed text is never cleanup authority until all identity checks pass.
func aggregateQualificationOwnedCleanup(worker, observed string, forbidden []string, signal func(string) error) (func() error, error) {
	valid := func(raw string) bool {
		n, err := strconv.Atoi(raw)
		return err == nil && n > 1 && strconv.Itoa(n) == raw
	}
	if !valid(worker) || !valid(observed) || worker != observed || len(forbidden) < 2 || signal == nil {
		return nil, fmt.Errorf("verifier group identity not owned: worker=%q group=%q", worker, observed)
	}
	for _, caller := range forbidden {
		if !valid(caller) || observed == caller {
			return nil, fmt.Errorf("caller group excluded or unobserved: %q", caller)
		}
	}
	return func() error { return signal(observed) }, nil
}

func TestAggregateQualificationOwnedCleanup(t *testing.T) {
	for _, tc := range []struct {
		name, worker, observed string
		callers                []string
		accepted               bool
	}{
		{"owned", "300", "300", []string{"100", "200"}, true},
		{"foreign", "300", "400", []string{"100", "200"}, false},
		{"harness", "100", "100", []string{"100", "200"}, false},
		{"caller", "200", "200", []string{"100", "200"}, false},
		{"malformed", "300", "garbage", []string{"100", "200"}, false},
		{"negative", "-300", "-300", []string{"100", "200"}, false},
		{"zero", "0", "0", []string{"100", "200"}, false},
		{"one", "1", "1", []string{"100", "200"}, false},
		{"noncanonical", "0300", "0300", []string{"100", "200"}, false},
		{"prebarrier", "", "", []string{"100", "200"}, false},
		{"caller-unobserved", "300", "300", []string{"100", ""}, false},
		{"missing-callers", "300", "300", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var signals []string
			cleanup, err := aggregateQualificationOwnedCleanup(tc.worker, tc.observed, tc.callers, func(group string) error { signals = append(signals, group); return nil })
			if (err == nil) != tc.accepted || (cleanup != nil) != tc.accepted {
				t.Fatalf("cleanup admission: %v", err)
			}
			// Exercise the same conditional cleanup used by the failure defer.
			if cleanup != nil {
				if err := cleanup(); err != nil {
					t.Fatal(err)
				}
			}
			if tc.accepted {
				if !reflect.DeepEqual(signals, []string{tc.observed}) {
					t.Fatal(signals)
				}
			} else if len(signals) != 0 {
				t.Fatal("rejected group signalled", signals)
			}
		})
	}
}
