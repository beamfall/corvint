package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ledgerDir is the failure-signature ledger beside the preflight cache.
func ledgerDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(filepath.Dir(privateCache(t)), "failure-signatures")
}

func signatureMember(t *testing.T, x run) wire.Value {
	t.Helper()
	if len(x.res.Items) != 1 {
		t.Fatalf("no item: %s", x.stdout)
	}
	v, ok := x.res.Items[0].Obj.Get("failureSignature")
	if !ok || v.Kind != wire.KindObject {
		t.Fatalf("no failureSignature member: %s", x.stdout)
	}
	return v
}

func auditLines(t *testing.T, dir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "remediations.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// TestTOLV0051_FailureLedgerIsOffByDefault: without --refuse-repeat a
// failing preflight repeats freely, carries no failureSignature member and
// creates no ledger; the flags are validated; with it, a failure records
// the signature and count at the input digest and a pass clears it.
func TestTOLV0051_FailureLedgerIsOffByDefault(t *testing.T) {
	dir := ledgerDir(t)
	r, id, _ := preflightRepo(t, preflightSpec, preflightSeed)
	for i := 0; i < 3; i++ {
		x := atm(t, r.Root, nil, "preflight", id)
		if x.res.Outcome != wire.OutcomeRefused || hasCode(x.res, wire.CodeLoopDetected) {
			t.Fatalf("run %d without --refuse-repeat: %s", i, x.stdout)
		}
		if _, ok := x.res.Items[0].Obj.Get("failureSignature"); ok {
			t.Fatalf("a run without --refuse-repeat carries the member: %s", x.stdout)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the ledger exists without --refuse-repeat: %v", err)
	}
	for name, args := range map[string][]string{
		"remediation alone": {"--remediation", "fix"},
		"zero":              {"--refuse-repeat", "0"},
		"too many":          {"--refuse-repeat", "21"},
		"not canonical":     {"--refuse-repeat", "01"},
		"multi-line ref":    {"--refuse-repeat", "1", "--remediation", "fix\nmore"},
		"blank ref":         {"--refuse-repeat", "1", "--remediation", "  "},
	} {
		if x := atm(t, r.Root, nil, append([]string{"preflight", id}, args...)...); x.res.Outcome != wire.OutcomeError || len(x.res.Items) != 0 {
			t.Fatalf("%s: %s", name, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1", "--remediation", "token AKIAABCDEFGHIJKLMNOP"); !hasCode(x.res, wire.CodeSecretDetected) || strings.Contains(string(x.stdout), "AKIAABCDEFGHIJKLMNOP") {
		t.Fatalf("secret remediation: %s", x.stdout)
	}
	first := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "3")
	m := signatureMember(t, first)
	if field(m, "status").Str != "RECORDED" || field(m, "count").Str != "1" || field(m, "threshold").Str != "3" ||
		len(field(m, "signature").Str) != 64 || len(field(m, "inputDigest").Str) != 64 || field(m, "remediation").Kind != wire.KindNull {
		t.Fatalf("first failure: %s", first.stdout)
	}
	second := signatureMember(t, atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "3"))
	if field(second, "count").Str != "2" || field(second, "signature").Str != field(m, "signature").Str {
		t.Fatalf("same failure did not count: %s", wire.Encode(second))
	}
	// A clean ledger passes and clears its own entry.
	clean, cid, _ := preflightRepo(t, cleanSpec, cleanSeed)
	pass := signatureMember(t, atm(t, clean.Root, nil, "preflight", cid, "--refuse-repeat", "1"))
	if field(pass, "status").Str != "CLEARED" || field(pass, "count").Str != "0" || field(pass, "signature").Kind != wire.KindNull {
		t.Fatalf("pass: %s", wire.Encode(pass))
	}
}

// TestTOLV0052_RepeatedSignatureRefusesNamingSignatureAndCount: after N
// failures with one signature at one input digest the next run refuses
// LOOP_DETECTED naming the signature, the count and the digest, without
// running the checks; a changed input digest admits the run.
func TestTOLV0052_RepeatedSignatureRefusesNamingSignatureAndCount(t *testing.T) {
	ledgerDir(t)
	r, id, _ := preflightRepo(t, preflightSpec, preflightSeed)
	var m wire.Value
	for i := 0; i < 2; i++ {
		m = signatureMember(t, atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "2"))
	}
	x := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "2")
	text := strings.Join(x.res.Warnings, " ")
	if !hasCode(x.res, wire.CodeLoopDetected) || len(x.res.Items) != 0 || !strings.Contains(text, "FAILURE_SIGNATURE_REPEATED:") ||
		!strings.Contains(text, "failed 2 times") || !strings.Contains(text, field(m, "signature").Str) || !strings.Contains(text, field(m, "inputDigest").Str) {
		t.Fatalf("repeat refusal: %s", x.stdout)
	}
	// An untracked file changes the input digest; the run is admitted and
	// starts a new count there.
	fixture.Write(t, filepath.Join(r.Root, "notes.txt"), []byte("investigating\n"))
	moved := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "2")
	mm := signatureMember(t, moved)
	if moved.res.Outcome != wire.OutcomeRefused || hasCode(moved.res, wire.CodeLoopDetected) || field(mm, "count").Str != "1" || field(mm, "inputDigest").Str == field(m, "inputDigest").Str {
		t.Fatalf("changed input: %s", moved.stdout)
	}
}

// TestTOLV0053_RemediationClearsOnceWithAudit: a declared remediation
// admits one run, appending an audit record before it runs; the same
// reference is refused afterwards; a run it admits bypasses a cache replay.
func TestTOLV0053_RemediationClearsOnceWithAudit(t *testing.T) {
	dir := ledgerDir(t)
	r, id, _ := preflightRepo(t, preflightSpec, preflightSeed)
	first := signatureMember(t, atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1", "--cache"))
	if x := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1", "--cache"); !hasCode(x.res, wire.CodeLoopDetected) {
		t.Fatalf("second run: %s", x.stdout)
	}
	admitted := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1", "--cache", "--remediation", "fixed in abc123")
	am := signatureMember(t, admitted)
	if admitted.res.Outcome != wire.OutcomeRefused || cacheStatus(t, admitted) != "MISS" || field(am, "remediation").Str != "fixed in abc123" ||
		field(am, "status").Str != "RECORDED" || field(am, "count").Str != "2" {
		t.Fatalf("remediation run: %s", admitted.stdout)
	}
	lines := auditLines(t, dir)
	if len(lines) != 1 || !strings.Contains(lines[0], `"remediation":"fixed in abc123"`) || !strings.Contains(lines[0], field(first, "signature").Str) ||
		!strings.Contains(lines[0], `"verb":"preflight"`) || !strings.Contains(lines[0], `"count":"1"`) {
		t.Fatalf("audit: %q", lines)
	}
	if x := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1"); !hasCode(x.res, wire.CodeLoopDetected) {
		t.Fatalf("after the remediated failure: %s", x.stdout)
	}
	reused := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1", "--remediation", "fixed in abc123")
	if !hasCode(reused.res, wire.CodeLoopDetected) || !strings.Contains(strings.Join(reused.res.Warnings, " "), "FAILURE_REMEDIATION_REUSED:") || len(auditLines(t, dir)) != 1 {
		t.Fatalf("reused remediation: %s", reused.stdout)
	}
	if x := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1", "--remediation", "fixed in def456"); x.res.Outcome != wire.OutcomeRefused || hasCode(x.res, wire.CodeLoopDetected) || len(auditLines(t, dir)) != 2 {
		t.Fatalf("second remediation: %s", x.stdout)
	}
}

// TestTOLV0054_LedgerStaysPrivateAndFailsOpenToRunning: the ledger lives
// outside the repository and the store, and an input digest that cannot be
// computed runs the checks unguarded with status SKIPPED and the reason.
func TestTOLV0054_LedgerStaysPrivateAndFailsOpenToRunning(t *testing.T) {
	dir := ledgerDir(t)
	r, id, _ := preflightRepo(t, preflightSpec, preflightSeed)
	status := gitIn(t, r.Root, "", "status", "--porcelain", "--ignored")
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	signatureMember(t, atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1"))
	if got := gitIn(t, r.Root, "", "status", "--porcelain", "--ignored"); got != status {
		t.Fatalf("the ledger changed the repository:\n%s\n%s", status, got)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("the ledger changed store or intent state")
	}
	if st, err := os.Stat(dir); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("ledger directory: %v %v", st, err)
	}
	defer cli.SetImplementationDigest(func() (string, error) { return "", os.ErrPermission })()
	for i := 0; i < 2; i++ {
		x := atm(t, r.Root, nil, "preflight", id, "--refuse-repeat", "1")
		m := signatureMember(t, x)
		if x.res.Outcome != wire.OutcomeRefused || hasCode(x.res, wire.CodeLoopDetected) || field(m, "status").Str != "SKIPPED" || field(m, "reason").Kind != wire.KindString {
			t.Fatalf("unkeyable run %d: %s", i, x.stdout)
		}
	}
}

// TestTOLV0052_RunBatchRefusesRepeatedSignature: run-batch with
// --refuse-repeat refuses a repeated failure signature at unchanged inputs
// before the test command runs or a results directory is written, even
// across different --out directories that TOL-V0-031 does not compare.
func TestTOLV0052_RunBatchRefusesRepeatedSignature(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	ledgerDir(t)
	r, id := obligationBatchRepo(t)
	test, counter := rbTools(t)
	config := rbConfig(t, test, nil)
	runs := 0
	var last wire.Value
	for i, request := range []string{"b1", "b2", "b3"} {
		out := filepath.Join(t.TempDir(), "out")
		x := atm(t, r.Root, nil, runBatchArgs(id, config, out, request, "--refuse-repeat", "1")...)
		if hasCode(x.res, wire.CodeLoopDetected) {
			text := strings.Join(x.res.Warnings, " ")
			if i == 0 || !strings.Contains(text, "FAILURE_SIGNATURE_REPEATED: run-batch failed 1 times") || !strings.Contains(text, field(last, "signature").Str) {
				t.Fatalf("%s refusal: %s", request, x.stdout)
			}
			if rbRuns(t, counter) != runs {
				t.Fatal("the refused job ran the test command")
			}
			if _, err := os.Stat(filepath.Join(out, request)); !os.IsNotExist(err) {
				t.Fatalf("the refused job wrote evidence: %v", err)
			}
			return
		}
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s: %s", request, x.stdout)
		}
		runs++
		last = signatureMember(t, x)
		if field(last, "status").Str != "RECORDED" || field(last, "count").Str != "1" {
			t.Fatalf("%s member: %s", request, x.stdout)
		}
	}
	t.Fatal("three identical failing batches were never refused")
}

const guardInput = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func guardCode(err error) string {
	var e *wire.Error
	if errors.As(err, &e) {
		return string(e.Code)
	}
	return ""
}

// TestTOLV0053_RemediationAllowsOneRunAtAnyThreshold: at --refuse-repeat
// 3 a remediation admits exactly one run; its identical failure refuses
// the next run at once rather than allowing N more.
func TestTOLV0053_RemediationAllowsOneRunAtAnyThreshold(t *testing.T) {
	ledgerDir(t)
	lines := []string{"AC-1\tFAILED\tError: total was 0"}
	for i := 1; i <= 3; i++ {
		if st, n, err := cli.FailureGuardRun(guardInput, 3, "", true, lines); err != nil || st != "RECORDED" || n != strconv.Itoa(i) {
			t.Fatalf("failure %d: %s %s %v", i, st, n, err)
		}
	}
	if _, _, err := cli.FailureGuardRun(guardInput, 3, "", true, lines); guardCode(err) != string(wire.CodeLoopDetected) {
		t.Fatalf("fourth run: %v", err)
	}
	if st, n, err := cli.FailureGuardRun(guardInput, 3, "fixed in abc123", true, lines); err != nil || st != "RECORDED" || n != "4" {
		t.Fatalf("remediated run: %s %s %v", st, n, err)
	}
	if _, _, err := cli.FailureGuardRun(guardInput, 3, "", true, lines); guardCode(err) != string(wire.CodeLoopDetected) || !strings.Contains(err.Error(), "FAILURE_SIGNATURE_REPEATED:") {
		t.Fatalf("the run after the remediated failure was admitted: %v", err)
	}
	if _, _, err := cli.FailureGuardRun(guardInput, 3, "fixed in abc123", true, lines); guardCode(err) != string(wire.CodeLoopDetected) || !strings.Contains(err.Error(), "FAILURE_REMEDIATION_REUSED:") {
		t.Fatalf("a spent remediation was admitted: %v", err)
	}
}

// TestTOLV0051_SignatureKeepsStructuralIDs: normalization folds durations,
// ports and line numbers but keeps identifier digits, so alternating
// failures of AC-1 and AC-2 are different signatures that never
// accumulate one count.
func TestTOLV0051_SignatureKeepsStructuralIDs(t *testing.T) {
	a := cli.NormalizeFailure("AC-1\tFAILED\tTimeout 5000ms exceeded at localhost:3000 (e2e/cart.spec.ts:12:5)")
	if b := cli.NormalizeFailure("AC-1  FAILED Timeout 7000ms exceeded at localhost:3001 (e2e/cart.spec.ts:14:9)"); a != b {
		t.Fatalf("run-specific numbers split one signature: %q %q", a, b)
	}
	if c := cli.NormalizeFailure("AC-2\tFAILED\tTimeout 5000ms exceeded at localhost:3000 (e2e/cart.spec.ts:12:5)"); a == c {
		t.Fatalf("AC-1 and AC-2 normalize alike: %q", a)
	}
	if !strings.Contains(a, "AC-1") || !strings.Contains(a, "e2e/cart.spec.ts") || strings.Contains(a, "5000") {
		t.Fatalf("normalized: %q", a)
	}
	if x, y := cli.NormalizeFailure("V1-1605 failed"), cli.NormalizeFailure("V1-1606 failed"); x == y {
		t.Fatalf("ticket ids normalize alike: %q", x)
	}
	ledgerDir(t)
	for i := 0; i < 4; i++ {
		id := []string{"AC-1", "AC-2"}[i%2]
		if st, n, err := cli.FailureGuardRun(guardInput, 2, "", true, []string{id + "\tFAILED\tError: timeout"}); err != nil || st != "RECORDED" || n != "1" {
			t.Fatalf("alternating failure %d (%s): %s %s %v", i, id, st, n, err)
		}
	}
}

// TestTOLV0054_ConcurrentRunsSerializeTheLedger: concurrent failures at one
// input digest each count once, and one remediation reference given to
// two concurrent runs admits exactly one of them.
func TestTOLV0054_ConcurrentRunsSerializeTheLedger(t *testing.T) {
	ledgerDir(t)
	lines := []string{"AC-1\tFAILED\tError: total was 0"}
	const runs = 8
	var wg sync.WaitGroup
	errs := make(chan error, runs)
	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := cli.FailureGuardRun(guardInput, 20, "", true, lines); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent run: %v", err)
	}
	if st, n, err := cli.FailureGuardRun(guardInput, 20, "", true, lines); err != nil || st != "RECORDED" || n != "9" {
		t.Fatalf("after %d concurrent failures: %s %s %v (lost updates)", runs, st, n, err)
	}
	other := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	if _, _, err := cli.FailureGuardRun(other, 1, "", true, lines); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	admitted, reused := 0, 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := cli.FailureGuardRun(other, 1, "fixed in abc123", true, lines)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				admitted++
			case strings.Contains(err.Error(), "FAILURE_REMEDIATION_REUSED:"):
				reused++
			}
		}()
	}
	wg.Wait()
	if admitted != 1 || reused != 1 {
		t.Fatalf("one remediation given twice: admitted %d, reused %d", admitted, reused)
	}
}

// TestTOLV0051_RunBatchDigestCoversEnvironmentAndPool: the run-batch input
// digest moves with the value of a passed environment variable, with its
// presence and with the pool, so a corrected BASE_URL or another pool is
// a new input.
func TestTOLV0051_RunBatchDigestCoversEnvironmentAndPool(t *testing.T) {
	env := []string{"CORVINT_TOL_BASE_URL"}
	digest := func(pool string) string {
		t.Helper()
		d, reason := cli.RunBatchInputDigest(pool, env)
		if reason != "" || d == "" {
			t.Fatalf("digest: %q %q", d, reason)
		}
		return d
	}
	t.Setenv("CORVINT_TOL_BASE_URL", "http://localhost:3000")
	a := digest("")
	if again := digest(""); again != a {
		t.Fatal("the digest is not stable")
	}
	t.Setenv("CORVINT_TOL_BASE_URL", "http://localhost:4000")
	b := digest("")
	os.Unsetenv("CORVINT_TOL_BASE_URL")
	unset := digest("")
	t.Setenv("CORVINT_TOL_BASE_URL", "http://localhost:3000")
	pooled := digest("lanes")
	if a == b || a == unset || b == unset || a == pooled {
		t.Fatalf("digest ignores an input: value %v, presence %v, pool %v", a == b, a == unset || b == unset, a == pooled)
	}
}
