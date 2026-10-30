package cli_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// witnessed is a repo whose ticket holds one report witness, made with or
// without --retain; it returns the report path and its bytes.
func witnessed(t *testing.T, retain bool) (*fixture.Repo, string, string, []byte, run) {
	t.Helper()
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	report := obligationCaseReport(t, r.Root)
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	extra := []string{"--from-playwright-report", report}
	if retain {
		extra = append(extra, "--retain")
	}
	x := atm(t, r.Root, nil, witnessArgs(id, "w-retain", obligationRevision(t, r.Root, id), head, extra...)...)
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "written").Bool {
		t.Fatalf("witness: %s", x.stdout)
	}
	return r, id, report, raw, x
}

func witnessAudit(t *testing.T, root string) (run, wire.Value) {
	t.Helper()
	a := atm(t, root, nil, "receipt", "audit", "--witness-receipts")
	return a, field(a.res.Items[0], "witnessReceipts")
}

func findingCodes(v wire.Value) []string {
	var out []string
	for _, f := range field(v, "findings").Arr {
		out = append(out, field(f, "code").Str)
	}
	return out
}

// TestTOLV0135_WitnessRetainResolves: --retain stores the exact report
// bytes whose digest the witness records and reports it in `retained`; the
// digest then resolves through receipt path and receipt get, the
// --witness-receipts audit finds every witness resolvable, and the plain
// journal audit is unchanged by the store (TOL-V0-135, 136, 138).
func TestTOLV0135_WitnessRetainResolves(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, _, _, raw, x := witnessed(t, true)
	ret := field(x.res.Items[0], "retained")
	sum := wire.Sum(raw)
	if field(ret, "sha256").Str != string(sum) || field(ret, "stored").Str != evidencestore.StoredNew {
		t.Fatalf("retained: %s", x.stdout)
	}
	stored, err := os.ReadFile(evidencestore.Path(r.StateDir, sum))
	if err != nil || !bytes.Equal(stored, raw) {
		t.Fatalf("stored bytes differ: %v", err)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("the witness receipt store broke the journal audit")
	}
	a, v := witnessAudit(t, r.Root)
	if a.res.Outcome != wire.OutcomeOK || field(v, "findingsTotal").Str != "0" || field(v, "resolved").Str == "0" ||
		field(v, "resolved").Str != field(v, "witnessedReports").Str || field(v, "storedFiles").Str != "1" || field(v, "uncitedStoredFiles").Str != "0" {
		t.Fatalf("witness audit: %s", a.stdout)
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	p := atm(t, r.Root, nil, "receipt", "path", string(sum))
	if p.res.Outcome != wire.OutcomeOK || field(p.res.Items[0], "path").Str != evidencestore.Path(r.StateDir, sum) || !field(p.res.Items[0], "verified").Bool {
		t.Fatalf("receipt path: %s", p.stdout)
	}
	var out, errb bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: r.Root, Args: []string{"receipt", "get", string(sum)}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb}); code != 0 || !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("receipt get: exit %d, %d bytes", code, out.Len())
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("receipt path/get wrote state")
	}
}

// TestTOLV0138_AuditReportsUnresolvable: a witness made without --retain
// leaves the plain audit consistent but the --witness-receipts audit
// refuses MISSING_EVIDENCE with a RECEIPT_MISSING finding marked
// unreviewable; a put of the report resolves it, and tampering with the
// stored bytes is RECEIPT_MISMATCH.
func TestTOLV0138_AuditReportsUnresolvable(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, id, report, raw, _ := witnessed(t, false)
	if !auditConsistent(t, r.Root) {
		t.Fatal("plain audit refused")
	}
	a, v := witnessAudit(t, r.Root)
	if a.res.Outcome != wire.OutcomeRefused || !hasCode(a.res, wire.CodeMissingEvidence) || findingCodes(v)[0] != evidencestore.StateMissing ||
		field(field(v, "findings").Arr[0], "reviewable").Bool || field(field(v, "findings").Arr[0], "reportSha256").Str != string(wire.Sum(raw)) {
		t.Fatalf("missing receipt: %s", a.stdout)
	}
	if _, err := os.Lstat(filepath.Join(r.StateDir, evidencestore.Dir)); err == nil {
		t.Fatal("the audit created the witness receipt store")
	}
	put := atm(t, r.Root, nil, "receipt", "put", report)
	if put.res.Outcome != wire.OutcomeOK || field(put.res.Items[0], "stored").Str != evidencestore.StoredNew {
		t.Fatalf("put: %s", put.stdout)
	}
	if again := atm(t, r.Root, nil, "receipt", "put", report); field(again.res.Items[0], "stored").Str != evidencestore.StoredExisting {
		t.Fatalf("repeat put: %s", again.stdout)
	}
	if a, _ := witnessAudit(t, r.Root); a.res.Outcome != wire.OutcomeOK {
		t.Fatalf("after put: %s", a.stdout)
	}
	// TOL-V0-137: a writer and the journal audit accept the store beside
	// the journal.
	head := headOID(t, r.Root)
	if w := atm(t, r.Root, nil, declareArgs(id, "w-after-put", obligationRevision(t, r.Root, id), head, "AC-9")...); w.res.Outcome != wire.OutcomeOK {
		t.Fatalf("writer after put: %s", w.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("journal audit refused the witness receipt store")
	}
	if err := os.WriteFile(evidencestore.Path(r.StateDir, wire.Sum(raw)), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if a, v := witnessAudit(t, r.Root); a.res.Outcome != wire.OutcomeRefused || findingCodes(v)[0] != evidencestore.StateMismatch {
		t.Fatalf("tampered: %s", a.stdout)
	}
	if p := atm(t, r.Root, nil, "receipt", "path", string(wire.Sum(raw))); p.res.Outcome == wire.OutcomeOK || !hasCode(p.res, wire.CodeMalformed) {
		t.Fatalf("path of a tampered receipt: %s", p.stdout)
	}
	if p := atm(t, r.Root, nil, "receipt", "path", string(wire.Sum([]byte("absent")))); p.res.Outcome == wire.OutcomeOK || !hasCode(p.res, wire.CodeMissingEvidence) {
		t.Fatalf("path of an absent receipt: %s", p.stdout)
	}
	// A cited receipt that is not a regular file keeps its finding when the
	// store listing then fails, and the listing failure is a finding too.
	if err := os.Remove(evidencestore.Path(r.StateDir, wire.Sum(raw))); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(evidencestore.Path(r.StateDir, wire.Sum(raw)), 0o700); err != nil {
		t.Fatal(err)
	}
	a, v = witnessAudit(t, r.Root)
	if codes := findingCodes(v); a.res.Outcome != wire.OutcomeRefused || !hasCode(a.res, wire.CodeMissingEvidence) || len(codes) < 2 ||
		codes[0] != evidencestore.StateUnreadable || codes[len(codes)-1] != "STORE_UNREADABLE" {
		t.Fatalf("unreadable receipt: %s", a.stdout)
	}
}

// TestTOLV0140_PruneKeepsCited: prune removes an old receipt no ledger
// cites, keeps the cited one and a recently put one, and --dry-run writes
// nothing.
func TestTOLV0140_PruneKeepsCited(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, _, _, raw, _ := witnessed(t, true)
	cited := evidencestore.Path(r.StateDir, wire.Sum(raw))
	dir := t.TempDir()
	oldFile, newFile := filepath.Join(dir, "old.json"), filepath.Join(dir, "new.json")
	fixture.Write(t, oldFile, []byte("uncited old"))
	fixture.Write(t, newFile, []byte("uncited new"))
	for _, f := range []string{oldFile, newFile} {
		if x := atm(t, r.Root, nil, "receipt", "put", f); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("put: %s", x.stdout)
		}
	}
	old := time.Now().Add(-2 * evidencestore.PruneGrace)
	oldSum := wire.Sum([]byte("uncited old"))
	for _, p := range []string{cited, evidencestore.Path(r.StateDir, oldSum)} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	store := filepath.Join(r.StateDir, evidencestore.Dir)
	staleTemp, freshTemp := filepath.Join(store, evidencestore.TempPrefix+"dead"), filepath.Join(store, evidencestore.TempPrefix+"live")
	fixture.Write(t, staleTemp, []byte("a put that died"))
	fixture.Write(t, freshTemp, []byte("a put in progress"))
	if err := os.Chtimes(staleTemp, old, old); err != nil {
		t.Fatal(err)
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	dry := atm(t, r.Root, nil, "receipt", "prune", "--dry-run")
	if dry.res.Outcome != wire.OutcomeOK || strList(dry.res.Items[0], "removed") != string(oldSum) || field(dry.res.Items[0], "cited").Str != "1" || field(dry.res.Items[0], "keptRecent").Str != "1" {
		t.Fatalf("dry run: %s", dry.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("prune --dry-run wrote state")
	}
	x := atm(t, r.Root, nil, "receipt", "prune")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "removedTotal").Str != "1" || field(x.res.Items[0], "removedStaleTemps").Str != "1" {
		t.Fatalf("prune: %s", x.stdout)
	}
	if _, err := os.Lstat(staleTemp); err == nil {
		t.Fatal("prune kept a stale temp file")
	}
	if _, err := os.Lstat(freshTemp); err != nil {
		t.Fatalf("prune removed a put in progress: %v", err)
	}
	if _, err := os.Stat(evidencestore.Path(r.StateDir, oldSum)); err == nil {
		t.Fatal("prune kept an old uncited receipt")
	}
	for _, p := range []string{cited, evidencestore.Path(r.StateDir, wire.Sum([]byte("uncited new")))} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("prune removed a kept receipt: %v", err)
		}
	}
	if a, _ := witnessAudit(t, r.Root); a.res.Outcome != wire.OutcomeOK {
		t.Fatalf("audit after prune: %s", a.stdout)
	}
}

// TestTOLV0139_ArchiveCarriesReceipts: archive export includes every
// stored witness receipt under witness-receipts/<sha256> and the export
// verifies; a put in progress is skipped and a stray file is refused.
func TestTOLV0139_ArchiveCarriesReceipts(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	r, _, _, raw, _ := witnessed(t, true)
	store := filepath.Join(r.StateDir, evidencestore.Dir)
	fixture.Write(t, filepath.Join(store, evidencestore.TempPrefix+"00ff"), []byte("partial"))
	x := atm(t, r.Root, nil, "archive", "export")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("export: %s", x.stderr)
	}
	want := evidencestore.Dir + "/" + string(wire.Sum(raw))
	found, temp := false, false
	tr := tar.NewReader(bytes.NewReader(x.stdout))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(h.Name, want) {
			body, _ := io.ReadAll(tr)
			found = bytes.Equal(body, raw)
		}
		temp = temp || strings.Contains(h.Name, evidencestore.TempPrefix)
	}
	if !found || temp {
		t.Fatalf("archive: receipt found=%v temp=%v", found, temp)
	}
	if v := atm(t, r.Root, x.stdout, "archive", "verify"); v.res.Outcome != wire.OutcomeOK {
		t.Fatalf("verify: %s", v.stdout)
	}
	fixture.Write(t, filepath.Join(store, "stray.txt"), []byte("x"))
	if x := atm(t, r.Root, nil, "archive", "export"); x.res.Outcome == wire.OutcomeOK {
		t.Fatal("export accepted a stray file in the witness receipt store")
	}
}

// TestTOLV0135_RetainNeedsReport: --retain belongs to a report witness.
func TestTOLV0135_RetainNeedsReport(t *testing.T) {
	r, id, head := obligationRepo(t)
	x := atm(t, r.Root, nil, declareArgs(id, "w-d", obligationRevision(t, r.Root, id), head, "AC-1", "--retain")...)
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "--retain belongs to --from-playwright-report") {
		t.Fatalf("retain without a report: %s", x.stdout)
	}
}
