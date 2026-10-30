package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

const (
	snapshotNotice = `corvint impact: no index snapshot matches this tree, so this call built the full index in memory (1824 ms); run ["corvint","--root","/tmp/replay","index"] once per checkout to reuse it` + "\n"
	// IDX-SNAP-V0-115 form of the same notice.
	treeNotice   = `corvint impact: no index snapshot matches this tree, so this call built the full index in memory (1824 ms); run ["corvint","--root","/tmp/replay","index","--if-stale"] once per committed tree, which every worktree of this repository then reuses` + "\n"
	rangeRefusal = `{"code": "unsupported-impact-range", "error": "impact range exceeds the 256-path bound", "ok": false}` + "\n"
)

// TestDCWV0377SnapshotMissNoticeKeepsTypedAbstention replays the 583-path
// range's stderr (V1-2893): the IDX-SNAP-V0-102 notice before the typed
// refusal keeps the abstention typed in change and admitted by check, while
// any other extra line still reports context-abstention-invalid.
func TestDCWV0377SnapshotMissNoticeKeepsTypedAbstention(t *testing.T) {
	root, evidence, _ := impactChange(t, func(_, errOut io.Writer) int {
		_, _ = io.WriteString(errOut, snapshotNotice+rangeRefusal)
		return 2
	})
	report := readReport(t, root)
	if !bytes.Contains(report, []byte(`{"name": "coordination-time-impact", "status": "NOT_PRODUCED", "reason": "unsupported-impact-range"}`)) {
		t.Fatalf("notice turned the typed abstention into another reason:\n%s", report)
	}
	if !exists(filepath.Join(evidence, "coordination-time-impact-abstention.json")) {
		t.Fatal("no digest-bound abstention artifact")
	}
	base, target := testGit(t, root, "rev-parse", "HEAD~1"), testGit(t, root, "rev-parse", "HEAD")
	k := &check{flow: flow{ctx: context.Background(), root: root, stderr: io.Discard, base: base, target: target}, evidence: evidence}
	run := func() int {
		code, _ := boundary(context.Background(), func() int { k.checkContextAbstention(report); return 0 }, func() {})
		return code
	}
	if code := run(); code != 0 {
		t.Fatalf("check refused the abstention with its snapshot notice: %d", code)
	}

	for name, stderr := range map[string]string{
		"two notices":      snapshotNotice + snapshotNotice + rangeRefusal,
		"other first line": "corvint impact: something else\n" + rangeRefusal,
		"notice after":     rangeRefusal + snapshotNotice,
		"other verb":       `corvint context: no index snapshot matches this tree, so this call built the full index in memory (1824 ms); run ["corvint","--root","/tmp/replay","index"] once per checkout to reuse it` + "\n" + rangeRefusal,
	} {
		t.Run(name, func(t *testing.T) {
			root, _, _ := impactChange(t, func(_, errOut io.Writer) int {
				_, _ = io.WriteString(errOut, stderr)
				return 2
			})
			if report := readReport(t, root); !bytes.Contains(report, []byte(`"reason": "context-abstention-invalid"`)) {
				t.Fatalf("an inadmissible stderr shape was kept as a typed abstention:\n%s", report)
			}
		})
	}
}

// TestDCWV0377ImpactRerunIgnoresOnlyTheNotice: the check rerun agrees across
// verifiers whose notices differ only in duration or presence, never across a
// different refusal.
func TestDCWV0377ImpactRerunIgnoresOnlyTheNotice(t *testing.T) {
	plain := verification{status: 2, stderr: []byte(rangeRefusal)}
	noticed := verification{status: 2, stderr: []byte(snapshotNotice + rangeRefusal)}
	slower := verification{status: 2, stderr: bytes.Replace(noticed.stderr, []byte("1824 ms"), []byte("2950 ms"), 1)}
	if !agreeImpact(plain, noticed) || !agreeImpact(noticed, slower) {
		t.Fatal("verifiers that differ only in the diagnostic notice disagree")
	}
	other := verification{status: 2, stderr: []byte(snapshotNotice + `{"code": "unsupported-impact-path", "error": "x", "ok": false}` + "\n")}
	if agreeImpact(noticed, other) {
		t.Fatal("different refusals agree")
	}
	if !impactAbstentionStderr([]byte(rangeRefusal)) || impactAbstentionStderr([]byte("x\n"+rangeRefusal)) {
		t.Fatal("abstention shape admits the wrong stderr")
	}
}

// TestIDXSNAPV0115TreeNoticeKeepsTypedAbstention (V1-3133): the IDX-SNAP-V0-115
// notice, which names `index --if-stale` once per committed tree, is admitted
// exactly as the IDX-SNAP-V0-102 form an older binary writes; a line that mixes
// the two forms or names another flag is not the notice.
func TestIDXSNAPV0115TreeNoticeKeepsTypedAbstention(t *testing.T) {
	plain := verification{status: 2, stderr: []byte(rangeRefusal)}
	older := verification{status: 2, stderr: []byte(snapshotNotice + rangeRefusal)}
	newer := verification{status: 2, stderr: []byte(treeNotice + rangeRefusal)}
	if !impactAbstentionStderr(newer.stderr) || !agreeImpact(plain, newer) || !agreeImpact(older, newer) {
		t.Fatal("the IDX-SNAP-V0-115 notice is not admitted as the snapshot-miss notice")
	}
	for name, line := range map[string]string{
		"new argv, old wording": strings.Replace(snapshotNotice, `"index"]`, `"index","--if-stale"]`, 1),
		"old argv, new wording": strings.Replace(treeNotice, `,"--if-stale"]`, `]`, 1),
		"another flag":          strings.Replace(treeNotice, `"--if-stale"`, `"--force"`, 1),
		"trailing text":         strings.Replace(treeNotice, "reuses\n", "reuses it\n", 1),
	} {
		if impactAbstentionStderr([]byte(line + rangeRefusal)) {
			t.Errorf("%s: %q admitted as the notice", name, line)
		}
	}
}
