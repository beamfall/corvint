package lrfrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// TestOCMV0940LinkAccumulatesClaimsFromTwoTestFiles links one requirement to
// exact-ID assertions in two test files and checks that neither a plain re-link
// nor a second path silently erases the earlier file's claims.
func TestOCMV0940LinkAccumulatesClaimsFromTwoTestFiles(t *testing.T) {
	root, options := makeOCMTwoFileFixture(t)
	mapPath := filepath.Join(root, options.MapPath)
	link := func(testPath, selector string, accumulate bool) (*LinkResult, error) {
		options.TestPath, options.Claims, options.Accumulate = testPath, []string{selector}, accumulate
		return LinkOCM(context.Background(), root, options)
	}
	const first, second = "a_test.go", "b_test.go"
	firstCase, secondCase := fixtureClaimSelector(t, root, options.Target, first, "/case:"), fixtureClaimSelector(t, root, options.Target, second, "/case:")
	if _, err := link(first, firstCase, false); err != nil {
		t.Fatal(err)
	}
	// A plain re-link on the same single path still replaces (compatibility).
	if _, err := link(first, firstCase, false); err != nil {
		t.Fatalf("same-path replace: %v", err)
	}
	before, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := link(second, secondCase, false); CodeOf(err) != "link-would-drop-claims" || !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), "--accumulate") {
		t.Fatalf("second path without --accumulate: %v", err)
	}
	if after, _ := os.ReadFile(mapPath); !bytes.Equal(before, after) {
		t.Fatal("a refused link changed the map")
	}
	if _, err := link(second, fixtureClaimSelector(t, root, options.Target, second, "TestUnrelated"), true); CodeOf(err) != "claim-obligation-mismatch" {
		t.Fatalf("accumulating a claim without the exact ID: %v", err)
	}
	result, err := link(second, secondCase, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ClaimIDs) != 2 || len(result.HunkIDs) != 1 {
		t.Fatalf("accumulated result: %+v", result)
	}
	checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: options.ExpectedBase,
		Target: options.Target, ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil || checked.Verification["valid"] != true {
		t.Fatalf("accumulated map verdict: %v %v", checked.Verification, err)
	}
	document, err := wire.Parse(checked.OCMRaw)
	if err != nil {
		t.Fatal(err)
	}
	paths := claimPaths(document)
	var linkedPaths []string
	for _, id := range result.ClaimIDs {
		linkedPaths = append(linkedPaths, paths[id])
	}
	slices.Sort(linkedPaths)
	if !slices.Equal(linkedPaths, []string{first, second}) {
		t.Fatalf("linked claim paths = %v", linkedPaths)
	}
	// With both files linked, a plain re-link of either path refuses too.
	if _, err := link(first, firstCase, false); CodeOf(err) != "link-would-drop-claims" || !strings.Contains(err.Error(), second) {
		t.Fatalf("plain re-link after accumulate: %v", err)
	}
	// Accumulating a claim already linked is idempotent.
	again, err := link(first, firstCase, true)
	if err != nil || !slices.Equal(again.ClaimIDs, result.ClaimIDs) {
		t.Fatalf("idempotent accumulate: %+v %v", again, err)
	}
}

// fixtureClaimSelector returns the selector of the one claim extracted from
// path at target whose selector contains marker.
func fixtureClaimSelector(t *testing.T, root, target, path, marker string) string {
	t.Helper()
	blobOID := gitOCMPrepare(t, root, "rev-parse", target+":"+path)
	source, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := enumerateClaims(path, blobOID, source)
	if err != nil {
		t.Fatal(err)
	}
	var selectors []string
	for _, claim := range claims {
		if strings.Contains(claim.selector, marker) {
			selectors = append(selectors, claim.selector)
		}
	}
	if len(selectors) != 1 {
		t.Fatalf("%s claims matching %q: %v", path, marker, selectors)
	}
	return selectors[0]
}

// makeOCMTwoFileFixture commits two test files that each assert OCM-TEST-001
// in a t.Run case, cites the change's first hunk and prepares the OCM map.
func makeOCMTwoFileFixture(t *testing.T) (string, LinkOptions) {
	t.Helper()
	root, base, _ := makeOCMPrepareRepository(t)
	writeOCMPrepareFile(t, root, "a_test.go", `package example
import "testing"
func TestReducer(t *testing.T) {
 t.Run("OCM-TEST-001 reducer keeps the review", func(t *testing.T) {})
}
`)
	writeOCMPrepareFile(t, root, "b_test.go", `package example
import "testing"
func TestUnrelated(t *testing.T) {}
func TestCLI(t *testing.T) {
 t.Run("OCM-TEST-001 cli keeps the review", func(t *testing.T) {})
}
`)
	gitOCMPrepare(t, root, "add", "a_test.go", "b_test.go")
	gitOCMPrepare(t, root, "commit", "-qm", "two test files")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, target, false)
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(context.Background(), workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/specs/intent.md", Lines: "5:5", Relation: "specification",
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	cem, err := wire.ParseStable(raw)
	if err != nil {
		t.Fatal(err)
	}
	options := LinkOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath, Obligation: "OCM-TEST-001",
		Hunks: []string{cem.Change.Hunks[0].ID}, ExpectedBase: base, Target: target,
	}
	if _, err := PrepareOCM(context.Background(), root, PrepareOptions{
		MapPath: options.MapPath, CEMPath: options.CEMPath, IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}); err != nil {
		t.Fatal(err)
	}
	return root, options
}
