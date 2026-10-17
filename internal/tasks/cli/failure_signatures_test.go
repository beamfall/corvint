package cli_test

import (
	"os"
	"path/filepath"
	"strings"
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
		field(am, "status").Str != "RECORDED" || field(am, "count").Str != "1" {
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
