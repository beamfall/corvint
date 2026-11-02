package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestClaimsTombstoneOnlyClaim covers CKC-V0-017: removing every row of an
// edit keeps a claim that holds only its tombstones, so a refresh drafts the
// edit afresh without the removed cites.
func TestClaimsTombstoneOnlyClaim(t *testing.T) {
	root, base := claimsRepo(t)
	hunks := claimsHunks(t, root, base)
	code, draft, stderr := claimsRefresh(t, root, base, "")
	if code != 0 {
		t.Fatalf("draft = %d\n%s", code, stderr)
	}
	target, removed := -1, []string{}
	for index, hunk := range hunks {
		cites := []string{}
		for _, row := range planRowsFor(draft, hunk.ID) {
			if fields := strings.Split(row, "\t"); len(fields) == 4 && fields[1] != "unknown" {
				cites = append(cites, row)
			}
		}
		if len(cites) > 0 {
			target, removed = index, cites
			break
		}
	}
	if target < 0 {
		t.Fatalf("the draft cites nothing:\n%s\n%s", draft, stderr)
	}
	var plan strings.Builder
	for _, line := range textLines([]byte(draft)) {
		if !strings.HasPrefix(line, hunks[target].ID+"\t") {
			plan.WriteString(line + "\n")
		}
	}
	claims := filepath.Join(t.TempDir(), "claims.json")
	code, stderr = claimsRecord(t, root, base, plan.String(), claims, draft)
	if code != 0 || !regexp.MustCompile(`tombstones=[1-9]`).MatchString(stderr) {
		t.Fatalf("record dropped the tombstones of an edit left without rows = %d\n%s", code, stderr)
	}
	data, err := os.ReadFile(claims)
	if err != nil {
		t.Fatal(err)
	}
	file, why := decodeClaims(data)
	if file == nil {
		t.Fatalf("the recorded claims do not decode: %s\n%s", why, data)
	}
	only := 0
	for _, entry := range file.Claims {
		if len(entry.Rows) == 0 && len(entry.Tombstones) == len(removed) {
			only++
		}
	}
	if only != 1 {
		t.Fatalf("want one tombstone-only claim with %d tombstones:\n%s", len(removed), data)
	}
	code, compiled, stderr := claimsRefresh(t, root, base, claims)
	if code != 0 || !regexp.MustCompile(`suppressed=[1-9]`).MatchString(stderr) {
		t.Fatalf("refresh = %d\n%s\n%s", code, compiled, stderr)
	}
	for _, row := range removed {
		if strings.Contains(compiled, row) {
			t.Errorf("refresh redrafted the removed row %q:\n%s", row, compiled)
		}
	}
	if !regexp.MustCompile(`  hunk ` + hunks[target].ID + ` edits [^\n]*; drafted\n`).MatchString(stderr) {
		t.Errorf("the tombstone-only edit is not drafted afresh:\n%s", stderr)
	}
	// Re-recording over the claims, with no draft, keeps the tombstones.
	want := fmt.Sprintf("tombstones=%d ", len(removed))
	if code, stderr := claimsRecord(t, root, base, plan.String(), claims, ""); code != 0 || !strings.Contains(stderr, want) {
		t.Errorf("re-record dropped the tombstone-only claim = %d, want %q\n%s", code, want, stderr)
	}
}

// TestClaimsEveryEditRevertedIsGone covers CKC-V0-005: when the fix reverts
// every claimed edit the patch is empty, and the refresh reports each claim
// gone instead of refusing the patch as unavailable.
func TestClaimsEveryEditRevertedIsGone(t *testing.T) {
	root, base := claimsRepo(t)
	hunks := claimsHunks(t, root, base)
	claims := filepath.Join(t.TempDir(), "claims.json")
	if code, stderr := claimsRecord(t, root, base, keyed(hunks, claimsReviewed), claims, ""); code != 0 {
		t.Fatalf("record = %d\n%s", code, stderr)
	}
	claimsCommit(t, root, "revert", map[string]string{"a.go": claimsLines("a", 40, nil), "b.go": claimsLines("b", 20, nil)})
	code, compiled, stderr := claimsRefresh(t, root, base, claims)
	if code != 0 || strings.Contains(stderr, "claims-patch-unavailable") || !strings.Contains(stderr, "gone=3") || strings.Count(stderr, " gone: no current hunk overlaps it") != 3 {
		t.Fatalf("refresh after every edit was reverted = %d\n%s\n%s", code, compiled, stderr)
	}
}

// TestClaimsStrictDecode covers CKC-V0-018: a claims file with a byte-span
// row, a BASE range past int64 or data after the JSON document is refused
// by name, and a plan's byte-span row is refused at record.
func TestClaimsStrictDecode(t *testing.T) {
	root, base := claimsRepo(t)
	hunks := claimsHunks(t, root, base)
	claim := func(start, count, rows string) string {
		return "{\"schema\":\"" + claimsSchema + "\",\"base\":\"" + base + "\",\"claims\":[{\"path\":\"a.go\",\"content\":\"" + hunks[0].ContentSha256 +
			"\",\"oldStart\":" + start + ",\"oldCount\":" + count + ",\"anchor\":\"" + hunks[0].ContentSha256 + "\",\"occurrences\":1,\"rows\":[" + rows + "],\"tombstones\":[]}],\"deferred\":[]}\n"
	}
	unknown := "{\"kind\":\"unknown\",\"reason\":\"insufficient-evidence\",\"note\":\"no contract\"}"
	claims := filepath.Join(t.TempDir(), "claims.json")
	if code, stderr := claimsRecord(t, root, base, keyed(hunks, claimsReviewed), claims, ""); code != 0 {
		t.Fatalf("record = %d\n%s", code, stderr)
	}
	recorded, err := os.ReadFile(claims)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct{ text, want string }{
		"byte span":      {claim("2", "7", "{\"kind\":\"cite\",\"path\":\"docs/spec.md\",\"lines\":\"bytes:0:4\",\"relation\":\"specification\"}"), "holds a malformed row"},
		"range overflow": {claim("9223372036854775807", "2", unknown), "has an invalid BASE range"},
		"range start":    {claim("0", "3", unknown), "has an invalid BASE range"},
		"trailing json":  {string(recorded) + "{}\n", "holds data after the claims JSON document"},
		"trailing text":  {string(recorded) + "garbage", "holds data after the claims JSON document"},
	} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := claimsRefresh(t, root, base, writeInput(t, tc.text))
			if code != 2 || stdout != "" || !strings.Contains(stderr, "REFUSE claims-unreadable") || !strings.Contains(stderr, tc.want) {
				t.Errorf("refresh = %d, want 2 and %q\n%s\n%s", code, tc.want, stdout, stderr)
			}
		})
	}
	t.Run("plan byte span", func(t *testing.T) {
		claims := filepath.Join(t.TempDir(), "claims.json")
		code, stderr := claimsRecord(t, root, base, hunks[0].ID+"\tdocs/spec.md\tbytes:0:4\tspecification\n", claims, "")
		if code != 2 || !strings.Contains(stderr, "REFUSE claims-plan-row plan row 1") || !strings.Contains(stderr, "byte span") {
			t.Errorf("record of a byte-span row = %d\n%s", code, stderr)
		}
		if _, err := os.Lstat(claims); err == nil {
			t.Error("a refused record wrote the claims file")
		}
	})
	t.Run("whitespace after the document", func(t *testing.T) {
		code, _, stderr := claimsRefresh(t, root, base, writeInput(t, string(recorded)+"\n \n"))
		if code != 0 {
			t.Errorf("trailing whitespace refused = %d\n%s", code, stderr)
		}
	})
}

// TestClaimsRangeRecordsBounds: rangeRecords refuses a range whose end
// overflows int64 instead of panicking on the slice (CKC-V0-018).
func TestClaimsRangeRecordsBounds(t *testing.T) {
	records := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	for _, tc := range []struct {
		start, count int64
		ok           bool
	}{
		{1, 3, true}, {3, 1, true}, {2, 0, true}, {4, 0, true}, {0, 1, false}, {3, 2, false}, {4, 1, false},
		{math.MaxInt64, 2, false}, {math.MaxInt64, math.MaxInt64, false}, {2, math.MaxInt64, false}, {1, -1, false},
	} {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("rangeRecords(%d, %d) panicked: %v", tc.start, tc.count, recovered)
				}
			}()
			got, ok := rangeRecords(records, tc.start, tc.count)
			if ok != tc.ok || ok && int64(len(got)) != tc.count {
				t.Errorf("rangeRecords(%d, %d) = %d records, %v; want %v", tc.start, tc.count, len(got), ok, tc.ok)
			}
		}()
	}
}

// TestClaimsBatchBoundsEveryObject covers CKC-V0-015: a symlink or tree
// past the Git output bound stops the batch reader before it reads the
// object, and the per-file read answers, as blob does, that it is absent.
func TestClaimsBatchBoundsEveryObject(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	files := map[string]string{"big.txt": strings.Repeat("x", 300) + "\n"}
	for _, name := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		files["d/file-"+name+".go"] = "package d\n// " + name + "\n"
	}
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "update-index", "--add", "--cacheinfo", "120000,"+testGit(t, root, "hash-object", "-w", "big.txt")+",link")
	testGit(t, root, "commit", "-q", "-m", "base")
	commit := testGit(t, root, "rev-parse", "HEAD")
	for _, name := range []string{"link", "d"} {
		reader := func(batch bool) *proposal {
			var stderr bytes.Buffer
			p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &stderr}, blobs: map[string][]byte{}, batchBlobs: batch}
			p.gitOutputLimit = 100
			return p
		}
		batched, single := reader(true), reader(false)
		got, gotOK := batched.blob(commit, name)
		want, wantOK := single.blob(commit, name)
		if gotOK || wantOK || got != nil || want != nil {
			t.Errorf("%s: batched = %v %q, blob = %v %q; want both absent", name, gotOK, got, wantOK, want)
		}
		if batched.batch == nil || !batched.batch.failed || batched.batch.command != nil {
			t.Errorf("%s: the batch reader read an object past the bound instead of stopping: %+v", name, batched.batch)
		}
		batched.closeBlobBatch()
	}
}

// gitHasObjectmode reports whether this Git's cat-file --batch knows the
// %(objectmode) atom the batch reader asks for.
func gitHasObjectmode(t *testing.T, root string) bool {
	t.Helper()
	p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &bytes.Buffer{}}}
	command := p.gitCommand(true, "cat-file", "--batch=%(objectmode)")
	command.Stdin = strings.NewReader("HEAD\n")
	return command.Run() == nil
}
