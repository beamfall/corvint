package lrfrepo

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// TestOCMV0951LinkKeepsOtherTestPaths pins OCM-V0-951 (V1-0585): one requirement
// linked to an assertion in a second test file keeps its first file's claim, so
// both immutable test-path anchors are retained and the map still verifies;
// relinking a path replaces only that path's claims.
func TestOCMV0951LinkKeepsOtherTestPaths(t *testing.T) {
	root, base, _ := makeOCMPrepareRepository(t)
	sources := map[string]string{
		"reducer_test.go": "package example\nimport \"testing\"\nfunc TestReducer(t *testing.T) {\n t.Run(\"OCM-TEST-001 reducer keeps the review\", func(t *testing.T) {})\n t.Run(\"OCM-TEST-001 reducer second case\", func(t *testing.T) {})\n}\n",
		"cli_test.go":     "package example\nimport \"testing\"\nfunc TestCLI(t *testing.T) {\n t.Run(\"OCM-TEST-001 cli keeps the review\", func(t *testing.T) {})\n}\n",
	}
	for path, source := range sources {
		writeOCMPrepareFile(t, root, path, source)
		gitOCMPrepare(t, root, "add", path)
	}
	gitOCMPrepare(t, root, "commit", "-qm", "test anchors in two files")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareStableCEM(t, root, base, target)
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, ordinal := range []string{"1", "2"} {
		if _, err := session.Cite(context.Background(), workflow.CiteOptions{
			MapPath: wire.ExcludedCEMPath, Hunk: ordinal, EvidencePath: "docs/specs/intent.md", Lines: "5:5", Relation: "specification",
		}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(raw)
	if err != nil || len(document.Change.Hunks) < 2 {
		t.Fatalf("stable map: err=%v", err)
	}
	first, other := document.Change.Hunks[0].ID, document.Change.Hunks[1].ID
	if _, err := PrepareOCM(context.Background(), root, PrepareOptions{
		MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath, IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	}); err != nil {
		t.Fatal(err)
	}
	claimFor := func(path, caseText string) ocmClaim {
		t.Helper()
		claims, err := enumerateClaims(path, gitOCMPrepare(t, root, "rev-parse", target+":"+path), []byte(sources[path]))
		if err != nil {
			t.Fatal(err)
		}
		for _, claim := range claims {
			if strings.Contains(claim.selector, "/case:") && strings.Contains(claim.selector, caseText) {
				return claim
			}
		}
		t.Fatalf("no %q case claim in %s: %+v", caseText, path, claims)
		return ocmClaim{}
	}
	link := func(path, caseText, hunk string) {
		t.Helper()
		if _, err := LinkOCM(context.Background(), root, LinkOptions{
			MapPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath, Obligation: "OCM-TEST-001", Hunks: []string{hunk},
			TestPath: path, Claims: []string{claimFor(path, caseText).selector}, ExpectedBase: base, Target: target,
		}); err != nil {
			t.Fatalf("link %s: %v", path, err)
		}
	}
	linked := func() (claims, hunks []string) {
		t.Helper()
		checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
			OCMPath: DefaultOCMMapPath, CEMPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target,
			ExpectedBaseGiven: true, TargetGiven: true,
		})
		if err != nil || checked.Verification["valid"] != true {
			t.Fatalf("linked OCM read: err=%v result=%+v", err, checked)
		}
		value, parseErr := wire.Parse(checked.OCMRaw)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		obligations, err := requireArray(value.Obj, "obligations")
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range obligations {
			if id, _ := item.Obj.Get("id"); id.Str == "OCM-TEST-001" {
				claims, hunks = stringMembers(item.Obj, "claimIds"), stringMembers(item.Obj, "hunkIds")
				sort.Strings(claims)
				sort.Strings(hunks)
				return claims, hunks
			}
		}
		t.Fatal("no OCM-TEST-001 obligation")
		return nil, nil
	}
	reducer, second, cli := claimFor("reducer_test.go", "reducer-keep"), claimFor("reducer_test.go", "second-case"), claimFor("cli_test.go", "cli-keep")
	want := func(claims ...ocmClaim) []string {
		ids := []string{}
		for _, claim := range claims {
			ids = append(ids, claim.id)
		}
		sort.Strings(ids)
		return ids
	}

	both := []string{first, other}
	sort.Strings(both)
	check := func(step string, wantClaims, wantHunks []string) {
		t.Helper()
		claims, hunks := linked()
		if strings.Join(claims, ",") != strings.Join(wantClaims, ",") || strings.Join(hunks, ",") != strings.Join(wantHunks, ",") {
			t.Fatalf("%s: claims %v hunks %v, want claims %v hunks %v", step, claims, hunks, wantClaims, wantHunks)
		}
	}

	link("reducer_test.go", "reducer-keep", first)
	check("one path", want(reducer), []string{first})
	// A second path keeps the first path's claim, and the kept link's hunk joins the new one.
	link("cli_test.go", "cli-keep", other)
	check("two paths", want(reducer, cli), both)
	// Relinking the first path replaces that path's claim, keeps the other path's, and keeps the
	// union of hunks while a claim is kept.
	link("reducer_test.go", "second-case", first)
	check("relinked path", want(second, cli), both)
}
