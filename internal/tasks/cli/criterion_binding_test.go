package cli_test

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCB-V0-003/004/006: verify is offline, carries no live outer snapshot,
// preserves producer independently of verifier, and uses a strict stdin contract.
func TestCriterionBindingOfflineVerify(t *testing.T) {
	raw, e := os.ReadFile("../../criterionexperiment/testdata/capture.json")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	before, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	got := atm(t, filepath.Join(dir, "absent"), raw, "criterion-binding", "verify")
	if got.code != 0 || got.res.Snapshot != nil {
		t.Fatalf("offline: %s", got.stdout)
	}
	v, e := wire.ReadCriterionVerification(got.res.Items[0])
	if e != nil {
		t.Fatal(e)
	}
	if v.Producer == v.Verifier || v.CaptureSHA256 != wire.Sum(raw) || v.Binding.Snapshot.HeadSeq != "1" {
		t.Fatal("producer/verifier/historical binding")
	}
	after, e := os.ReadDir(dir)
	if e != nil || len(before) != len(after) {
		t.Fatal("offline verify mutated cwd")
	}
	for _, input := range [][]byte{nil, append(raw, '\n'), bytes.Repeat([]byte("x"), wire.MaxCriterionCaptureBytes+1)} {
		if x := atm(t, dir, input, "criterion-binding", "verify"); x.code == 0 {
			t.Fatal("bad framing admitted")
		}
	}
	for _, args := range [][]string{{"verify", "--file", "x"}, {"capture", "--ticket", "acme:main:AT-0001", "--ticket", "acme:main:AT-0001", "--attempt", "x"}, {"capture", "--attempt", "x"}, {"capture", "--ticket", "AT-0001", "--attempt", "x"}} {
		if x := atm(t, dir, raw, append([]string{"criterion-binding"}, args...)...); x.code == 0 {
			t.Fatalf("bad flags admitted: %v", args)
		}
	}
}

// A real native REFINE after CLAIM changes the current file digest while the
// accepted criteria and the attempt's original ticket digest remain valid.
func TestCriterionCaptureRetainsClaimAcrossNativeBodyEdit(t *testing.T) {
	r := fixture.TempRepo(t)
	root := r.Root
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-qm", "base")
	if x := atm(t, root, nil, "init"); x.code != 0 {
		t.Fatalf("init: %s", x.stdout)
	}
	// This synthetic qualification input exercises
	// the native cutover parser and is not deployment qualification evidence.
	var qualification strings.Builder
	qualification.WriteString(`{"Action":"start","Package":"` + transaction.QualificationPackage + `"}` + "\n")
	for _, name := range transaction.QualificationSuite {
		for _, action := range []string{"run", "pass"} {
			qualification.WriteString(`{"Action":"` + action + `","Package":"` + transaction.QualificationPackage + `","Test":"` + name + `"}` + "\n")
		}
	}
	qualification.WriteString(`{"Action":"pass","Package":"` + transaction.QualificationPackage + `"}` + "\n")
	qualificationPath := filepath.Join(t.TempDir(), "qualification.jsonl")
	if e := os.WriteFile(qualificationPath, []byte(qualification.String()), 0600); e != nil {
		t.Fatal(e)
	}
	qualificationPath, _ = filepath.EvalSymlinks(qualificationPath)
	cutover := atm(t, root, nil, "cutover", "--execution", "--decision", "test-criterion-capture", "--qualification", qualificationPath)
	if cutover.code != 0 {
		t.Fatalf("test cutover: %s", cutover.stdout)
	}
	created := atm(t, root, nil, "ticket", "create", "--request-id", "create-body-proof", "--payload", createPayloadJSON)
	if created.code != 0 {
		t.Fatalf("create: %s", created.stdout)
	}
	ticketID := field(created.res.Items[0], "ticketId").Str
	admitted := atm(t, root, nil, "claim", ticketID, "--holder", "test", "--request-id", "claim-body-proof", "--scope", "src/")
	if admitted.code != 0 {
		t.Fatalf("claim: %s", admitted.stdout)
	}
	attemptID := field(admitted.res.Items[0], "attemptId").Str
	get := func() wire.CriterionCapture {
		t.Helper()
		x := atm(t, root, nil, "criterion-binding", "capture", "--ticket", ticketID, "--attempt", attemptID)
		if x.code != 0 {
			t.Fatalf("capture: %s", x.stdout)
		}
		c, _, e := wire.ReadCriterionCaptureResult(x.res.Items[0])
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	before := get()
	x := atm(t, root, nil, "ticket", "refine", "--request-id", "body-after-claim", "--target", ticketID, "--expected-revision", "1", "--payload", `{"body":"Native body-only edit after claim"}`)
	if x.code != 0 {
		t.Fatalf("native refine: %s", x.stdout)
	}
	after := get()
	if before.ClaimedTicket != after.ClaimedTicket || before.Ticket == after.Ticket {
		t.Fatal("claim-era bytes not retained across edit")
	}
	verified := atm(t, filepath.Join(t.TempDir(), "absent"), wire.EncodeFile(after.Value()), "criterion-binding", "verify")
	if verified.code != 0 {
		t.Fatalf("offline current+claim replay: %s", verified.stdout)
	}
	audit := atm(t, root, nil, "receipt", "audit")
	if audit.code != 0 {
		t.Fatalf("audit: %s", audit.stdout)
	}
}
