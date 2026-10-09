package lrfrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

func TestOCMExistingFalseLinkRejected(t *testing.T) {
	t.Run("OCM-V0-006 verifier rejects a persisted nonanchoring claim", func(t *testing.T) {
		root, options, document, claims := makeOCMAnchorFixture(t)
		_, falseClaim := falseAnchorClaim(t, claims)
		linked, err := linkObligation(document, options.Obligation, options.Hunks, []ocmClaim{falseClaim})
		if err != nil {
			t.Fatal(err)
		}
		raw := canonicalOCMBytes(linked)
		path := filepath.Join(root, options.MapPath)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
			OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: options.ExpectedBase,
			Target: options.Target, ExpectedBaseGiven: true, TargetGiven: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		issues := checked.Verification["issues"].([]any)
		if checked.Verification["valid"] != false || len(issues) != 1 || issues[0].(map[string]any)["code"] != "claim-obligation-mismatch" {
			t.Fatalf("false link verdict: %v", checked.Verification)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, raw) {
			t.Fatalf("verifier mutated map: %v", err)
		}
	})
}

func TestOCMLinkRejectsFalseAnchorBeforePublication(t *testing.T) {
	t.Run("OCM-V0-005 ordinal selector and claim ID reject false anchors", func(t *testing.T) {
		root, options, _, claims := makeOCMAnchorFixture(t)
		path := filepath.Join(root, options.MapPath)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ordinal, falseClaim := falseAnchorClaim(t, claims)
		for _, selector := range []string{ordinal, falseClaim.selector, falseClaim.id} {
			for _, output := range []string{"", ".corvint/existing.ocm.json", ".corvint/new.ocm.json"} {
				options.Claims, options.Output = []string{selector}, output
				if output == ".corvint/existing.ocm.json" {
					if err := os.WriteFile(filepath.Join(root, output), []byte("existing destination"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := LinkOCM(context.Background(), root, options); CodeOf(err) != "claim-obligation-mismatch" {
					t.Fatalf("selector=%s output=%s: %v", selector, output, err)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("input changed: %v", err)
				}
				if output == ".corvint/existing.ocm.json" {
					after, err := os.ReadFile(filepath.Join(root, output))
					if err != nil || string(after) != "existing destination" {
						t.Fatalf("destination changed: %v", err)
					}
				}
				if output == ".corvint/new.ocm.json" {
					if _, err := os.Stat(filepath.Join(root, output)); !os.IsNotExist(err) {
						t.Fatalf("destination created: %v", err)
					}
				}
			}
		}
	})
	t.Run("OCM-V0-005 exact anchor and full hunk ID link and verify", func(t *testing.T) {
		root, options, _, claims := makeOCMAnchorFixture(t)
		for _, claim := range claims {
			if strings.Contains(claim.selector, "/case:") {
				options.Claims = []string{claim.selector}
			}
		}
		if _, err := LinkOCM(context.Background(), root, options); err != nil {
			t.Fatal(err)
		}
		checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
			OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: options.ExpectedBase,
			Target: options.Target, ExpectedBaseGiven: true, TargetGiven: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if checked.Verification["valid"] != true {
			t.Fatalf("valid link verdict: %v", checked.Verification)
		}
	})
}

func TestOCMLinkMutatesTheVerifiedMapBytes(t *testing.T) {
	root, options, document, claims := makeOCMAnchorFixture(t)
	_, retained := falseAnchorClaim(t, claims)
	for _, claim := range claims {
		if strings.Contains(claim.selector, "/case:") {
			options.Claims = []string{claim.selector}
		}
	}
	merged, err := mergeClaims(document, []ocmClaim{retained})
	if err != nil {
		t.Fatal(err)
	}
	objectMember(document.Obj, "claims", merged)
	verifiedBytes := canonicalOCMBytes(document)
	options.beforeVerification = func() {
		if err := os.WriteFile(filepath.Join(root, options.MapPath), verifiedBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LinkOCM(context.Background(), root, options); err != nil {
		t.Fatal(err)
	}
	linkedBytes, err := os.ReadFile(filepath.Join(root, options.MapPath))
	if err != nil {
		t.Fatal(err)
	}
	linked, err := wire.Parse(linkedBytes)
	if err != nil {
		t.Fatal(err)
	}
	linkedClaims, _ := linked.Obj.Get("claims")
	if len(linkedClaims.Arr) != 2 {
		t.Fatalf("linked claims=%d want=2", len(linkedClaims.Arr))
	}
}

func makeOCMAnchorFixture(t *testing.T) (string, LinkOptions, wire.Value, []ocmClaim) {
	t.Helper()
	return makeOCMAnchorFixtureCase(t, "OCM-TEST-001 exact anchor")
}

// makeOCMAnchorFixtureCase is makeOCMAnchorFixture with the one t.Run case
// literal supplied, so token-boundary tests vary only the anchor bytes.
func makeOCMAnchorFixtureCase(t *testing.T, caseName string) (string, LinkOptions, wire.Value, []ocmClaim) {
	t.Helper()
	root, base, _ := makeOCMPrepareRepository(t)
	const testPath = "example_test.go"
	source := `package example
import "testing"
func TestAUnrelated(t *testing.T) {}
func TestAnchors(t *testing.T) {
 t.Run("` + caseName + `", func(t *testing.T) {})
}
`
	writeOCMPrepareFile(t, root, testPath, source)
	gitOCMPrepare(t, root, "add", testPath)
	gitOCMPrepare(t, root, "commit", "-qm", "test anchors")
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
	cem, err := wire.ParseMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	options := LinkOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath, Obligation: "OCM-TEST-001",
		Hunks: []string{cem.Hunks[0].ID}, TestPath: testPath, ExpectedBase: base, Target: target,
	}
	prepared, err := PrepareOCM(context.Background(), root, PrepareOptions{
		MapPath: options.MapPath, CEMPath: options.CEMPath, IntentPath: "docs/specs/intent.md", ExpectedBase: base, Target: target,
	})
	if err != nil {
		t.Fatal(err)
	}
	blobOID := gitOCMPrepare(t, root, "rev-parse", target+":"+testPath)
	claims, err := enumerateClaims(testPath, blobOID, []byte(source))
	if err != nil || len(claims) != 3 {
		t.Fatalf("claims=%v err=%v", claims, err)
	}
	return root, options, prepared.Document, claims
}

// caseClaim returns the fixture's one t.Run case claim.
func caseClaim(t *testing.T, claims []ocmClaim) ocmClaim {
	t.Helper()
	for _, claim := range claims {
		if strings.Contains(claim.selector, "/case:") {
			return claim
		}
	}
	t.Fatal("no case claim")
	return ocmClaim{}
}

// TestOCMLinkExplainsRequirementTokenBoundary covers V1-0555: an anchor that
// names the obligation ID only inside a longer token stays refused, and the
// link refusal says why; a space-separated ID with the same normalized
// selector links. The matcher and the verifier's issue message are unchanged.
func TestOCMLinkExplainsRequirementTokenBoundary(t *testing.T) {
	const base = "claim anchor lacks the exact obligation ID"
	t.Run("OCM-V0-017 adjoined IDs stay refused and the refusal names the boundary", func(t *testing.T) {
		for _, tc := range []struct{ name, hint string }{
			{"OCM-TEST-001-exact-anchor", "; OCM-TEST-001 occurs only inside a longer token ('-' after it)"},
			{"XOCM-TEST-001 exact anchor", "; OCM-TEST-001 occurs only inside a longer token ('X' before it)"},
			{"OCM-TEST-0012 exact anchor", "; OCM-TEST-001 occurs only inside a longer token ('2' after it)"},
			{"_OCM-TEST-001_ exact anchor", "; OCM-TEST-001 occurs only inside a longer token ('_' before it)"},
			{"OCM-TEST-002 exact anchor", ""},
		} {
			root, options, _, claims := makeOCMAnchorFixtureCase(t, tc.name)
			path := filepath.Join(root, options.MapPath)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			options.Claims = []string{caseClaim(t, claims).selector}
			_, err = LinkOCM(context.Background(), root, options)
			var refusal *Error
			if CodeOf(err) != "claim-obligation-mismatch" || !errors.As(err, &refusal) || !strings.HasPrefix(refusal.Message, base+tc.hint) {
				t.Fatalf("%q: %v", tc.name, err)
			}
			if tc.hint == "" && refusal.Message != base {
				t.Fatalf("%q: absent ID gained a hint: %q", tc.name, refusal.Message)
			}
			if tc.hint != "" && !strings.HasSuffix(refusal.Message, `separate the exact ID with a space or the anchor edge, for example t.Run("OCM-TEST-001 case name", ...)`) {
				t.Fatalf("%q: no separated example: %q", tc.name, refusal.Message)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(before, after) {
				t.Fatalf("%q: refused link changed the map: %v", tc.name, err)
			}
		}
	})
	t.Run("OCM-V0-017 a space-separated ID with the same normalized selector links", func(t *testing.T) {
		_, _, _, hyphenClaims := makeOCMAnchorFixtureCase(t, "OCM-TEST-001-exact-anchor")
		root, options, _, claims := makeOCMAnchorFixtureCase(t, "OCM-TEST-001 exact anchor")
		selector := caseClaim(t, claims).selector
		if selector != caseClaim(t, hyphenClaims).selector || selector != "test:TestAnchors/case:ocm-test-exact-anchor" {
			t.Fatalf("normalized selectors differ: %q %q", selector, caseClaim(t, hyphenClaims).selector)
		}
		options.Claims = []string{selector}
		if _, err := LinkOCM(context.Background(), root, options); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("OCM-V0-005 verifier issue message is unchanged for an adjoined ID", func(t *testing.T) {
		root, options, document, claims := makeOCMAnchorFixtureCase(t, "OCM-TEST-001-exact-anchor")
		linked, err := linkObligation(document, options.Obligation, options.Hunks, []ocmClaim{caseClaim(t, claims)})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, options.MapPath), canonicalOCMBytes(linked), 0600); err != nil {
			t.Fatal(err)
		}
		checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
			OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: options.ExpectedBase,
			Target: options.Target, ExpectedBaseGiven: true, TargetGiven: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		issues, _ := checked.Verification["issues"].([]any)
		if len(issues) != 1 || !reflect.DeepEqual(issues[0], map[string]any{"code": "claim-obligation-mismatch", "message": base}) {
			t.Fatalf("verifier issue changed: %v", checked.Verification)
		}
	})
	t.Run("OCM-V0-005 matcher semantics are unchanged", func(t *testing.T) {
		for anchor, exact := range map[string]bool{
			`"CVI-V0-002 strict grammar"`: true, `"CVI-V0-002-strict-grammar"`: false, `"CVI-V0-0021 x"`: false,
			`"ACVI-V0-002 x"`: false, `"x_CVI-V0-002"`: false, `"(CVI-V0-002)"`: true, `CVI-V0-002`: true,
		} {
			if containsExactRequirement([]byte(anchor), "CVI-V0-002") != exact {
				t.Fatalf("%s: exact=%t", anchor, !exact)
			}
			if hint := requirementBoundaryHint([]byte(anchor), "CVI-V0-002"); (hint == "") != exact {
				t.Fatalf("%s: hint=%q", anchor, hint)
			}
		}
		if requirementBoundaryHint([]byte(`"OTHER-V0-001"`), "CVI-V0-002") != "" {
			t.Fatal("hint for an absent ID")
		}
	})
}

// falseAnchorClaim returns the first top-level test claim, which carries no obligation, and its
// ordinal selector; claim ids sort by digest, so the position is not fixed.
func falseAnchorClaim(t *testing.T, claims []ocmClaim) (string, ocmClaim) {
	t.Helper()
	for index, claim := range claims {
		if !strings.Contains(claim.selector, "/case:") {
			return strconv.Itoa(index + 1), claim
		}
	}
	t.Fatal("no top-level claim")
	return "", ocmClaim{}
}

func TestOCMClaimBlobOIDVerifiedWhenPathRepeats(t *testing.T) {
	t.Run("OCM-V0-005 a second claim on a verified path must still match its blob OID", func(t *testing.T) {
		root, options, _, claims := makeOCMAnchorFixture(t)
		var good ocmClaim
		for _, claim := range claims {
			if strings.Contains(claim.selector, "/case:") {
				options.Claims = []string{claim.selector}
				good = claim
			}
		}
		if _, err := LinkOCM(context.Background(), root, options); err != nil {
			t.Fatal(err)
		}
		forged := good
		for ordinal := 1; forged.id <= good.id; ordinal++ {
			forged.blobOID = fmt.Sprintf("%040x", ordinal)
			forged.id = claimIdentity(forged)
		}
		path := filepath.Join(root, options.MapPath)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		merged, err := mergeClaims(document, []ocmClaim{forged})
		if err != nil {
			t.Fatal(err)
		}
		objectMember(document.Obj, "claims", merged)
		if err := os.WriteFile(path, canonicalOCMBytes(document), 0600); err != nil {
			t.Fatal(err)
		}
		checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
			OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: options.ExpectedBase,
			Target: options.Target, ExpectedBaseGiven: true, TargetGiven: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		issues, _ := checked.Verification["issues"].([]any)
		if checked.Verification["valid"] != false || len(issues) != 1 || issues[0].(map[string]any)["code"] != "repository-object-unavailable" {
			t.Fatalf("forged blob OID verdict: %v", checked.Verification)
		}
	})
}

// Decision 0273: an explicit mark or link output naming the bound CEM file is
// refused like prepare, leaving the CEM bytes unchanged.
func TestOCMMarkAndLinkRefuseOutputAliasOfCEMPath(t *testing.T) {
	mutations := map[string]func(root string, options LinkOptions, output string) error{
		"mark": func(root string, options LinkOptions, output string) error {
			_, err := MarkOCM(context.Background(), root, MarkOptions{
				MapPath: options.MapPath, Obligation: "OCM-TEST-001", Reason: "no-test-claim", Output: output,
			})
			return err
		},
		"link": func(root string, options LinkOptions, output string) error {
			options.Output = output
			_, err := LinkOCM(context.Background(), root, options)
			return err
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root, options, _, claims := makeOCMAnchorFixture(t)
			for _, claim := range claims {
				if strings.Contains(claim.selector, "/case:") {
					options.Claims = []string{claim.selector}
				}
			}
			cemFile := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
			before, err := os.ReadFile(cemFile)
			if err != nil {
				t.Fatal(err)
			}
			for _, output := range []string{wire.ExcludedCEMPath, cemFile} {
				err := mutate(root, options, output)
				after, _ := os.ReadFile(cemFile)
				if CodeOf(err) != "output-path-conflict" || string(after) != string(before) {
					t.Fatalf("--output %s: code=%q err=%v cemChanged=%t", output, CodeOf(err), err, string(after) != string(before))
				}
			}
		})
	}
}
