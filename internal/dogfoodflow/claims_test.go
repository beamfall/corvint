package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/patch"
)

// claimsLines returns count numbered lines "// NAME line N", with the
// lines in edits replaced.
func claimsLines(name string, count int, edits map[int]string) string {
	var text strings.Builder
	for line := 1; line <= count; line++ {
		if edit, ok := edits[line]; ok {
			text.WriteString(edit)
			continue
		}
		fmt.Fprintf(&text, "// %s line %d\n", name, line)
	}
	return text.String()
}

const claimsSpec = "# Spec\n\n- `S-1`: widget alpha returns one.\n- `S-2`: widget beta returns two.\n- `S-3`: the gamma helper lives in `a.go`.\n- `S-4`: delta is documented.\n- `S-5`: epsilon is documented.\n"

// claimsRepo commits a BASE and a change editing a.go lines 5 and 25 and
// b.go line 10: three hunks, in patch order a.go, a.go, b.go.
func claimsRepo(t *testing.T) (root, base string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	claimsCommit(t, root, "base", map[string]string{"docs/spec.md": claimsSpec, "a.go": claimsLines("a", 40, nil), "b.go": claimsLines("b", 20, nil)})
	base = testGit(t, root, "rev-parse", "HEAD")
	claimsCommit(t, root, "change", map[string]string{
		"a.go": claimsLines("a", 40, map[int]string{5: "// a line 5 changed\n", 25: "// a line 25 changed\n"}),
		"b.go": claimsLines("b", 20, map[int]string{10: "// b line 10 changed\n"}),
	})
	return root, base
}

func claimsCommit(t *testing.T, root, message string, files map[string]string) {
	t.Helper()
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", message)
}

// claimsHunks parses the canonical BASE..HEAD patch.
func claimsHunks(t *testing.T, root, base string) []*patch.Hunk {
	t.Helper()
	var stderr bytes.Buffer
	p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &stderr, base: base, target: testGit(t, root, "rev-parse", "HEAD")}}
	parsed, reason, detail := p.readCanonicalPatch()
	if parsed == nil {
		t.Fatalf("canonical patch: %s %s", reason, detail)
	}
	return parsed.Hunks
}

func claimsRecord(t *testing.T, root, base, plan, claims, draft string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	options := RecordClaimsOptions{Root: root, Base: base, Plan: writeInput(t, plan), Claims: claims}
	if draft != "" {
		options.Draft = writeInput(t, draft)
	}
	code, err := RecordClaims(context.Background(), options, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RecordClaims: %v\n%s", err, &stderr)
	}
	return code, stderr.String()
}

func claimsRefresh(t *testing.T, root, base, refresh string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: refresh}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ProposeCitations: %v\n%s", err, &stderr)
	}
	return code, stdout.String(), stderr.String()
}

// claimsBind binds plan with real cem prepare, cite and mark and returns
// the map bytes, removing the map so later drafts see none.
func claimsBind(t *testing.T, root, base, plan string) []byte {
	t.Helper()
	cem := func(args ...string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := cemcli.Run(context.Background(), root, args, &stdout, &stderr); code != 0 {
			t.Fatalf("cem %q = %d\n%s%s", args, code, &stdout, &stderr)
		}
	}
	const mapArg = ".corvint/change.cem.json"
	cem("prepare", "--base", base, "--target", testGit(t, root, "rev-parse", "HEAD"))
	for _, line := range textLines([]byte(plan)) {
		fields := strings.Split(line, "\t")
		if fields[0] == deferredRowKind || len(fields) != 4 {
			continue
		}
		if fields[1] == "unknown" {
			cem("mark", "--map", mapArg, "--hunk", fields[0], "--disposition", "unknown", "--reason", fields[2])
			continue
		}
		cem("cite", "--map", mapArg, "--hunk", fields[0], "--evidence-path", fields[1], "--lines", fields[2], "--relation", fields[3])
	}
	data, err := os.ReadFile(filepath.Join(root, mapArg))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, mapArg)); err != nil {
		t.Fatal(err)
	}
	return data
}

// keyed renders rows keyed by each hunk's full ID, in patch order.
func keyed(hunks []*patch.Hunk, rows map[int][]string) string {
	var plan strings.Builder
	for index, hunk := range hunks {
		for _, row := range rows[index] {
			plan.WriteString(hunk.ID + "\t" + row + "\n")
		}
	}
	return plan.String()
}

// ordinals renders the same rows keyed by ordinal, the hand-keyed form.
func ordinals(hunks []*patch.Hunk, rows map[int][]string) string {
	var plan strings.Builder
	for index := range hunks {
		for _, row := range rows[index] {
			fmt.Fprintf(&plan, "%d\t%s\n", index+1, row)
		}
	}
	return plan.String()
}

// planRowsFor returns the rows of plan keyed by id.
func planRowsFor(plan, id string) []string {
	rows := []string{}
	for _, line := range textLines([]byte(plan)) {
		if strings.HasPrefix(line, id+"\t") {
			rows = append(rows, line)
		}
	}
	return rows
}

var claimsReviewed = map[int][]string{
	0: {"docs/spec.md\t3:3\tspecification"},
	1: {"docs/spec.md\t4:4\tspecification", "docs/spec.md\t7:7\tspecification"},
	2: {"unknown\tinsufficient-evidence\tno BASE contract governs b yet"},
}

// TestClaimsCompileByteIdenticalCEM is the CKC-V0-008 and CKC-V0-011
// differential: claims recorded before a fix commit compile, after it, to
// the rows a hand-keyed plan holds, and both bind to the same cem/1.0 map
// bytes; claims carried through the fix equal claims recorded once after it.
func TestClaimsCompileByteIdenticalCEM(t *testing.T) {
	root, base := claimsRepo(t)
	step1 := claimsHunks(t, root, base)
	if len(step1) != 3 {
		t.Fatalf("step 1 hunks = %d, want 3", len(step1))
	}
	claims := filepath.Join(t.TempDir(), "claims.json")
	if code, stderr := claimsRecord(t, root, base, keyed(step1, claimsReviewed), claims, ""); code != 0 || !strings.Contains(stderr, "RECORDED claims=3 rows=4 tombstones=0") {
		t.Fatalf("record = %d\n%s", code, stderr)
	}
	// Unchanged HEAD: the compiled plan is the reviewed plan.
	code, compiled, stderr := claimsRefresh(t, root, base, claims)
	if code != 0 || compiled != keyed(step1, claimsReviewed) {
		t.Fatalf("refresh at step 1 = %d\n%s\nwant\n%s\n%s", code, compiled, keyed(step1, claimsReviewed), stderr)
	}
	if !strings.Contains(stderr, "carried=4 reassess=0 gone=0 suppressed=0 deferred=0") {
		t.Errorf("claims header missing: %s", stderr)
	}

	// The fix inserts a line between the two a.go edits: the second a.go
	// hunk keeps its content and BASE range but takes a new hunk ID, and a
	// new hunk appears.
	claimsCommit(t, root, "fix", map[string]string{"a.go": claimsLines("a", 40, map[int]string{5: "// a line 5 changed\n", 15: "// a line 15\n// a line 15 inserted\n", 25: "// a line 25 changed\n"})})
	step2 := claimsHunks(t, root, base)
	if len(step2) != 4 || step2[2].ID == step1[1].ID || step2[2].ContentSha256 != step1[1].ContentSha256 {
		t.Fatalf("step 2 hunks do not re-key the second a.go hunk: %d", len(step2))
	}
	truth := map[int][]string{0: claimsReviewed[0], 1: {"docs/spec.md\t6:6\tspecification"}, 2: claimsReviewed[1], 3: claimsReviewed[2]}
	code, compiled, stderr = claimsRefresh(t, root, base, claims)
	if code != 0 {
		t.Fatalf("refresh at step 2 = %d\n%s", code, stderr)
	}
	for _, index := range []int{0, 2, 3} {
		want := strings.Split(strings.TrimSuffix(keyed(step2[index:index+1], map[int][]string{0: truth[index]}), "\n"), "\n")
		if got := planRowsFor(compiled, step2[index].ID); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("hunk %d compiled to %q, want %q", index+1, got, want)
		}
	}
	// The author reviews the new hunk's drafted row; everything else stays.
	var reviewed strings.Builder
	for _, line := range textLines([]byte(compiled)) {
		if strings.HasPrefix(line, step2[1].ID+"\t") {
			continue
		}
		reviewed.WriteString(line + "\n")
	}
	final := keyed(step2, truth)
	if got := reviewed.String(); strings.Replace(final, step2[1].ID+"\tdocs/spec.md\t6:6\tspecification\n", "", 1) != got {
		t.Fatalf("compiled rows differ from the truth beyond the new hunk:\n%s\nwant\n%s", got, final)
	}
	compiledMap := claimsBind(t, root, base, final)
	handMap := claimsBind(t, root, base, ordinals(step2, truth))
	if !bytes.Equal(compiledMap, handMap) {
		t.Errorf("compiled and hand-keyed plans bind to different maps:\n%s\n---\n%s", compiledMap, handMap)
	}

	// Incremental equals full: claims carried through the fix and recorded
	// again equal claims recorded once from the final plan.
	if code, stderr := claimsRecord(t, root, base, final, claims, ""); code != 0 {
		t.Fatalf("incremental record = %d\n%s", code, stderr)
	}
	full := filepath.Join(t.TempDir(), "full.json")
	if code, stderr := claimsRecord(t, root, base, final, full, ""); code != 0 {
		t.Fatalf("full record = %d\n%s", code, stderr)
	}
	incrementalBytes, _ := os.ReadFile(claims)
	fullBytes, _ := os.ReadFile(full)
	if !bytes.Equal(incrementalBytes, fullBytes) {
		t.Errorf("incremental claims differ from full claims:\n%s\n---\n%s", incrementalBytes, fullBytes)
	}
	info, err := os.Stat(claims)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("claims mode = %v, %v; want 0600", info, err)
	}
}

// claimsStep1 records the reviewed rows at step 1 and returns the claims
// file and the step-1 hunks.
func claimsStep1(t *testing.T, root, base string) (string, []*patch.Hunk) {
	t.Helper()
	hunks := claimsHunks(t, root, base)
	claims := filepath.Join(t.TempDir(), "claims.json")
	if code, stderr := claimsRecord(t, root, base, keyed(hunks, claimsReviewed), claims, ""); code != 0 {
		t.Fatalf("record = %d\n%s", code, stderr)
	}
	return claims, hunks
}

// assertNoCarry checks that no row of the claim reviewed for reviewedIndex
// reached any hunk in compiled.
func assertNoCarry(t *testing.T, compiled string, reviewedIndex int) {
	t.Helper()
	for _, line := range textLines([]byte(compiled)) {
		for _, row := range claimsReviewed[reviewedIndex] {
			if strings.HasSuffix(line, "\t"+row) && !strings.Contains(line, "proposed:") {
				t.Errorf("refused claim %d carried: %q", reviewedIndex+1, line)
			}
		}
	}
}

// TestClaimsAmbiguousRefused covers the injected AC2 cases (CKC-V0-005):
// every ambiguous match is refused for reassessment and carries no row.
func TestClaimsAmbiguousRefused(t *testing.T) {
	t.Run("changed", func(t *testing.T) {
		root, base := claimsRepo(t)
		claims, _ := claimsStep1(t, root, base)
		claimsCommit(t, root, "fix", map[string]string{"a.go": claimsLines("a", 40, map[int]string{5: "// a line 5 changed twice\n", 25: "// a line 25 changed\n"})})
		code, compiled, stderr := claimsRefresh(t, root, base, claims)
		if code != 0 || !strings.Contains(stderr, "REASSESS changed claim a.go BASE 2,7") || !strings.Contains(stderr, "carried=3 reassess=1") {
			t.Fatalf("changed = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 0)
	})
	t.Run("split", func(t *testing.T) {
		root, base := claimsRepo(t)
		// The change edits a.go lines 5 to 20 as one hunk.
		edits := map[int]string{}
		for line := 5; line <= 20; line++ {
			edits[line] = fmt.Sprintf("// a line %d changed\n", line)
		}
		claimsCommit(t, root, "wide", map[string]string{"a.go": claimsLines("a", 40, edits)})
		hunks := claimsHunks(t, root, base)
		reviewed := map[int][]string{0: claimsReviewed[0], 1: claimsReviewed[2]}
		claims := filepath.Join(t.TempDir(), "claims.json")
		if len(hunks) != 2 {
			t.Fatalf("hunks = %d", len(hunks))
		}
		if code, stderr := claimsRecord(t, root, base, keyed(hunks, reviewed), claims, ""); code != 0 {
			t.Fatalf("record = %d\n%s", code, stderr)
		}
		for line := 8; line <= 17; line++ {
			delete(edits, line)
		}
		claimsCommit(t, root, "revert middle", map[string]string{"a.go": claimsLines("a", 40, edits)})
		code, compiled, stderr := claimsRefresh(t, root, base, claims)
		if code != 0 || !strings.Contains(stderr, "REASSESS split claim a.go") {
			t.Fatalf("split = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 0)
	})
	t.Run("merged", func(t *testing.T) {
		root, base := claimsRepo(t)
		claims, _ := claimsStep1(t, root, base)
		claimsCommit(t, root, "join", map[string]string{"a.go": claimsLines("a", 40, map[int]string{5: "// a line 5 changed\n", 11: "// a line 11 changed\n", 18: "// a line 18 changed\n", 25: "// a line 25 changed\n"})})
		if hunks := claimsHunks(t, root, base); len(hunks) != 2 {
			t.Fatalf("merged fixture has %d hunks, want 2", len(hunks))
		}
		code, compiled, stderr := claimsRefresh(t, root, base, claims)
		if code != 0 || strings.Count(stderr, "REASSESS merged claim a.go") != 2 {
			t.Fatalf("merged = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 0)
		assertNoCarry(t, compiled, 1)
	})
	t.Run("duplicate", func(t *testing.T) {
		root, base := claimsRepo(t)
		// c.go holds one block twice, and the change edits the same line of
		// each copy, so both hunks have one content.
		block := claimsLines("c", 12, nil)
		claimsCommit(t, root, "add c", map[string]string{"c.go": block + block})
		base = testGit(t, root, "rev-parse", "HEAD")
		edited := claimsLines("c", 12, map[int]string{6: "// c line 6 changed\n"})
		claimsCommit(t, root, "edit c", map[string]string{"c.go": edited + edited})
		hunks := claimsHunks(t, root, base)
		if len(hunks) != 2 || hunks[0].ContentSha256 != hunks[1].ContentSha256 {
			for _, hunk := range hunks {
				t.Logf("%s %d,%d\n%s", hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count, hunk.RawBody)
			}
			t.Fatalf("duplicate fixture hunks = %d", len(hunks))
		}
		claims := filepath.Join(t.TempDir(), "claims.json")
		if code, stderr := claimsRecord(t, root, base, keyed(hunks, map[int][]string{0: claimsReviewed[0], 1: claimsReviewed[1]}), claims, ""); code != 0 {
			t.Fatalf("record = %d\n%s", code, stderr)
		}
		code, compiled, stderr := claimsRefresh(t, root, base, claims)
		if code != 0 || strings.Count(stderr, "REASSESS duplicate claim c.go") != 2 {
			t.Fatalf("duplicate = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 0)
		assertNoCarry(t, compiled, 1)
	})
	t.Run("gone", func(t *testing.T) {
		root, base := claimsRepo(t)
		claims, _ := claimsStep1(t, root, base)
		claimsCommit(t, root, "revert", map[string]string{"a.go": claimsLines("a", 40, map[int]string{5: "// a line 5 changed\n"})})
		code, compiled, stderr := claimsRefresh(t, root, base, claims)
		if code != 0 || !strings.Contains(stderr, "claim a.go BASE 22,7 gone") || !strings.Contains(stderr, "gone=1") {
			t.Fatalf("gone = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 1)
	})
	rebased := func(t *testing.T, files map[string]string) (string, string, string, []*patch.Hunk) {
		t.Helper()
		root, base := claimsRepo(t)
		claims, step1 := claimsStep1(t, root, base)
		change := map[string]string{}
		for _, name := range []string{"a.go", "b.go"} {
			change[name] = testGit(t, root, "show", "HEAD:"+name) + "\n"
		}
		testGit(t, root, "checkout", "-q", "--detach", base)
		claimsCommit(t, root, "new base", files)
		newBase := testGit(t, root, "rev-parse", "HEAD")
		// The rebased change applies the same edits to the moved lines.
		for name, text := range change {
			if moved, ok := files[name]; ok {
				for _, line := range strings.SplitAfter(text, "\n") {
					if strings.Contains(line, "changed") {
						original := strings.Replace(line, " changed", "", 1)
						moved = strings.Replace(moved, original, line, 1)
					}
				}
				change[name] = moved
			}
		}
		claimsCommit(t, root, "rebased change", change)
		return root, newBase, claims, step1
	}
	t.Run("unlocated", func(t *testing.T) {
		// The new BASE repeats a.go's first ten lines, so the first edit's
		// anchor is no longer unique; the second still relocates.
		a := claimsLines("a", 40, nil)
		head := strings.Join(strings.SplitAfter(a, "\n")[:10], "")
		root, newBase, claims, _ := rebased(t, map[string]string{"a.go": a + head})
		code, compiled, stderr := claimsRefresh(t, root, newBase, claims)
		if code != 0 || !strings.Contains(stderr, "REASSESS unlocated claim a.go BASE 2,7") {
			t.Fatalf("unlocated = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 0)
		hunks := claimsHunks(t, root, newBase)
		if got := planRowsFor(compiled, hunks[len(hunks)-2].ID); len(got) != 2 || !strings.HasSuffix(got[0], "\tdocs/spec.md\t4:4\tspecification") {
			t.Errorf("the second a.go edit did not relocate: %q", got)
		}
	})
	t.Run("cite-unlocated", func(t *testing.T) {
		spec := strings.Replace(claimsSpec, "widget alpha returns one", "widget alpha returns uno", 1)
		root, newBase, claims, _ := rebased(t, map[string]string{"docs/spec.md": spec})
		code, compiled, stderr := claimsRefresh(t, root, newBase, claims)
		if code != 0 || !strings.Contains(stderr, "REASSESS cite-unlocated claim a.go BASE 2,7") {
			t.Fatalf("cite-unlocated = %d\n%s\n%s", code, compiled, stderr)
		}
		assertNoCarry(t, compiled, 0)
	})
	t.Run("reorder", func(t *testing.T) {
		// The new BASE moves a.go lines 21 to 40 before line 1, so the two
		// edits swap places; each claim must follow its own content.
		parts := strings.SplitAfter(claimsLines("a", 40, nil), "\n")
		moved := strings.Join(parts[20:40], "") + strings.Join(parts[:20], "")
		root, newBase, claims, _ := rebased(t, map[string]string{"a.go": moved})
		code, compiled, stderr := claimsRefresh(t, root, newBase, claims)
		if code != 0 || strings.Contains(stderr, "REASSESS") {
			t.Fatalf("reorder = %d\n%s\n%s", code, compiled, stderr)
		}
		hunks := claimsHunks(t, root, newBase)
		for _, hunk := range hunks {
			rows := planRowsFor(compiled, hunk.ID)
			body := string(hunk.RawBody)
			switch {
			case strings.Contains(body, "a line 25 changed"):
				if len(rows) != 2 || !strings.HasSuffix(rows[0], "\tdocs/spec.md\t4:4\tspecification") {
					t.Errorf("line-25 edit took %q", rows)
				}
			case strings.Contains(body, "a line 5 changed"):
				if len(rows) != 1 || !strings.HasSuffix(rows[0], "\tdocs/spec.md\t3:3\tspecification") {
					t.Errorf("line-5 edit took %q", rows)
				}
			}
		}
	})
}

// TestClaimsRebaseRelocates: after BASE moves every file's lines, each claim
// relocates by its anchor and each cite by its text, and the compiled plan
// binds to the same map as the hand-keyed plan (CKC-V0-004, CKC-V0-006).
func TestClaimsRebaseRelocates(t *testing.T) {
	root, base := claimsRepo(t)
	claims, _ := claimsStep1(t, root, base)
	change := map[string]string{}
	for _, name := range []string{"a.go", "b.go"} {
		change[name] = "// new header one\n// new header two\n// new header three\n" + testGit(t, root, "show", "HEAD:"+name) + "\n"
	}
	testGit(t, root, "checkout", "-q", "--detach", base)
	spec := "# Spec\nIntroduction added.\n" + strings.TrimPrefix(claimsSpec, "# Spec\n")
	claimsCommit(t, root, "new base", map[string]string{
		"docs/spec.md": spec,
		"a.go":         "// new header one\n// new header two\n// new header three\n" + claimsLines("a", 40, nil),
		"b.go":         "// new header one\n// new header two\n// new header three\n" + claimsLines("b", 20, nil),
	})
	newBase := testGit(t, root, "rev-parse", "HEAD")
	claimsCommit(t, root, "rebased change", change)
	hunks := claimsHunks(t, root, newBase)
	code, compiled, stderr := claimsRefresh(t, root, newBase, claims)
	moved := map[int][]string{
		0: {"docs/spec.md\t4:4\tspecification"},
		1: {"docs/spec.md\t5:5\tspecification", "docs/spec.md\t8:8\tspecification"},
		2: claimsReviewed[2],
	}
	if code != 0 || compiled != keyed(hunks, moved) || !strings.Contains(stderr, "carried=4 reassess=0") {
		t.Fatalf("rebase refresh = %d\n%s\nwant\n%s\n%s", code, compiled, keyed(hunks, moved), stderr)
	}
	if !bytes.Equal(claimsBind(t, root, newBase, compiled), claimsBind(t, root, newBase, ordinals(hunks, moved))) {
		t.Error("relocated plan binds to a different map than the hand-keyed plan")
	}
}

// TestClaimsPrefetchBlobsMatchesBlob covers CKC-V0-012: a batched read
// caches every clean name exactly as blob reads it one by one, including the
// absent cases, and leaves the rest to blob.
func TestClaimsPrefetchBlobsMatchesBlob(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"a.go": claimsLines("a", 40, nil), "empty.txt": "", "dir/x.go": "package x\n", "exec.sh": "echo hi\n", "glob*.go": "// glob\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "update-index", "--chmod=+x", "exec.sh")
	testGit(t, root, "update-index", "--add", "--cacheinfo", "120000,"+testGit(t, root, "hash-object", "-w", "dir/x.go")+",link")
	testGit(t, root, "commit", "-q", "-m", "base")
	commit := testGit(t, root, "rev-parse", "HEAD")
	reader := func() *proposal {
		var stderr bytes.Buffer
		return &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &stderr}, blobs: map[string][]byte{}}
	}
	clean := []string{"a.go", "empty.txt", "dir/x.go", "exec.sh", "link", "dir", "missing.go", "dir/missing.go"}
	names := append([]string{"glob*.go", "./a.go", "dir/../a.go", ""}, clean...)
	batched := reader()
	batched.prefetchBlobs(commit, append(names, "a.go"))
	for _, name := range clean {
		if _, cached := batched.blobs[commit+":"+name]; !cached {
			t.Errorf("%q was not cached by the batched read", name)
		}
	}
	for _, name := range names[:4] {
		if _, cached := batched.blobs[commit+":"+name]; cached {
			t.Errorf("%q was cached; it must stay with blob", name)
		}
	}
	single := reader()
	for _, name := range names {
		got, gotOK := batched.blob(commit, name)
		want, wantOK := single.blob(commit, name)
		if gotOK != wantOK || !bytes.Equal(got, want) {
			t.Errorf("%q: batched = %v %q, blob = %v %q", name, gotOK, got, wantOK, want)
		}
	}
	bounded := reader()
	bounded.gitOutputLimit = 256
	bounded.prefetchBlobs(commit, []string{"a.go", "dir/x.go"})
	if _, cached := bounded.blobs[commit+":a.go"]; cached {
		t.Error("a.go is over the batch bound but was cached")
	}
	if data, cached := bounded.blobs[commit+":dir/x.go"]; !cached || string(data) != "package x\n" {
		t.Errorf("dir/x.go = %v %q under the bound", cached, data)
	}
}

// TestClaimsTombstones covers AC4 (CKC-V0-003, CKC-V0-007): a drafted cite
// the author removed is never drafted again for that edit, even once the
// edit is reassessed, and the draft still passes the bind precheck.
func TestClaimsTombstones(t *testing.T) {
	root, base := claimsRepo(t)
	hunks := claimsHunks(t, root, base)
	code, draft, stderr := claimsRefresh(t, root, base, "")
	if code != 0 {
		t.Fatalf("draft = %d\n%s", code, stderr)
	}
	// Find a drafted cite for an a.go hunk; the author replaces it.
	target, removed := -1, ""
	for index, hunk := range hunks[:2] {
		for _, row := range planRowsFor(draft, hunk.ID) {
			if fields := strings.Split(row, "\t"); len(fields) == 4 && fields[1] != "unknown" {
				target, removed = index, row
			}
		}
	}
	if target < 0 {
		t.Fatalf("the draft cites nothing for an a.go hunk:\n%s\n%s", draft, stderr)
	}
	removedFields := strings.Split(removed, "\t")
	var plan strings.Builder
	for _, line := range textLines([]byte(draft)) {
		if line == removed {
			line = hunks[target].ID + "\tunknown\tinsufficient-evidence\tthe author found no BASE contract"
		}
		plan.WriteString(line + "\n")
	}
	claims := filepath.Join(t.TempDir(), "claims.json")
	if code, stderr := claimsRecord(t, root, base, plan.String(), claims, draft); code != 0 || !strings.Contains(stderr, "tombstones=1") {
		t.Fatalf("record = %d\n%s", code, stderr)
	}
	// Unchanged HEAD: the reviewed unknown row is carried, not the cite.
	code, compiled, stderr := claimsRefresh(t, root, base, claims)
	if code != 0 || strings.Contains(compiled, removed) {
		t.Fatalf("carried refresh redrafted the removed row = %d\n%s\n%s", code, compiled, stderr)
	}
	// A content change reassesses the edit and drafts it afresh; the
	// tombstone keeps the removed cite out.
	edits := map[int]string{5: "// a line 5 changed\n", 25: "// a line 25 changed\n"}
	edits[[]int{5, 25}[target]] = fmt.Sprintf("// a line %d changed twice\n", []int{5, 25}[target])
	claimsCommit(t, root, "fix", map[string]string{"a.go": claimsLines("a", 40, edits)})
	code, compiled, stderr = claimsRefresh(t, root, base, claims)
	if code != 0 || !strings.Contains(stderr, "REASSESS changed") || !strings.Contains(stderr, "suppressed=1") || !strings.Contains(compiled, "CKC-V0-007") {
		t.Fatalf("reassessed refresh = %d\n%s\n%s", code, compiled, stderr)
	}
	// The stderr hunk notes mark the reassessed hunk drafted and the
	// others carried (CKC-V0-013).
	for index, hunk := range claimsHunks(t, root, base) {
		mark := "; carried\n"
		if index == target {
			mark = "; drafted\n"
		}
		if !strings.Contains(stderr, "  hunk "+hunk.ID+" edits "+hunk.DisplayPath+" at BASE ") || !regexp.MustCompile(`  hunk `+hunk.ID+` edits [^\n]*`+regexp.QuoteMeta(mark)).MatchString(stderr) {
			t.Errorf("hunk %d note lacks %q:\n%s", index, mark, stderr)
		}
	}
	for _, line := range textLines([]byte(compiled)) {
		if fields := strings.Split(line, "\t"); len(fields) == 4 && fields[1] == removedFields[1] && fields[2] == removedFields[2] && fields[3] == removedFields[3] {
			t.Errorf("tombstoned cite redrafted: %q", line)
		}
	}
	precheck, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, compiled)})
	if precheck != 0 {
		t.Errorf("refreshed draft fails the precheck = %d\n%s\n%s", precheck, out, errText)
	}
	// Recording the refreshed plan keeps the tombstone on the edit.
	if code, stderr := claimsRecord(t, root, base, compiled, claims, ""); code != 0 || !strings.Contains(stderr, "tombstones=1") {
		t.Errorf("re-record dropped the tombstone = %d\n%s", code, stderr)
	}
}

// TestClaimsRecordRefusals: record-claims refuses an unresolved, ordinal,
// malformed or out-of-file row and an unusable --claims path, writing
// nothing; a refresh refuses an invalid claims file (CKC-V0-002, CKC-V0-009).
func TestClaimsRecordRefusals(t *testing.T) {
	root, base := claimsRepo(t)
	hunks := claimsHunks(t, root, base)
	for name, tc := range map[string]struct{ plan, want string }{
		"unknown hunk": {"hunk:sha256:" + strings.Repeat("0", 64) + "\tdocs/spec.md\t3:3\tspecification\n", "REFUSE claims-plan-row plan row 1"},
		"ordinal":      {"1\tdocs/spec.md\t3:3\tspecification\n", "REFUSE claims-plan-row plan row 1"},
		"malformed":    {hunks[0].ID + "\tdocs/spec.md\t3:3\n", "REFUSE claims-plan-row plan row 1"},
		"relation":     {hunks[0].ID + "\tdocs/spec.md\t3:3\tvibes\n", "REFUSE claims-plan-row plan row 1"},
		"outside":      {hunks[0].ID + "\tdocs/spec.md\t3:3\tspecification\n" + hunks[1].ID + "\tdocs/spec.md\t90:90\tspecification\n", "REFUSE claims-plan-row plan row 2"},
		"bare unknown": {hunks[0].ID + "\tunknown\tinsufficient-evidence\t--\n", "REFUSE claims-plan-row plan row 1"},
	} {
		t.Run(name, func(t *testing.T) {
			claims := filepath.Join(t.TempDir(), "claims.json")
			code, stderr := claimsRecord(t, root, base, tc.plan, claims, "")
			if code != 2 || !strings.Contains(stderr, tc.want) {
				t.Errorf("record = %d, want 2 and %q\n%s", code, tc.want, stderr)
			}
			if _, err := os.Lstat(claims); err == nil {
				t.Error("a refused record wrote the claims file")
			}
		})
	}
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		code, stderr := claimsRecord(t, root, base, keyed(hunks, claimsReviewed), dir, "")
		if code != 2 || !strings.Contains(stderr, "REFUSE claims-unwritable") {
			t.Errorf("record into a directory = %d\n%s", code, stderr)
		}
	})
	t.Run("unreadable prior", func(t *testing.T) {
		claims := writeInput(t, "{\"schema\":\"other\"}\n")
		code, stderr := claimsRecord(t, root, base, keyed(hunks, claimsReviewed), claims, "")
		if code != 2 || !strings.Contains(stderr, "REFUSE claims-unreadable") {
			t.Errorf("record over an invalid claims file = %d\n%s", code, stderr)
		}
	})
	for name, text := range map[string]string{
		"schema":  "{\"schema\":\"other\",\"base\":\"" + base + "\",\"claims\":[],\"deferred\":[]}\n",
		"base":    "{\"schema\":\"" + claimsSchema + "\",\"base\":\"main\",\"claims\":[],\"deferred\":[]}\n",
		"unknown": "{\"schema\":\"" + claimsSchema + "\",\"base\":\"" + base + "\",\"claims\":[],\"deferred\":[],\"extra\":1}\n",
		"row":     "{\"schema\":\"" + claimsSchema + "\",\"base\":\"" + base + "\",\"claims\":[{\"path\":\"a.go\",\"content\":\"" + hunks[0].ContentSha256 + "\",\"oldStart\":2,\"oldCount\":7,\"anchor\":\"" + hunks[0].ContentSha256 + "\",\"occurrences\":1,\"rows\":[{\"kind\":\"cite\",\"path\":\"docs/spec.md\",\"lines\":\"3:3\",\"relation\":\"vibes\",\"text\":\"" + hunks[0].ContentSha256 + "\"}],\"tombstones\":[]}],\"deferred\":[]}\n",
	} {
		t.Run("refresh "+name, func(t *testing.T) {
			code, stdout, stderr := claimsRefresh(t, root, base, writeInput(t, text))
			if code != 2 || stdout != "" || !strings.Contains(stderr, "REFUSE claims-unreadable") {
				t.Errorf("refresh with an invalid claims file = %d\n%s\n%s", code, stdout, stderr)
			}
		})
	}
}
