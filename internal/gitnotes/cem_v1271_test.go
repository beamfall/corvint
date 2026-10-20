package gitnotes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The verdict ledger tests (CEM-V1-271..279) pin recordedAt; they share the
// package clock, so none of them runs in parallel with another.
func pinVerdictClock(t *testing.T) {
	t.Helper()
	previous := verdictNow
	verdictNow = func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { verdictNow = previous })
}

// ledgerNote is commit's verdict note, "" when there is none.
func ledgerNote(t *testing.T, root, commit string) string {
	t.Helper()
	if fixtureGit(t, root, nil, "for-each-ref", VerdictRef) == "" {
		return ""
	}
	listed := fixtureGit(t, root, nil, "notes", "--ref="+VerdictRef, "list")
	if !strings.Contains(listed, commit) {
		return ""
	}
	return fixtureGit(t, root, nil, "cat-file", "blob", strings.Fields(fixtureGit(t, root, nil, "notes", "--ref="+VerdictRef, "list", commit))[0]) + "\n"
}

// attachLedger writes body byte-exact as commit's verdict note by hand.
func attachLedger(t *testing.T, root, commit string, body []byte) {
	t.Helper()
	blob := fixtureGit(t, root, body, "hash-object", "-w", "--stdin")
	fixtureGit(t, root, nil, "notes", "--ref="+VerdictRef, "add", "-f", "-C", blob, commit)
}

// expectedVerdict is the verdict the native stable verifier reaches for the
// committed fixture map on HEAD, computed independently of the ledger.
func expectedVerdict(t *testing.T, root string) (string, string, string) {
	t.Helper()
	raw := committedMap(t, root)
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	parsed, err := wire.ParseStable(raw)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := verify.Stable(context.Background(), raw, verify.StableOptions{
		Repository: root, ExpectedBase: parsed.Change.BaseRevision, Target: head, ArtifactRoot: root,
	})
	if result.Outcome != "REJECT" && result.Outcome != "ACCEPT" {
		t.Fatalf("fixture: the verifier reached no verdict: %s %v", result.Outcome, result.Code)
	}
	digest, err := result.VerdictDigest()
	if err != nil {
		t.Fatal(err)
	}
	return result.Outcome, digest, *result.MapSha256
}

func recordLine(t *testing.T, record VerdictRecord) []byte {
	t.Helper()
	line, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return line
}

// handRecord is a well-formed record on root's HEAD for mapSha256 with the
// given verdict and chain link.
func handRecord(t *testing.T, root, mapSha256, verdictSha256 string, previous *string) VerdictRecord {
	t.Helper()
	return VerdictRecord{
		Schema: VerdictSchema, MapSha256: mapSha256,
		Target: VerdictTarget{
			GitCommit: fixtureGit(t, root, nil, "rev-parse", "HEAD"),
			GitTree:   fixtureGit(t, root, nil, "rev-parse", "HEAD^{tree}"),
		},
		Outcome: "ACCEPT", VerdictSha256: verdictSha256, Consumer: VerdictConsumer,
		Runtime: VerdictRuntime{GoVersion: "go1.27.2"}, RecordedAt: "2026-10-19T08:00:00Z",
		PreviousRecordSha256: previous,
	}
}

func listVerdicts(t *testing.T, root string) map[string]any {
	t.Helper()
	read, err := ProvenanceWithVerdicts(context.Background(), root, "HEAD", "", true)
	if err != nil {
		t.Fatal(err)
	}
	return read["verdicts"].(map[string]any)
}

// CEM-V1-273, CEM-V1-274: anchoring with --artifacts appends exactly one
// canonical record for (mapSha256, target) whose verdictSha256 is the native
// verifier's, and a repeat with the same verdict writes nothing.
func TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	ctx := context.Background()
	outcome, digest, mapSha256 := expectedVerdict(t, root)
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")

	receipt, err := AnchorWithVerdict(ctx, root, mapPath, "HEAD", root, true)
	if err != nil {
		t.Fatal(err)
	}
	verdict := receipt["verdict"].(map[string]any)
	if receipt["ok"] != true || verdict["written"] != true || verdict["state"] != "verified" || verdict["ref"] != VerdictRef {
		t.Fatalf("anchor receipt: %v", receipt)
	}
	want := VerdictRecord{
		Schema: VerdictSchema, MapSha256: mapSha256,
		Target:  VerdictTarget{GitCommit: head, GitTree: fixtureGit(t, root, nil, "rev-parse", "HEAD^{tree}")},
		Outcome: outcome, VerdictSha256: digest, Consumer: VerdictConsumer,
		Runtime: VerdictRuntime{GoVersion: verdict["record"].(VerdictRecord).Runtime.GoVersion}, RecordedAt: "2026-10-20T12:00:00Z",
	}
	if got := ledgerNote(t, root, head); got != string(recordLine(t, want))+"\n" {
		t.Fatalf("ledger note:\n%s\nwant one line:\n%s", got, recordLine(t, want))
	}
	if want.Runtime.GoVersion == "" || !strings.HasPrefix(want.Runtime.GoVersion, "go") {
		t.Fatalf("runtime.goVersion: %q", want.Runtime.GoVersion)
	}

	before := storeState(t, root)
	verdictNow = func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }
	again, err := AnchorWithVerdict(ctx, root, mapPath, "HEAD", root, true)
	if err != nil {
		t.Fatal(err)
	}
	if again["written"] != false || again["verdict"].(map[string]any)["written"] != false ||
		again["verdict"].(map[string]any)["record"].(VerdictRecord).RecordedAt != "2026-10-20T12:00:00Z" {
		t.Fatalf("repeat receipt: %v", again)
	}
	if storeState(t, root) != before {
		t.Fatal("an identical verdict for the same key changed the repository")
	}
	listed := listVerdicts(t, root)
	if listed["state"] != "verified" || len(listed["records"].([]VerdictRecord)) != 1 {
		t.Fatalf("listing: %v", listed)
	}
}

// CEM-V1-274: a record for another map on the same target is extended, never
// rewritten: the earlier line keeps its bytes and the new record links to it.
func TestCEMV1274AppendLinksToThePreviousRecord(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	first := recordLine(t, handRecord(t, root, strings.Repeat("a", 64), strings.Repeat("b", 64), nil))
	attachLedger(t, root, head, append(append([]byte{}, first...), '\n'))

	if _, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(ledgerNote(t, root, head), "\n"), "\n")
	if len(lines) != 2 || lines[0] != string(first) {
		t.Fatalf("ledger after append: %q", lines)
	}
	var second VerdictRecord
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if second.PreviousRecordSha256 == nil || *second.PreviousRecordSha256 != lineDigest(first) {
		t.Fatalf("chain link: %v", second.PreviousRecordSha256)
	}
	if listed := listVerdicts(t, root); listed["state"] != "verified" || len(listed["records"].([]VerdictRecord)) != 2 {
		t.Fatalf("listing: %v", listed)
	}
}

// CEM-V1-275: a differing verdictSha256 for the same (mapSha256, target) is
// refused with verdict-ledger-conflict before any note, pointer or ledger,
// is written.
func TestCEMV1275DifferingVerdictIsRefused(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	_, digest, mapSha256 := expectedVerdict(t, root)
	other := strings.Repeat("c", 64)
	if other == digest {
		t.Fatal("fixture: the forged verdict equals the real one")
	}
	attachLedger(t, root, head, append(recordLine(t, handRecord(t, root, mapSha256, other, nil)), '\n'))
	before := storeState(t, root)
	_, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true)
	if cemcode.CodeOf(err) != CodeVerdictConflict {
		t.Fatalf("code %q (%v)", cemcode.CodeOf(err), err)
	}
	if storeState(t, root) != before || fixtureGit(t, root, nil, "for-each-ref", Ref) != "" {
		t.Fatal("a refused verdict wrote a note")
	}
}

// CEM-V1-274: a verifier that reaches no verdict is refused with
// verdict-ledger-unsupported-result and writes neither note.
func TestCEMV1274UnsupportedResultWritesNothing(t *testing.T) {
	root := fixtureRepo(t)
	before := storeState(t, root)
	_, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", "relative/artifacts", true)
	if cemcode.CodeOf(err) != CodeVerdictUnsupported {
		t.Fatalf("code %q (%v)", cemcode.CodeOf(err), err)
	}
	if storeState(t, root) != before {
		t.Fatal("an unsupported verification wrote a note")
	}
}

// CEM-V1-276: a tampered ledger is never listed and never extended. Each
// tamper reads its closed state with no records, and an anchor that would
// append refuses verdict-ledger-malformed leaving the note byte-identical.
func TestCEMV1276TamperedLedgerIsNeitherListedNorExtended(t *testing.T) {
	pinVerdictClock(t)
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)
	cases := []struct {
		name, state string
		body        func(t *testing.T, root string) []byte
	}{
		{"unknown member", "malformed", func(t *testing.T, root string) []byte {
			line := recordLine(t, handRecord(t, root, a, b, nil))
			return append(append(line[:len(line)-1:len(line)-1], []byte(`,"extra":1}`)...), '\n')
		}},
		{"non-canonical spacing", "malformed", func(t *testing.T, root string) []byte {
			return append(bytes.Replace(recordLine(t, handRecord(t, root, a, b, nil)), []byte(`":"`), []byte(`": "`), 1), '\n')
		}},
		{"no final line feed", "malformed", func(t *testing.T, root string) []byte {
			return recordLine(t, handRecord(t, root, a, b, nil))
		}},
		{"unknown outcome", "malformed", func(t *testing.T, root string) []byte {
			record := handRecord(t, root, a, b, nil)
			record.Outcome = "UNSUPPORTED"
			return append(recordLine(t, record), '\n')
		}},
		{"edited earlier record", "chain-broken", func(t *testing.T, root string) []byte {
			first := recordLine(t, handRecord(t, root, a, b, nil))
			link := lineDigest(first)
			second := recordLine(t, handRecord(t, root, b, a, &link))
			edited := bytes.Replace(first, []byte(b), []byte(strings.Repeat("d", 64)), 1)
			return []byte(string(edited) + "\n" + string(second) + "\n")
		}},
		{"removed first record", "chain-broken", func(t *testing.T, root string) []byte {
			link := lineDigest(recordLine(t, handRecord(t, root, a, b, nil)))
			return append(recordLine(t, handRecord(t, root, b, a, &link)), '\n')
		}},
		{"record names another commit", "scope-mismatch", func(t *testing.T, root string) []byte {
			record := handRecord(t, root, a, b, nil)
			record.Target.GitCommit = fixtureGit(t, root, nil, "rev-parse", "HEAD~1")
			return append(recordLine(t, record), '\n')
		}},
		{"repeated key", "conflict", func(t *testing.T, root string) []byte {
			first := recordLine(t, handRecord(t, root, a, b, nil))
			link := lineDigest(first)
			return []byte(string(first) + "\n" + string(recordLine(t, handRecord(t, root, a, a, &link))) + "\n")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := fixtureRepo(t)
			head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
			body := c.body(t, root)
			attachLedger(t, root, head, body)
			listed := listVerdicts(t, root)
			if listed["state"] != c.state || len(listed["records"].([]VerdictRecord)) != 0 {
				t.Fatalf("listing: %v", listed)
			}
			before := storeState(t, root)
			_, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true)
			if cemcode.CodeOf(err) != CodeVerdictMalformed {
				t.Fatalf("code %q (%v)", cemcode.CodeOf(err), err)
			}
			if storeState(t, root) != before {
				t.Fatal("a refused append changed the repository")
			}
		})
	}
}

// CEM-V1-277: the listing is a read command: with or without records it
// writes no ref, note, object, index or config byte, and without --verdicts
// provenance carries no verdicts member.
func TestCEMV1277VerdictListingIsReadOnly(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	ctx := context.Background()
	before := storeState(t, root)
	if listed := listVerdicts(t, root); listed["state"] != "absent" || len(listed["records"].([]VerdictRecord)) != 0 {
		t.Fatalf("absent listing: %v", listed)
	}
	if storeState(t, root) != before {
		t.Fatal("listing an absent ledger changed the repository")
	}
	if _, err := AnchorWithVerdict(ctx, root, mapPath, "HEAD", root, true); err != nil {
		t.Fatal(err)
	}
	before = storeState(t, root)
	read, err := ProvenanceWithVerdicts(ctx, root, "HEAD", "", true)
	if err != nil || read["mutates"] != false {
		t.Fatalf("listing: %v %v", read, err)
	}
	if storeState(t, root) != before {
		t.Fatal("listing a ledger changed the repository")
	}
	plain, err := Provenance(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["verdicts"]; present {
		t.Fatal("provenance without --verdicts lists the ledger")
	}
}

// CEM-V1-275: a ledger already holding MaxVerdictRecords records is never
// extended; the refusal names verdict-ledger-full and plans no note body.
func TestCEMV1275FullLedgerIsRefused(t *testing.T) {
	full := ledger{state: "verified"}
	for index := 0; index < MaxVerdictRecords; index++ {
		key := fmt.Sprintf("%064x", index)
		full.records = append(full.records, VerdictRecord{MapSha256: key, VerdictSha256: key})
		full.lines = append(full.lines, []byte(key))
	}
	record := VerdictRecord{MapSha256: strings.Repeat("f", 64), VerdictSha256: strings.Repeat("e", 64)}
	body, recorded, err := planAppend(full, record)
	if cemcode.CodeOf(err) != CodeVerdictFull || body != nil || recorded != nil {
		t.Fatalf("full ledger: code %q body %q record %v", cemcode.CodeOf(err), body, recorded)
	}
	full.records, full.lines = full.records[1:], full.lines[1:]
	if body, _, err := planAppend(full, record); err != nil || body == nil {
		t.Fatalf("one slot left: %v", err)
	}
}

// interleave makes the next joint publish run second first, once.
func interleave(t *testing.T, second func()) {
	t.Helper()
	previous := beforePublish
	beforePublish = func() {
		beforePublish = previous
		second()
	}
	t.Cleanup(func() { beforePublish = previous })
}

// CEM-V1-275: two writers that both validate an absent ledger cannot both
// succeed. The second writer publishes between the first's validation and its
// publish; the first then loses the compare-and-swap, is refused
// verdict-ledger-conflict and overwrites nothing.
func TestCEMV1275LostRaceIsRefusedWithoutOverwrite(t *testing.T) {
	t.Run("second anchor", func(t *testing.T) {
		pinVerdictClock(t)
		root := fixtureRepo(t)
		head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
		var winner map[string]any
		interleave(t, func() {
			verdictNow = func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }
			var err error
			if winner, err = AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true); err != nil {
				t.Errorf("winner: %v", err)
			}
		})
		_, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true)
		if cemcode.CodeOf(err) != CodeVerdictConflict {
			t.Fatalf("loser code %q (%v)", cemcode.CodeOf(err), err)
		}
		if winner == nil || winner["ok"] != true {
			t.Fatalf("winner receipt: %v", winner)
		}
		want := winner["verdict"].(map[string]any)["record"].(VerdictRecord)
		if want.RecordedAt != "2026-10-21T00:00:00Z" || ledgerNote(t, root, head) != string(recordLine(t, want))+"\n" {
			t.Fatalf("ledger holds %q, want only the winner's record", ledgerNote(t, root, head))
		}
		if fixtureGit(t, root, nil, "for-each-ref", "refs/notes/corvint-staging/") != "" {
			t.Fatal("a staging ref survived")
		}
	})
	t.Run("differing verdict", func(t *testing.T) {
		pinVerdictClock(t)
		root := fixtureRepo(t)
		head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
		_, _, mapSha256 := expectedVerdict(t, root)
		forged := append(recordLine(t, handRecord(t, root, mapSha256, strings.Repeat("c", 64), nil)), '\n')
		interleave(t, func() { attachLedger(t, root, head, forged) })
		_, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true)
		if cemcode.CodeOf(err) != CodeVerdictConflict {
			t.Fatalf("loser code %q (%v)", cemcode.CodeOf(err), err)
		}
		if ledgerNote(t, root, head) != string(forged) || fixtureGit(t, root, nil, "for-each-ref", Ref) != "" {
			t.Fatal("the losing anchor overwrote the ledger or wrote its pointer")
		}
	})
}

// CEM-V1-275: the pointer and the ledger are published in one transaction.
// A lock on the ledger ref fails the anchor and leaves the pointer ref
// unwritten; once the lock is gone the same anchor writes both.
func TestCEMV1275LockedLedgerRefLeavesThePointerUnwritten(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	lock := filepath.Join(root, ".git", filepath.FromSlash(VerdictRef)+".lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true); err == nil {
		t.Fatal("anchor succeeded with the ledger ref locked")
	}
	if refs := fixtureGit(t, root, nil, "for-each-ref", "refs/notes/"); refs != "" {
		t.Fatalf("a failed publish left refs: %s", refs)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	receipt, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true)
	if err != nil || receipt["ok"] != true || receipt["written"] != true || receipt["verdict"].(map[string]any)["written"] != true {
		t.Fatalf("anchor after unlock: %v (%v)", receipt, err)
	}
}
