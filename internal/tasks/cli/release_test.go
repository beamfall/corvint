package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func releaseCreatePayload(version, title, ticketID, predecessor string) string {
	preds := "[]"
	if predecessor != "" {
		preds = fmt.Sprintf(`[%q]`, predecessor)
	}
	return fmt.Sprintf(`{"acceptanceCriteria":["release criterion"],"predecessorReleaseIds":%s,"requiredGates":[],"ticketIds":[%q],"title":%q,"version":%q}`, preds, ticketID, title, version)
}

func TestTMV0028_AS38_DurableMultiTicketCandidateDefinitionOrder(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	git(t, r.Root, "init")
	git(t, r.Root, "config", "user.email", "fixture@example.invalid")
	git(t, r.Root, "config", "user.name", "Fixture")
	git(t, r.Root, "add", ".taskman")
	git(t, r.Root, "commit", "-m", "baseline")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	var first, second string
	var highest wire.Digest
	for i := 0; i < 32; i++ {
		id := createTicket(t, r.Root, fmt.Sprintf("multi-ticket-%d", i))
		raw, err := os.ReadFile(filepath.Join(r.IntentDir, "tickets", id[len(id)-7:]+".json"))
		if err != nil {
			t.Fatal(err)
		}
		d := wire.Sum(raw)
		if first != "" && d < highest {
			second = id
			break
		}
		first, highest = id, d
	}
	if second == "" {
		t.Fatal("could not construct opposite digest/ID order")
	}
	payload := strings.Replace(releaseCreatePayload("v1", "Release", first, ""), fmt.Sprintf(`"ticketIds":[%q]`, first), fmt.Sprintf(`"ticketIds":[%q,%q]`, first, second), 1)
	if x := atm(t, r.Root, nil, "release", "create", "--request-id", "multi-release", "--target", "v1", "--payload", payload); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	args := []string{"release", "candidate", "--request-id", "multi-candidate", "--target", "v1", "--expected-revision", "1", "--issued-at", "2026-09-20T12:00:00Z"}
	if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	raw, err := os.ReadFile(filepath.Join(r.IntentDir, "releases", "v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := release.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	bindings := record.Candidate.Tickets
	if len(bindings) != 2 || bindings[0].TicketID.Raw != first || bindings[1].TicketID.Raw != second || bindings[0].RecordSha256 <= bindings[1].RecordSha256 || string(release.Encode(record)) != string(raw) {
		t.Fatalf("candidate order/round-trip: %+v", bindings)
	}
	if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatal(x.res)
	}
}

func TestTMV0028_AS38_ReleaseKeepJournalReconciliation(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	id := createTicket(t, r.Root, "release-reconcile-ticket")
	payload := releaseCreatePayload("0-9", "Version 0.9", id, "")
	if x := atm(t, r.Root, nil, "release", "create", "--request-id", "release-reconcile-create", "--target", "v0-9", "--issued-at", "2026-09-20T12:01:00Z", "--payload", payload); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	projection := filepath.Join(r.IntentDir, "releases", "v0-9.json")
	edited := []byte("{manual malformed edit\n")
	fixture.Write(t, projection, edited)
	inspect := atm(t, r.Root, nil, "reconcile", "inspect", "v0-9")
	if inspect.res.Outcome != wire.OutcomeOK {
		t.Fatalf("inspect: %+v", inspect.res)
	}
	digest := field(inspect.res.Items[0], "canonicalRecordSha256").Str
	copyPath := filepath.Join(r.Root, "original-release-edit.json")
	if err := os.WriteFile(copyPath, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	kept := atm(t, r.Root, nil, "reconcile", "intent", "--request-id", "release-reconcile-keep", "--target", "v0-9", "--file", copyPath, "--keep-journal", "--canonical-sha256", digest)
	if kept.res.Outcome != wire.OutcomeOK {
		t.Fatalf("keep: %+v", kept.res)
	}
	show := atm(t, r.Root, nil, "release", "show", "v0-9")
	if show.res.Outcome != wire.OutcomeOK || field(show.res.Items[0], "revision").Str != "1" {
		t.Fatalf("show: %+v", show.res)
	}
	adopt := atm(t, r.Root, nil, "reconcile", "intent", "--request-id", "release-reconcile-adopt", "--target", "v0-9", "--file", copyPath, "--adopt-file")
	if adopt.res.Outcome == wire.OutcomeOK {
		t.Fatal("release ADOPT_FILE succeeded")
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func createTicket(t *testing.T, root, request string) string {
	t.Helper()
	x := atm(t, root, nil, "ticket", "create", "--request-id", request, "--issued-at", "2026-09-20T12:00:00Z", "--payload", createPayloadJSON)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", x.res)
	}
	return field(x.res.Items[0], "ticketId").Str
}

func completeReleaseTicket(t *testing.T, root, request, ticketID string) {
	t.Helper()
	x := atm(t, root, nil, "ticket", "complete-manual", "--request-id", request, "--target", ticketID, "--expected-revision", "1", "--issued-at", "2026-09-20T12:01:00Z", "--payload", `{"evidence":[],"reason":"release accepted"}`)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket complete: %+v", x.res)
	}
}

func publicReleaseCandidate(t *testing.T, root, releaseID string) (revision, digest string, item wire.Value) {
	t.Helper()
	x := atm(t, root, nil, "release", "show", releaseID)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("release show %s: %+v", releaseID, x.res)
	}
	item = x.res.Items[0]
	revision = field(item, "revision").Str
	digestValue := field(item, "candidateSha256")
	if digestValue.Kind != wire.KindString {
		t.Fatalf("release show %s omitted candidate digest: %+v", releaseID, item)
	}
	return revision, digestValue.Str, item
}

func recordPublicReleaseGate(t *testing.T, root, releaseID, revision, candidate, request string) {
	t.Helper()
	evidence := wire.Sum([]byte("public workflow evidence for " + releaseID))
	payload := fmt.Sprintf(`{"attestation":{"actor":"external-ci","attestationId":%q,"candidateSha256":%q,"criteria":["0"],"evidence":[%q],"gateId":"verify","profile":"taskman-release-attestation/0","provenance":"EXTERNAL_ATTESTATION","recordedAt":"2026-09-20T12:04:00Z","result":"PASS","sourceIdentity":"external-ci"}}`, request, candidate, evidence)
	x := atm(t, root, nil, "release", "record-gate", "--request-id", request, "--target", releaseID, "--expected-revision", revision, "--issued-at", "2026-09-20T12:04:00Z", "--payload", payload)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("record gate %s: %+v", releaseID, x.res)
	}
}

// TM-V0-028 and TM-V0-029 / AS-38: public output drives both ordered promotions.
func TestTMV0028_AS38_PublicOutputDrivesOrderedReleasePromotion(t *testing.T) {
	t.Run("TM-V0-028 TM-V0-029 public output drives both ordered promotions", testPublicOutputDrivesOrderedReleasePromotion)
}

func testPublicOutputDrivesOrderedReleasePromotion(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.Root, "source.txt"), []byte("public release workflow\n"))
	git(t, r.Root, "init")
	git(t, r.Root, "config", "user.email", "fixture@example.invalid")
	git(t, r.Root, "config", "user.name", "Fixture")
	git(t, r.Root, "add", ".taskman", "source.txt")
	git(t, r.Root, "commit", "-m", "fixture baseline")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}

	t09 := createTicket(t, r.Root, "public-ticket-v0-9")
	t10 := createTicket(t, r.Root, "public-ticket-v1-0")
	for _, releaseCase := range []struct {
		id, version, title, ticket, predecessor, request string
	}{
		{"v0-9", "0-9", "Version 0.9", t09, "", "public-release-v0-9"},
		{"v1-0", "1-0", "Version 1.0", t10, "v0-9", "public-release-v1-0"},
	} {
		x := atm(t, r.Root, nil, "release", "create", "--request-id", releaseCase.request, "--target", releaseCase.id, "--issued-at", "2026-09-20T12:02:00Z", "--payload", releaseCreatePayload(releaseCase.version, releaseCase.title, releaseCase.ticket, releaseCase.predecessor))
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release create %s: %+v", releaseCase.id, x.res)
		}
	}
	list := atm(t, r.Root, nil, "release", "list")
	if list.res.Outcome != wire.OutcomeOK || len(list.res.Items) != 2 {
		t.Fatalf("release list: %+v", list.res)
	}
	_, listHasCandidateDigest := list.res.Items[0].Obj.Get("candidateSha256")
	if listHasCandidateDigest {
		t.Fatalf("release list is not bounded summary output: %+v", list.res)
	}
	before := atm(t, r.Root, nil, "release", "show", "v0-9")
	if before.res.Outcome != wire.OutcomeOK || field(before.res.Items[0], "candidateSha256").Kind != wire.KindNull || field(before.res.Items[0], "candidateBinding").Kind != wire.KindNull || field(before.res.Items[0], "promotionSha256").Kind != wire.KindNull || field(before.res.Items[0], "promotion").Kind != wire.KindNull {
		t.Fatalf("uncaptured release detail: %+v", before.res)
	}

	completeReleaseTicket(t, r.Root, "public-complete-v0-9", t09)
	if x := atm(t, r.Root, nil, "release", "candidate", "--request-id", "public-candidate-v0-9", "--target", "v0-9", "--expected-revision", "1", "--issued-at", "2026-09-20T12:03:00Z"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("candidate v0-9: %+v", x.res)
	}
	revision09, candidate09, show09 := publicReleaseCandidate(t, r.Root, "v0-9")
	if field(show09, "ticketIds").Arr[0].Str != t09 || len(field(show09, "acceptanceCriteria").Arr) != 1 || len(field(show09, "requiredGates").Arr) != 0 || field(field(show09, "candidateBinding"), "tickets").Arr[0].Obj == nil {
		t.Fatalf("incomplete release definition or candidate binding: %+v", show09)
	}
	recordPublicReleaseGate(t, r.Root, "v0-9", revision09, candidate09, "public-gate-v0-9")
	ready09 := atm(t, r.Root, nil, "release", "readiness", "v0-9")
	if ready09.res.Outcome != wire.OutcomeOK || field(ready09.res.Items[0], "readiness").Str != release.ReadyAttested || len(field(ready09.res.Items[0], "attestations").Arr) != 1 {
		t.Fatalf("public readiness v0-9: %+v", ready09.res)
	}
	attestation09 := field(ready09.res.Items[0], "attestations").Arr[0]
	attestationDigest09 := field(attestation09, "attestationSha256").Str
	if field(attestation09, "profile").Str != release.AttestationProfile || field(attestation09, "candidateSha256").Str != candidate09 || len(field(attestation09, "evidence").Arr) != 1 || attestationDigest09 == "" {
		t.Fatalf("public attestation v0-9: %+v", attestation09)
	}
	revision09 = field(ready09.res.Items[0], "revision").Str
	if x := atm(t, r.Root, nil, "release", "promote", "--request-id", "public-promote-v0-9", "--target", "v0-9", "--expected-revision", revision09, "--issued-at", "2026-09-20T12:05:00Z"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("promote v0-9: %+v", x.res)
	}
	promoted09 := atm(t, r.Root, nil, "release", "show", "v0-9")
	promotion09 := field(promoted09.res.Items[0], "promotionSha256").Str
	promotionDetail09 := field(promoted09.res.Items[0], "promotion")
	if promoted09.res.Outcome != wire.OutcomeOK || promotion09 == "" || field(promotionDetail09, "candidateSha256").Str != candidate09 || field(promotionDetail09, "attestationSha256s").Arr[0].Str != attestationDigest09 {
		t.Fatalf("public promotion v0-9: %+v", promoted09.res)
	}

	completeReleaseTicket(t, r.Root, "public-complete-v1-0", t10)
	if x := atm(t, r.Root, nil, "release", "candidate", "--request-id", "public-candidate-v1-0", "--target", "v1-0", "--expected-revision", "1", "--issued-at", "2026-09-20T12:06:00Z"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("candidate v1-0: %+v", x.res)
	}
	revision10, candidate10, show10 := publicReleaseCandidate(t, r.Root, "v1-0")
	predecessors := field(field(show10, "candidateBinding"), "predecessors").Arr
	if len(predecessors) != 1 || field(predecessors[0], "releaseId").Str != "v0-9" || field(predecessors[0], "promotionSha256").Str != promotion09 {
		t.Fatalf("successor did not bind public predecessor promotion: %+v", show10)
	}
	recordPublicReleaseGate(t, r.Root, "v1-0", revision10, candidate10, "public-gate-v1-0")
	ready10 := atm(t, r.Root, nil, "release", "readiness", "v1-0")
	if ready10.res.Outcome != wire.OutcomeOK || field(ready10.res.Items[0], "readiness").Str != release.ReadyAttested {
		t.Fatalf("public readiness v1-0: %+v", ready10.res)
	}
	if x := atm(t, r.Root, nil, "release", "promote", "--request-id", "public-promote-v1-0", "--target", "v1-0", "--expected-revision", field(ready10.res.Items[0], "revision").Str, "--issued-at", "2026-09-20T12:07:00Z"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("promote v1-0: %+v", x.res)
	}
	final10 := atm(t, r.Root, nil, "release", "show", "v1-0")
	if final10.res.Outcome != wire.OutcomeOK || len(final10.res.Items) != 1 {
		t.Fatalf("show promoted v1-0: %+v", final10.res)
	}
	show10 = final10.res.Items[0]
	if field(show10, "promotionSha256").Str == "" || field(field(show10, "promotion"), "predecessors").Arr[0].Obj == nil {
		t.Fatalf("public promotion v1-0: %+v", show10)
	}
}

// TestTMV0028_AS38_DurableTwoReleaseCreateAndReplay is the earliest durable
// multi-release path: two ordered projections commit through the ordinary
// receipt-before-state writer, and replay wins after later release changes.
func TestTMV0028_AS38_DurableTwoReleaseCreateAndReplay(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	t1 := createTicket(t, r.Root, "ticket-one")
	t2 := createTicket(t, r.Root, "ticket-two")
	firstArgs := []string{"release", "create", "--request-id", "release-one", "--target", "v0-9", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v0-9", "Version 0.9", t1, "")}
	first := atm(t, r.Root, nil, firstArgs...)
	if first.res.Outcome != wire.OutcomeOK || field(first.res.Items[0], "resultingRevision").Str != "1" {
		t.Fatalf("first release: %+v", first.res)
	}
	stale := atm(t, r.Root, nil, "release", "update", "--request-id", "stale-release-update", "--target", "v0-9", "--expected-revision", "9", "--issued-at", "2026-09-20T12:01:30Z", "--payload", releaseCreatePayload("v0-9", "Changed", t1, ""))
	if stale.res.Outcome == wire.OutcomeOK {
		t.Fatal("stale release update succeeded")
	}
	second := atm(t, r.Root, nil, "release", "create", "--request-id", "release-two", "--target", "v1-0", "--issued-at", "2026-09-20T12:02:00Z", "--payload", releaseCreatePayload("v1-0", "Version 1.0", t2, "v0-9"))
	if second.res.Outcome != wire.OutcomeOK {
		t.Fatalf("second release: %+v", second.res)
	}
	list := atm(t, r.Root, nil, "release", "list")
	if list.res.Outcome != wire.OutcomeOK || len(list.res.Items) != 2 {
		t.Fatalf("release list: %+v", list.res)
	}
	replay := atm(t, r.Root, nil, firstArgs...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool || field(replay.res.Items[0], "releaseId").Str != "v0-9" || field(replay.res.Items[0], "resultingRevision").Str != "1" {
		t.Fatalf("release replay after later mutation: %+v", replay.res)
	}
}

func TestTMV0028_AS38_PolicyCanRemoveReleaseAuthorization(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	policy := fixture.PolicyValue()
	policy.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("OWNER", wire.Strings([]string{"CREATE"}))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ticketID := createTicket(t, r.Root, "policy-ticket")
	denied := atm(t, r.Root, nil, "release", "create", "--request-id", "policy-release", "--target", "v1-0", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v1-0", "Version 1.0", ticketID, ""))
	if denied.res.Outcome == wire.OutcomeOK {
		t.Fatal("configured policy removal was bypassed")
	}
	if listed := atm(t, r.Root, nil, "release", "list"); listed.res.Outcome != wire.OutcomeOK || len(listed.res.Items) != 0 {
		t.Fatalf("denied release changed state: %+v", listed.res)
	}
}

func TestTMV0028_AS38_CandidateSurvivesTaskmanWritesAndSourceEditInvalidates(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.Root, "source.txt"), []byte("candidate source\n"))
	git(t, r.Root, "init")
	git(t, r.Root, "config", "user.email", "fixture@example.invalid")
	git(t, r.Root, "config", "user.name", "Fixture")
	git(t, r.Root, "add", ".taskman", "source.txt")
	git(t, r.Root, "commit", "-m", "fixture baseline")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ticketID := createTicket(t, r.Root, "candidate-ticket")
	create := atm(t, r.Root, nil, "release", "create", "--request-id", "candidate-release", "--target", "v1-0", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v1-0", "Version 1.0", ticketID, ""))
	if create.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %+v", create.res)
	}
	capture := atm(t, r.Root, nil, "release", "candidate", "--request-id", "capture-v1", "--target", "v1-0", "--expected-revision", "1", "--issued-at", "2026-09-20T12:02:00Z")
	if capture.res.Outcome != wire.OutcomeOK {
		t.Fatalf("candidate capture: %+v", capture.res)
	}
	ready := atm(t, r.Root, nil, "release", "readiness", "v1-0")
	if ready.res.Outcome != wire.OutcomeOK {
		t.Fatalf("readiness: %+v", ready.res)
	}
	for _, v := range field(ready.res.Items[0], "missing").Arr {
		if v.Str == "candidate-source-or-policy" {
			t.Fatal("taskman projection writes invalidated the candidate")
		}
	}
	if err := os.WriteFile(filepath.Join(r.Root, "source.txt"), []byte("changed source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Root, "source.txt"), []byte("candidate source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(r.Root, "source.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	modeChanged := atm(t, r.Root, nil, "release", "readiness", "v1-0")
	modeStale := false
	for _, v := range field(modeChanged.res.Items[0], "missing").Arr {
		modeStale = modeStale || v.Str == "candidate-source-or-policy"
	}
	if !modeStale {
		t.Fatal("chmod did not invalidate candidate")
	}
	if err := os.Chmod(filepath.Join(r.Root, "source.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Root, "source.txt"), []byte("changed source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := atm(t, r.Root, nil, "release", "readiness", "v1-0")
	if stale.res.Outcome != wire.OutcomeOK {
		t.Fatalf("stale readiness: %+v", stale.res)
	}
	found := false
	for _, v := range field(stale.res.Items[0], "missing").Arr {
		found = found || v.Str == "candidate-source-or-policy"
	}
	if !found {
		t.Fatalf("source edit did not invalidate candidate: %+v", stale.res.Items[0])
	}
}

func TestTMV0028_AS38_ReleaseDivergenceNeedsKeepJournal(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ticketID := createTicket(t, r.Root, "reconcile-ticket")
	created := atm(t, r.Root, nil, "release", "create", "--request-id", "reconcile-release", "--target", "v1-0", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v1-0", "Version 1.0", ticketID, ""))
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %+v", created.res)
	}
	path := filepath.Join(r.IntentDir, "releases", "v1-0.json")
	canonical, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manual := []byte("manual release edit\n")
	if err := os.WriteFile(path, manual, 0o644); err != nil {
		t.Fatal(err)
	}
	inspect := atm(t, r.Root, nil, "reconcile", "inspect", "v1-0")
	if inspect.res.Outcome != wire.OutcomeOK || len(inspect.res.Items) != 1 {
		t.Fatalf("inspect: %+v", inspect.res)
	}
	digest := field(inspect.res.Items[0], "canonicalRecordSha256").Str
	adopt := atm(t, r.Root, manual, "reconcile", "intent", "--target", "v1-0", "--request-id", "adopt-release", "--adopt-file", "--file", "-")
	if adopt.res.Outcome == wire.OutcomeOK {
		t.Fatal("release ADOPT_FILE unexpectedly succeeded")
	}
	kept := atm(t, r.Root, manual, "reconcile", "intent", "--target", "v1-0", "--request-id", "keep-release", "--keep-journal", "--canonical-sha256", digest, "--file", "-")
	if kept.res.Outcome != wire.OutcomeOK {
		t.Fatalf("keep journal: %+v", kept.res)
	}
	restored, err := os.ReadFile(path)
	if err != nil || string(restored) != string(canonical) {
		t.Fatalf("canonical projection not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.StateDir, "evidence", string(wire.Sum(manual)))); err != nil {
		t.Fatalf("discarded bytes not retained: %v", err)
	}
}

func TestTMV0028_AS38_AttestedPromotionAndImmutability(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.Root, "source.txt"), []byte("release source\n"))
	git(t, r.Root, "init")
	git(t, r.Root, "config", "user.email", "fixture@example.invalid")
	git(t, r.Root, "config", "user.name", "Fixture")
	git(t, r.Root, "add", ".taskman", "source.txt")
	git(t, r.Root, "commit", "-m", "fixture baseline")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ticketID := createTicket(t, r.Root, "promotion-ticket")
	create := atm(t, r.Root, nil, "release", "create", "--request-id", "promotion-release", "--target", "v0-9", "--issued-at", "2026-09-20T12:02:00Z", "--payload", releaseCreatePayload("v0-9", "Version 0.9", ticketID, ""))
	if create.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", create.res)
	}
	for _, verb := range []string{"pause", "unpause"} {
		if x := atm(t, r.Root, nil, verb, "--request-id", "release-"+verb); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s after release: %+v", verb, x.res)
		}
	}
	completed := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "complete-ticket", "--target", ticketID, "--expected-revision", "1", "--issued-at", "2026-09-20T12:01:00Z", "--payload", `{"evidence":[],"reason":"done"}`)
	if completed.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete after release: %+v", completed.res)
	}
	capture := atm(t, r.Root, nil, "release", "candidate", "--request-id", "promotion-candidate", "--target", "v0-9", "--expected-revision", "1", "--issued-at", "2026-09-20T12:03:00Z")
	if capture.res.Outcome != wire.OutcomeOK {
		t.Fatalf("capture: %+v", capture.res)
	}
	evidence := string(wire.Sum([]byte("external evidence")))
	raw, err := os.ReadFile(filepath.Join(r.IntentDir, "releases", "v0-9.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := release.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	candidate := string(release.CandidateDigest(record.Candidate))
	attestation := fmt.Sprintf(`{"attestation":{"actor":"untrusted","attestationId":"verify-v0-9","candidateSha256":"%s","criteria":["0"],"evidence":["%s"],"gateId":"verify","profile":"taskman-release-attestation/0","provenance":"EXTERNAL_ATTESTATION","recordedAt":"2026-09-20T12:04:00Z","result":"PASS","sourceIdentity":"external-ci"}}`, evidence, evidence)
	bad := atm(t, r.Root, nil, "release", "record-gate", "--request-id", "stale-candidate-gate", "--target", "v0-9", "--expected-revision", "2", "--payload", attestation)
	if bad.res.Outcome == wire.OutcomeOK {
		t.Fatal("mismatched attestation candidate accepted")
	}
	attestation = strings.Replace(attestation, `"candidateSha256":"`+evidence, `"candidateSha256":"`+candidate, 1)
	recorded := atm(t, r.Root, nil, "release", "record-gate", "--request-id", "promotion-gate", "--target", "v0-9", "--expected-revision", "2", "--issued-at", "2026-09-20T12:04:00Z", "--payload", attestation)
	if recorded.res.Outcome != wire.OutcomeOK {
		t.Fatalf("record gate: %+v", recorded.res)
	}
	ready := atm(t, r.Root, nil, "release", "readiness", "v0-9")
	if ready.res.Outcome != wire.OutcomeOK || field(ready.res.Items[0], "readiness").Str != "READY_ATTESTED" {
		t.Fatalf("readiness: %+v", ready.res)
	}
	promoted := atm(t, r.Root, nil, "release", "promote", "--request-id", "promote-v0-9", "--target", "v0-9", "--expected-revision", "3", "--issued-at", "2026-09-20T12:05:00Z")
	if promoted.res.Outcome != wire.OutcomeOK {
		t.Fatalf("promote: %+v", promoted.res)
	}
	update := atm(t, r.Root, nil, "release", "update", "--request-id", "rewrite-promoted", "--target", "v0-9", "--expected-revision", "4", "--issued-at", "2026-09-20T12:06:00Z", "--payload", releaseCreatePayload("v0-9", "Changed", ticketID, ""))
	if update.res.Outcome == wire.OutcomeOK {
		t.Fatal("promoted release was mutable")
	}
}

func TestTMV0028_AS38_UnknownGateRefusalLeavesStoreUsable(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	id := createTicket(t, r.Root, "unknown-gate-ticket")
	payload := strings.Replace(releaseCreatePayload("v1", "Release", id, ""), `"requiredGates":[]`, `"requiredGates":["missing"]`, 1)
	x := atm(t, r.Root, nil, "release", "create", "--request-id", "unknown-gate", "--target", "v1", "--payload", payload)
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatal("unknown gate committed")
	}
	if x := atm(t, r.Root, nil, "release", "list"); x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 0 {
		t.Fatal(x.res)
	}
	createTicket(t, r.Root, "after-unknown-gate")
	if x := atm(t, r.Root, nil, "release", "create", "--request-id", "valid-gate-release", "--target", "v1", "--payload", releaseCreatePayload("v1", "Release", id, "")); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	x = atm(t, r.Root, nil, "release", "update", "--request-id", "unknown-update-gate", "--target", "v1", "--expected-revision", "1", "--payload", payload)
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatal("unknown gate update committed")
	}
	if x := atm(t, r.Root, nil, "release", "show", "v1"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "revision").Str != "1" {
		t.Fatal(x.res)
	}
}

func TestTMV0028_AS38_PendingReleaseRedoAndTicketReconciliation(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	id := createTicket(t, r.Root, "redo-ticket")
	headPath := filepath.Join(r.StateDir, "head.json")
	head, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	if x := atm(t, r.Root, nil, "release", "create", "--request-id", "pending-release", "--target", "v1", "--payload", releaseCreatePayload("v1", "Release", id, "")); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	projection := filepath.Join(r.IntentDir, "releases", "v1.json")
	want, err := os.ReadFile(projection)
	if err != nil {
		t.Fatal(err)
	}
	// Reconstruct crash point C2: the receipt exists but head and projection
	// have not advanced. Active staging recovery is outside this fixture.
	fixture.Write(t, headPath, head)
	if err := os.Remove(projection); err != nil {
		t.Fatal(err)
	}
	createTicket(t, r.Root, "trigger-release-redo")
	got, err := os.ReadFile(projection)
	if err != nil || string(got) != string(want) {
		t.Fatalf("release redo: %v", err)
	}
	ticketPath := filepath.Join(r.IntentDir, "tickets", id[len(id)-7:]+".json")
	discard := []byte("{edited ticket")
	fixture.Write(t, ticketPath, discard)
	inspected := atm(t, r.Root, nil, "reconcile", "inspect", id)
	if inspected.res.Outcome != wire.OutcomeOK {
		t.Fatal(inspected.res)
	}
	digest := field(inspected.res.Items[0], "canonicalRecordSha256").Str
	copyPath := filepath.Join(r.Root, "discard.json")
	fixture.Write(t, copyPath, discard)
	kept := atm(t, r.Root, nil, "reconcile", "intent", "--request-id", "keep-ticket-after-release", "--target", id, "--file", copyPath, "--keep-journal", "--canonical-sha256", digest)
	if kept.res.Outcome != wire.OutcomeOK {
		t.Fatal(kept.res)
	}
	if x := atm(t, r.Root, nil, "release", "list"); x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatal(x.res)
	}
}

func TestTMV0028_AS38_ManualGateParsedFromPrettyPayloadAndStdin(t *testing.T) {
	for _, stdin := range []bool{false, true} {
		t.Run(fmt.Sprint(stdin), func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
			policy := fixture.PolicyValue()
			policy.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("OWNER", wire.Strings([]string{"CREATE", "RELEASE_CANDIDATE", "RELEASE_CREATE", "RELEASE_MANUAL_ATTEST"}))))
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
			git(t, r.Root, "init")
			git(t, r.Root, "config", "user.email", "fixture@example.invalid")
			git(t, r.Root, "config", "user.name", "Fixture")
			git(t, r.Root, "add", ".taskman")
			git(t, r.Root, "commit", "-m", "baseline")
			if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
				t.Fatal(x.res)
			}
			id := createTicket(t, r.Root, "manual-ticket")
			if x := atm(t, r.Root, nil, "release", "create", "--request-id", "manual-release", "--target", "v1", "--payload", releaseCreatePayload("v1", "Release", id, "")); x.res.Outcome != wire.OutcomeOK {
				t.Fatal(x.res)
			}
			if x := atm(t, r.Root, nil, "release", "candidate", "--request-id", "manual-candidate", "--target", "v1", "--expected-revision", "1"); x.res.Outcome != wire.OutcomeOK {
				t.Fatal(x.res)
			}
			raw, err := os.ReadFile(filepath.Join(r.IntentDir, "releases", "v1.json"))
			if err != nil {
				t.Fatal(err)
			}
			record, err := release.Decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			payload := fmt.Sprintf("{\n \"attestation\": {\"actor\":\"owner\",\"attestationId\":\"manual\",\"candidateSha256\":%q,\"criteria\":[\"0\"],\"evidence\":[%q],\"gateId\":\"verify\",\"profile\":\"taskman-release-attestation/0\",\"provenance\": \"MANUAL_ATTESTATION\",\"recordedAt\":\"2026-09-20T12:00:00Z\",\"result\":\"PASS\",\"sourceIdentity\":\"owner\"}\n}", release.CandidateDigest(record.Candidate), wire.Sum([]byte("evidence")))
			args := []string{"release", "record-gate", "--request-id", "manual-gate", "--target", "v1", "--expected-revision", "2"}
			var input []byte
			if stdin {
				args = append(args, "--payload-stdin")
				input = []byte(payload)
			} else {
				args = append(args, "--payload", payload)
			}
			if x := atm(t, r.Root, input, args...); x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("manual payload: %+v", x.res)
			}
			raw, err = os.ReadFile(filepath.Join(r.IntentDir, "releases", "v1.json"))
			if err != nil {
				t.Fatal(err)
			}
			record, err = release.Decode(raw)
			if err != nil || len(record.Attestations) != 1 || record.Attestations[0].Provenance != release.Manual {
				t.Fatalf("provenance: %+v %v", record, err)
			}
		})
	}
}
