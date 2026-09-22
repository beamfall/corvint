package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/untrackedallowance"
)

func commitRangeFixture(t *testing.T, root string) string {
	t.Helper()
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// feature:stream-token scenario:revoked-token\nfunc MintToken() string { return \"token\" }\n\nfunc stableRangeAnchorOne() {}\nfunc stableRangeAnchorTwo() {}\nfunc stableRangeAnchorThree() {}\nfunc stableRangeAnchorFour() {}\nfunc stableRangeAnchorFive() {}\n\nfunc obsoleteRangeHelper() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "range base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// feature:stream-token scenario:revoked-token\n// ADR-0012 governs the changed token behavior.\nfunc MintToken() string { return \"changed\" }\n\nfunc stableRangeAnchorOne() {}\nfunc stableRangeAnchorTwo() {}\nfunc stableRangeAnchorThree() {}\nfunc stableRangeAnchorFour() {}\nfunc stableRangeAnchorFive() {}\n")
	writeTestFile(t, root, "internal/token/token_test.go", "package token\n\n// feature:stream-token\nfunc TestMintToken() { _ = MintToken() }\n\n// feature:session-browser\nfunc TestChangedSession() { _ = MintToken() }\n")
	writeTestFile(t, root, "notes.txt", "non-Go range member\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "range target")
	return base
}

func TestRangeImpactQualifiesOnlyTouchedMarkersAndAcceptedADRs(t *testing.T) {
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	if receipt["profile"] != rangeImpactProfile || receipt["mode"] != "range-impact" || receipt["state"] != "READY" {
		t.Fatalf("profile=%v mode=%v state=%v", receipt["profile"], receipt["mode"], receipt["state"])
	}
	keys := resultKeys(receipt)
	for _, wanted := range []string{
		"path:internal/token/token.go",
		"path:internal/token/token_test.go",
		"feature:session-browser",
		"decision:docs/adr/0012-token.md",
	} {
		if !stringIn(keys, wanted) {
			t.Errorf("missing %s; keys=%v", wanted, keys)
		}
	}
	for _, unwanted := range []string{"feature:stream-token", "scenario:revoked-token", "reverse-import:internal/api/api.go"} {
		if stringIn(keys, unwanted) {
			t.Errorf("untouched relation leaked %s; keys=%v", unwanted, keys)
		}
	}
	rangeState := receipt["range"].(map[string]any)
	if rangeState["baseCommit"] != base || rangeState["headCommit"] != index.CommitRevision || rangeState["headTree"] != index.Revision || rangeState["changedPathCount"] != 3 || rangeState["changedGoPathCount"] != 2 {
		t.Fatalf("range=%v", rangeState)
	}
	if !strings.HasPrefix(stringValue(rangeState["changesSha256"]), "sha256:") || !strings.HasPrefix(stringValue(rangeState["hunksSha256"]), "sha256:") {
		t.Fatalf("missing range digests: %v", rangeState)
	}
	if !rangeHasDeletionOnlyEvidence(receipt) {
		t.Fatalf("deletion-only hunk is not represented in evidence: %v", receipt["results"])
	}
	omissions := receipt["omissions"].(map[string]any)
	if omissions["count"] != 1 || !strings.HasPrefix(stringValue(omissions["sha256"]), "sha256:") {
		t.Fatalf("omissions=%v", omissions)
	}
	verification := anySlice(receipt["verification"])
	for _, wanted := range []string{"go test ./internal/token/...", "make gate"} {
		if !anyStringIn(verification, wanted) {
			t.Errorf("verification missing %q: %v", wanted, verification)
		}
	}
	testGit(t, root, "config", "diff.algorithm", "histogram")
	testGit(t, root, "config", "diff.indentHeuristic", "true")
	testGit(t, root, "config", "diff.renameLimit", "1")
	second, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil || second["range"].(map[string]any)["changesSha256"] != rangeState["changesSha256"] || second["range"].(map[string]any)["hunksSha256"] != rangeState["hunksSha256"] {
		t.Fatalf("non-deterministic hunk digest: first=%v second=%v err=%v", rangeState, second, err)
	}
}

func TestRangeImpactEmptyDeltaIsExplicit(t *testing.T) {
	root := impactRepository(t, "")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RangeImpact(context.Background(), index, index.CommitRevision, 10)
	if err != nil {
		t.Fatal(err)
	}
	if receipt["state"] != "OUT_OF_SCOPE" || len(anySlice(receipt["results"])) != 0 || receipt["range"].(map[string]any)["changedPathCount"] != 0 {
		t.Fatalf("empty range=%v", receipt)
	}
}

func TestRangeImpactAdmitsCopyAndRenameFromNamedSources(t *testing.T) {
	for _, test := range []struct {
		name, status, target string
	}{
		{name: "copy", status: "C", target: "internal/token/range_copy.go"},
		{name: "rename", status: "R", target: "internal/token/range_renamed.go"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := impactRepository(t, "")
			const sourcePath = "internal/token/range_source.go"
			const source = "package token\n\nfunc rangeOne() int { return 1 }\nfunc rangeTwo() int { return 2 }\nfunc rangeThree() int { return 3 }\nfunc rangeFour() int { return 4 }\nfunc rangeFive() int { return 5 }\nfunc rangeSix() int { return 6 }\nfunc rangeSeven() int { return 7 }\nfunc rangeEight() int { return 8 }\nfunc rangeNine() int { return 9 }\nfunc rangeTen() int { return 10 }\n"
			writeTestFile(t, root, sourcePath, source)
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "range source")
			base := testGit(t, root, "rev-parse", "HEAD")
			baseTree := testGit(t, root, "rev-parse", base+"^{tree}")
			sourceBlob := testGit(t, root, "rev-parse", base+":"+sourcePath)
			if test.status == "R" {
				testGit(t, root, "mv", sourcePath, test.target)
			}
			writeTestFile(t, root, test.target, strings.Replace(source, "return 6", "return 60", 1))
			testGit(t, root, "add", "-A")
			testGit(t, root, "commit", "-qm", test.name+" range")

			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			changes, err := readRangeChanges(context.Background(), index, baseTree)
			if err != nil {
				t.Fatal(err)
			}
			if len(changes) != 1 {
				t.Fatalf("changes=%v", changes)
			}
			change := changes[0]
			if change.status != test.status || change.path != test.target || change.sourcePath != sourcePath || change.sourceBlob != sourceBlob || change.similarity <= 0 || change.similarity >= 100 {
				t.Fatalf("change=%+v", change)
			}
			hunks, err := readRangeHunks(context.Background(), index, baseTree, change)
			if err != nil || len(hunks) != 1 || hunks[0].oldLines != 1 || hunks[0].newLines != 1 {
				t.Fatalf("source-relative hunks=%v err=%v", hunks, err)
			}
			receipt, err := RangeImpact(context.Background(), index, base, 10)
			if err != nil {
				t.Fatal(err)
			}
			if !stringIn(resultKeys(receipt), "path:"+test.target) {
				t.Fatalf("copy or rename target not admitted: %v", resultKeys(receipt))
			}
			changeBytes, err := CanonicalJSON([]any{change.binding()})
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.New()
			hash.Write([]byte("corvint-range-impact-changes/0"))
			hash.Write([]byte{0})
			hash.Write(changeBytes)
			if got, want := receipt["range"].(map[string]any)["changesSha256"], fmt.Sprintf("sha256:%x", hash.Sum(nil)); got != want {
				t.Fatalf("changesSha256=%v want %s", got, want)
			}
			second, err := RangeImpact(context.Background(), index, base, 10)
			if err != nil {
				t.Fatal(err)
			}
			firstBytes, _ := CanonicalJSON(receipt)
			secondBytes, _ := CanonicalJSON(second)
			if !bytes.Equal(firstBytes, secondBytes) {
				t.Fatal("copy or rename range receipt is not deterministic")
			}
		})
	}
}

func TestParseRangeChangesRejectsCopyWithoutReportedSource(t *testing.T) {
	index := &Index{ObjectFormat: "sha1"}
	blob := strings.Repeat("1", 40)
	raw := []byte(fmt.Sprintf(":100644 100644 %s %s C090\x00internal/token/copied.go\x00", blob, blob))
	_, err := parseRangeChanges(raw, index)
	assertRangeErrorCode(t, err, "unsupported-impact-range")
}

func TestRangeImpactRejectsDirtyDeleteModeAndBinary(t *testing.T) {
	t.Run("dirty", func(t *testing.T) {
		root := impactRepository(t, "")
		base := commitRangeFixture(t, root)
		writeTestFile(t, root, "internal/token/token.go", "package token\n")
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		_, err = RangeImpact(context.Background(), index, base, 10)
		assertRangeErrorCode(t, err, "unsupported-impact-worktree")
	})
	for _, test := range []struct {
		name string
		edit func(*testing.T, string)
	}{
		{"delete", func(t *testing.T, root string) { testGit(t, root, "rm", "internal/token/token.go") }},
		{"mode", func(t *testing.T, root string) {
			testGit(t, root, "config", "core.filemode", "true")
			if err := os.Chmod(filepath.Join(root, "internal/token/token.go"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink", func(t *testing.T, root string) {
			if err := os.Symlink("token.go", filepath.Join(root, "internal/token", "link.go")); err != nil {
				t.Skipf("symlink unsupported: %v", err)
			}
		}},
		{"binary", func(t *testing.T, root string) { writeTestFile(t, root, "artifact.bin", "binary\x00bytes") }},
		{"invalid-go", func(t *testing.T, root string) {
			writeTestFile(t, root, "internal/token/token.go", "package token\n\nfunc broken( {\n")
		}},
		{"unsafe-path", func(t *testing.T, root string) {
			writeTestFile(t, root, "internal/bad;echo/unsafe.go", "package unsafe\n")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := impactRepository(t, "")
			base := testGit(t, root, "rev-parse", "HEAD")
			test.edit(t, root)
			testGit(t, root, "add", "-A")
			testGit(t, root, "commit", "-qm", test.name)
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = RangeImpact(context.Background(), index, base, 10)
			assertRangeErrorCode(t, err, "unsupported-impact-range")
		})
	}
}

func TestRangeImpactDisclosesNonAcceptedChangedHunkADR(t *testing.T) {
	root := impactRepository(t, "")
	writeTestFile(t, root, "docs/adr/0013-proposed.md", "# Proposed decision\n\nstatus: proposed\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "proposed authority")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// ADR-0013 is not binding.\nfunc MintToken() string { return \"changed\" }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "cite proposed authority")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RangeImpact(context.Background(), index, base, 10)
	if err != nil {
		t.Fatal(err)
	}
	if stringIn(resultKeys(receipt), "decision:docs/adr/0013-proposed.md") {
		t.Fatalf("non-accepted ADR became authority: %v", resultKeys(receipt))
	}
	if !anyContains(anySlice(receipt["coverage"].(map[string]any)["uncertainty"]), "not an accepted target-HEAD authority") {
		t.Fatalf("missing authority uncertainty: %v", receipt["coverage"])
	}
}

func TestRangeImpactRejectsMutableOrNonAncestorBase(t *testing.T) {
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{base[:12], strings.ToUpper(base), strings.Repeat("f", len(base))} {
		_, err := RangeImpact(context.Background(), index, invalid, 10)
		assertRangeErrorCode(t, err, "unsupported-impact-range")
	}
}

func TestRangeImpactRejectsResultAndRangeBounds(t *testing.T) {
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{0, maxLimit + 1} {
		if _, err := RangeImpact(context.Background(), index, base, limit); err == nil {
			t.Fatalf("limit %d unexpectedly accepted", limit)
		}
	}
	for position := 0; position <= maxImpactPaths; position++ {
		writeTestFile(t, root, fmt.Sprintf("notes/%03d.txt", position), "bounded range member\n")
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "oversized range")
	index, err = Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RangeImpact(context.Background(), index, base, 10)
	assertRangeErrorCode(t, err, "unsupported-impact-range")
}

func assertRangeErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	contextError, ok := err.(*Error)
	if !ok || contextError.Code != code {
		t.Fatalf("error=%#v, want code %q", err, code)
	}
}

func anyStringIn(values []any, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func anyContains(values []any, fragment string) bool {
	for _, value := range values {
		if strings.Contains(stringValue(value), fragment) {
			return true
		}
	}
	return false
}

func rangeHasDeletionOnlyEvidence(receipt map[string]any) bool {
	for _, result := range mapsFromAny(receipt["results"]) {
		for _, raw := range anySlice(result["evidence"]) {
			evidence, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			reason := stringValue(evidence["reason"])
			if strings.HasPrefix(reason, "committed diff hunk ") && strings.HasSuffix(reason, ",0") {
				return true
			}
		}
	}
	return false
}

// TestRangeImpactRefusesAuthorityFromSelfAuthoredADR performs the authority-laundering
// attack directly: the caller's own change set adds `docs/adr/9999-self-minted.md` with
// `status: accepted` and cites ADR-9999 from a changed hunk in the same range. Under
// `CF-V0-031` that document is caller-controlled at target HEAD and confers no
// authority, so the range MUST NOT emit an `authoritative`/`accepted-contract`
// citation for it, and MUST name the abstention. A pre-existing accepted ADR cited by
// the same change set is unaffected, proving the refusal is scoped and not a blanket kill.
func TestRangeImpactRefusesAuthorityFromSelfAuthoredADR(t *testing.T) {
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "docs/adr/9999-self-minted.md", "# Self-minted decision\n\nstatus: accepted\n")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\n// feature:stream-token\n// ADR-9999 authorizes this change; ADR-0012 also governs it.\nfunc MintToken() string { return \"changed\" }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "self-minted authority")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stringIn(resultKeys(receipt), "decision:docs/adr/9999-self-minted.md") {
		t.Fatalf("self-authored ADR became an authority: %v", resultKeys(receipt))
	}
	for _, entry := range anySlice(receipt["results"]) {
		for _, item := range anySlice(entry.(map[string]any)["evidence"]) {
			record := item.(map[string]any)
			if record["path"] == "docs/adr/9999-self-minted.md" {
				t.Fatalf("self-authored ADR emitted evidence: %v", record)
			}
			if record["authority"] == "accepted-contract" && record["path"] != "docs/adr/0012-token.md" {
				t.Fatalf("unexpected accepted-contract evidence: %v", record)
			}
		}
	}
	uncertainty := anySlice(receipt["coverage"].(map[string]any)["uncertainty"])
	if !anyContains(uncertainty, "ADR-9999 cited by a changed hunk is authored by the same change set and confers no authority") {
		t.Fatalf("abstention is not explicitly named: %v", uncertainty)
	}
	if !stringIn(resultKeys(receipt), "decision:docs/adr/0012-token.md") {
		t.Fatalf("independently authored accepted ADR lost authority: %v", resultKeys(receipt))
	}
}

// TestRangeImpactWithholdsSelfAuthoredLedgerAuthority covers the sibling surface
// DR-0007 named: `rangeMarkerResult` emits `authoritative`/`canonical-ledger` for
// a ledger record selected by a changed-hunk marker, and a record the caller's own
// change set declares reaches it exactly the same way. `range impact` does have a
// caller-independent base, so the refusal is scoped by the change set Git computed
// -- a pre-existing ledger record keeps its canonical authority in the very same
// receipt, which is what proves this is not a blanket downgrade.
func TestRangeImpactWithholdsSelfAuthoredLedgerAuthority(t *testing.T) {
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "testing/scenarios.yaml",
		"scenarios:\n  - id: revoked-token\n    area: auth\n    summary: Revoked tokens are denied.\n"+
			"    features: [stream-token]\n    applies: [server]\n    status: shipped\n"+
			"  - id: minted-token\n    area: auth\n    summary: Self-declared scenario.\n"+
			"    features: [stream-token]\n    applies: [server]\n    status: shipped\n")
	writeTestFile(t, root, "internal/token/token.go",
		"package token\n\n// feature:stream-token scenario:revoked-token scenario:minted-token\nfunc MintToken() string { return \"changed\" }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "self-declared ledger record")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	ledgerAuthority := make(map[string]string)
	for _, entry := range anySlice(receipt["results"]) {
		result := entry.(map[string]any)
		for _, item := range anySlice(result["evidence"]) {
			record := item.(map[string]any)
			if record["reason"] == "canonical ledger record selected by exact changed-hunk marker" {
				ledgerAuthority[stringValue(result["kind"])+":"+stringValue(result["id"])] =
					stringValue(record["authority"]) + "/" + stringValue(record["confidence"])
			}
		}
	}
	if got := ledgerAuthority["scenario:minted-token"]; got != UnverifiedLedgerAuthority+"/low" {
		t.Fatalf("self-declared ledger record kept its authority: %q (all=%v)", got, ledgerAuthority)
	}
	if got := ledgerAuthority["feature:stream-token"]; got != "canonical-ledger/authoritative" {
		t.Fatalf("pre-existing ledger record lost its authority: %q (all=%v)", got, ledgerAuthority)
	}
	uncertainty := anySlice(receipt["coverage"].(map[string]any)["uncertainty"])
	if !anyContains(uncertainty, "scenario:minted-token is declared by testing/scenarios.yaml, which the same change set authors") {
		t.Fatalf("withheld ledger authority is not explicitly named: %v", uncertainty)
	}
}

// TestRangeImpactCoverageCountsTruncationOnce pins the single writer of the
// range receipt's result counts.  RangeImpact used to restate them after the
// fact because the shared arithmetic was structurally zero; now that `receipt`
// denominates them in the pre-truncation universe, that restatement is gone and
// this proves the surviving path still reports a ranked-out result -- and that
// the critical set, which is still overridden, keeps naming every changed Go
// path rather than only the ones that survived the ceiling.
func TestRangeImpactCoverageCountsTruncationOnce(t *testing.T) {
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	universe := len(anySlice(admitted["results"]))
	if universe < 2 {
		t.Fatalf("fixture ranks %d results, too few to truncate", universe)
	}
	truncated, err := RangeImpact(context.Background(), index, base, universe-1)
	if err != nil {
		t.Fatal(err)
	}
	coverage := truncated["coverage"].(map[string]any)
	if coverage["requested_results"] != universe {
		t.Fatalf("requested_results = %v, want the %d admitted results", coverage["requested_results"], universe)
	}
	if coverage["included_results"] != universe-1 || coverage["omitted_results"] != 1 {
		t.Fatalf("included=%v omitted=%v, want %d and 1", coverage["included_results"], coverage["omitted_results"], universe-1)
	}
	changedGoPaths := admitted["range"].(map[string]any)["changedGoPathCount"]
	if got := len(anySlice(coverage["critical"])); got != changedGoPaths {
		t.Fatalf("critical names %d selectors, want one per changed Go path (%v)", got, changedGoPaths)
	}
}

func untrackedRangeReceipt(t *testing.T, edit func(root string)) (map[string]any, error) {
	t.Helper()
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	edit(root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return RangeImpact(context.Background(), index, base, 50)
}

func TestRangeImpactAllowsCommittedIgnoredUntrackedPath(t *testing.T) {
	receipt, err := untrackedRangeReceipt(t, func(root string) {
		writeTestFile(t, root, ".gitignore", "*.log\n")
		testGit(t, root, "add", ".gitignore")
		testGit(t, root, "commit", "-qm", "ignore logs")
		writeTestFile(t, root, "internal/token/build.log", "ignored\n")
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := receipt["range"].(map[string]any)
	if receipt["state"] != "READY" || binding["status"] != "CLEAN" || binding["untrackedAllowance"] != nil {
		t.Fatalf("state=%v range=%v", receipt["state"], binding)
	}
}

func TestRangeImpactAllowsDisjointUntrackedPath(t *testing.T) {
	receipt, err := untrackedRangeReceipt(t, func(root string) {
		writeTestFile(t, root, "scratch/notes.md", "operator notes\n")
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := receipt["range"].(map[string]any)
	if receipt["state"] != "READY" || binding["status"] != "UNTRACKED-ALLOWED" {
		t.Fatalf("state=%v range=%v", receipt["state"], binding)
	}
}

func TestRangeImpactRefusesUntrackedPathOverlappingGoBuild(t *testing.T) {
	for _, overlapping := range []string{"internal/token/testdata/fixture.txt", "tools/new.go", "go.work", "scratch/vendor/x.txt", "internal/GO.MOD", "scratch/Vendor/x.txt", ".GITIGNORE"} {
		t.Run(overlapping, func(t *testing.T) {
			_, err := untrackedRangeReceipt(t, func(root string) {
				writeTestFile(t, root, "scratch/notes.md", "disjoint\n")
				writeTestFile(t, root, overlapping, "overlap\n")
			})
			assertRangeErrorCode(t, err, "unsupported-impact-worktree")
			if !strings.HasSuffix(err.Error(), "untracked path overlaps the Go build: "+overlapping) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRangeImpactBindsUntrackedAllowanceDigest(t *testing.T) {
	allowed := []string{"web/draft.ts", "scratch/notes.md"}
	receipt, err := untrackedRangeReceipt(t, func(root string) {
		for _, path := range allowed {
			writeTestFile(t, root, path, "draft\n")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := untrackedallowance.Bind(allowed)
	got, ok := receipt["range"].(map[string]any)["untrackedAllowance"].(map[string]any)
	if !ok || got["rule"] != untrackedallowance.Rule || got["count"] != 2 || got["sha256"] != "sha256:"+want.SHA256 {
		t.Fatalf("untrackedAllowance=%v, want %+v", got, want)
	}
}
