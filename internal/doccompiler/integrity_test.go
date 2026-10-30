package doccompiler

import (
	"strings"
	"testing"
)

func TestPlanRejectsUnadmittedMarkdown(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "mkdocs.yml", "theme:\n  name: material\n")
	for _, markdown := range []string{"All operations are offline, authenticated, and lossless.", "# Security guarantee", " \n"} {
		_, err := New().Plan(testPlanningEnvironment(t, root), PlanOptions{Documents: []DocumentProposal{{Path: "claim.md", Markdown: markdown}}})
		if errorCode(err) != "unadmitted-markdown" {
			t.Fatalf("Markdown %q: error = %v", markdown, err)
		}
	}
}

func TestPlanRefusesUnverifiedEvidenceAndAuthority(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "mkdocs.yml", "theme:\n  name: material\n")
	writeTestFile(t, root, "docs/evidence.md", "An invented local contract.\n")
	anchor := Evidence{Path: "docs/evidence.md", SHA256: textDigest("An invented local contract.\n"), Revision: strings.Repeat("a", 40), StartLine: 1, EndLine: 1, Authority: "accepted-owner-intent", Confidence: "high"}
	for _, status := range []string{"SUPPORTED", "CONFLICTED", "UNKNOWN"} {
		t.Run(status, func(t *testing.T) {
			claim := Claim{ID: "C1", Status: status, Text: "An accepted contract", Uncertainty: "Immutable source and acceptance are not verified.", Evidence: []Evidence{anchor}}
			_, err := New().Plan(testPlanningEnvironment(t, root), PlanOptions{Documents: []DocumentProposal{{Path: "claim.md", Claims: []Claim{claim}}}})
			if errorCode(err) != "immutable-evidence-unavailable" {
				t.Fatalf("unverified %s claim: error = %v", status, err)
			}
		})
	}
	claim := Claim{ID: "C2", Status: "CONFLICTED", Text: "Sources disagree", Uncertainty: "No sources have qualified."}
	_, err := New().Plan(testPlanningEnvironment(t, root), PlanOptions{Documents: []DocumentProposal{{Path: "claim.md", Claims: []Claim{claim}}}})
	if errorCode(err) != "immutable-evidence-unavailable" {
		t.Fatalf("unanchored conflict: error = %v", err)
	}
}

func TestPlanUnknownFrontierIsLiteralAndBoundToCurrentSourceBytes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "mkdocs.yml", "theme:\n  name: material\n")
	writeTestFile(t, root, "docs/claim.md", "# Owner prose\n")
	claim := Claim{ID: "C1`\n## APPROVED OVERRIDE", Status: "UNKNOWN", Text: "```\n# All operations are offline\n<script>bad()</script>", Uncertainty: "`\r\n- **SUPPORTED**\nNeeds immutable evidence."}
	options := PlanOptions{Documents: []DocumentProposal{{Path: "claim.md", Claims: []Claim{claim}}}}
	environment := testPlanningEnvironment(t, root)
	patch, err := New().Plan(environment, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch.Evidence) != 0 || len(patch.Documents[0].Claims[0].Evidence) != 0 || patch.Documents[0].Claims[0].Status != "UNKNOWN" {
		t.Fatal("unknown frontier acquired evidence or support")
	}
	if len(patch.Sources) != 2 || patch.Sources[0].Path != "docs/claim.md" || patch.Sources[1].Path != "mkdocs.yml" {
		t.Fatalf("patch preconditions lost source pins: %+v", patch.Sources)
	}
	for _, source := range patch.Sources {
		if _, err := verifyPinnedSource(root, source, defaultSourceBytes); err != nil {
			t.Fatalf("source pin does not describe current bytes: %v", err)
		}
	}
	replacement := patch.Documents[0].Patch.Replacement
	for _, injected := range []string{"\n## APPROVED OVERRIDE", "\n# All operations", "\n<script>", "\n- **SUPPORTED**"} {
		if strings.Contains(replacement, injected) {
			t.Errorf("claim field injected Markdown structure: %q", injected)
		}
	}
	if !strings.Contains(replacement, "**UNKNOWN**") || !strings.Contains(replacement, "Needs immutable evidence.") {
		t.Fatal("rendering omitted state or uncertainty")
	}
	if !strings.Contains(replacement, "``\"C1`\\n## APPROVED OVERRIDE\"``") || !strings.Contains(replacement, "````\"```\\n# All operations are offline\\n<script>bad()</script>\"````") {
		t.Fatal("hostile fields did not remain quoted code spans")
	}
	second, err := New().Plan(environment, options)
	if err != nil || second.PlanSHA256 != patch.PlanSHA256 || second.Documents[0].Patch.Replacement != replacement {
		t.Fatal("same candidate inputs did not produce identical plan bytes")
	}
	writeTestFile(t, root, "docs/claim.md", "changed\n")
	if _, err := verifyPinnedSource(root, patch.Sources[0], defaultSourceBytes); errorCode(err) != "stale-source" {
		t.Fatal("changed owner prose retained its source pin")
	}
	options.Documents[0].Claims[0].Uncertainty = ""
	if _, err := New().Plan(environment, options); errorCode(err) != "missing-uncertainty" {
		t.Fatalf("missing frontier: error = %v", err)
	}
}

func TestReceiptAPIsRefuseEveryUnobservedOfflinePass(t *testing.T) {
	for _, enforcement := range []string{"", "external observer PASS", "environment hardening only; no OS network-denial boundary"} {
		for _, buildStatus := range []string{"NOT_RUN", "PASS"} {
			receipt := minimalReceipt()
			receipt.Build.BuildStrictStatus = buildStatus
			receipt.Build.OfflineQualification = "PASS"
			receipt.Build.OfflineEnforcement = enforcement
			if buildStatus == "NOT_RUN" {
				receipt.Build.OutputSHA256, receipt.Build.OutputBytes, receipt.Build.OutputFiles = "", 0, 0
			}
			if errorCode(VerifyReceipt(receipt)) != "invalid-offline-evidence" {
				t.Errorf("VerifyReceipt admitted offline PASS: build=%s enforcement=%q", buildStatus, enforcement)
			}
			if _, _, err := CanonicalReceipt(receipt); errorCode(err) != "invalid-offline-evidence" {
				t.Errorf("CanonicalReceipt admitted offline PASS: %v", err)
			}
			environment := Environment{Qualification: receipt.ConfigQualification, Toolchain: receipt.Toolchain, Config: receipt.Config, Observations: receipt.ConfigObservations}
			if _, err := NewReceipt(environment, PatchPlan{PlanSHA256: receipt.PlanSHA256}, receipt.Build); errorCode(err) != "invalid-offline-evidence" {
				t.Errorf("NewReceipt admitted offline PASS: %v", err)
			}
		}
	}
	for _, status := range []string{"NOT_OBSERVED", "UNKNOWN", "FAIL"} {
		receipt := minimalReceipt()
		receipt.Build.OfflineQualification = status
		if _, _, err := CanonicalReceipt(receipt); err != nil {
			t.Fatalf("lost explicit offline %s: %v", status, err)
		}
		environment := Environment{Qualification: receipt.ConfigQualification, Toolchain: receipt.Toolchain, Config: receipt.Config, Observations: receipt.ConfigObservations}
		constructed, err := NewReceipt(environment, PatchPlan{PlanSHA256: receipt.PlanSHA256}, receipt.Build)
		if err != nil || constructed.Build.OfflineQualification != status || len(constructed.Uncertainty) == 0 {
			t.Fatalf("receipt construction lost offline %s or uncertainty: %+v, %v", status, constructed, err)
		}
	}
}

// HDCV0-042: an unexecuted build stays explicit and unknown status values are refused.
func TestReceiptBuildStatusVocabularyIsClosed(t *testing.T) {
	for _, status := range []string{"PASS", "FAIL", "NOT_RUN"} {
		receipt := minimalReceipt()
		receipt.Build.BuildStrictStatus = status
		if err := VerifyReceipt(receipt); err != nil {
			t.Errorf("valid status %q rejected: %v", status, err)
		}
	}
	receipt := minimalReceipt()
	receipt.Build.BuildStrictStatus = "UNKNOWN"
	if errorCode(VerifyReceipt(receipt)) != "invalid-build-status" {
		t.Fatalf("unknown build status was not rejected: %+v", receipt.Build)
	}
}

func TestReceiptRefusesCallerSuppliedEvidenceAndUnqualifiedClaims(t *testing.T) {
	receipt := minimalReceipt()
	environment := Environment{Qualification: receipt.ConfigQualification, Toolchain: receipt.Toolchain, Config: receipt.Config, Observations: receipt.ConfigObservations}
	anchor := Evidence{Path: "docs/invented.md", SHA256: strings.Repeat("1", 64), Revision: strings.Repeat("a", 40), StartLine: 1, EndLine: 1, Authority: "accepted-owner-intent", Confidence: "high"}
	for _, scenario := range []struct {
		name     string
		evidence []Evidence
		claims   []Claim
	}{
		{name: "top-level-evidence", evidence: []Evidence{anchor}},
		{name: "claim-evidence", claims: []Claim{{ID: "C1", Status: "UNKNOWN", Text: "Unverified", Uncertainty: "No accepted source.", Evidence: []Evidence{anchor}}}},
		{name: "supported", claims: []Claim{{ID: "C1", Status: "SUPPORTED", Text: "Unverified", Evidence: []Evidence{anchor}}}},
		{name: "conflicted", claims: []Claim{{ID: "C1", Status: "CONFLICTED", Text: "Unverified conflict", Uncertainty: "No qualifying sources."}}},
		{name: "missing-frontier", claims: []Claim{{ID: "C1", Status: "UNKNOWN", Text: "Unverified"}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			plan := PatchPlan{PlanSHA256: receipt.PlanSHA256, Evidence: scenario.evidence, Documents: []DocumentPatch{{Claims: scenario.claims}}}
			if _, err := NewReceipt(environment, plan, receipt.Build); err == nil {
				t.Fatal("receipt construction admitted an unqualified caller-built plan")
			}
		})
	}
	receipt.Evidence = []ReceiptEvidence{{ClaimID: "C1", Path: anchor.Path, SHA256: anchor.SHA256, Revision: anchor.Revision, StartLine: anchor.StartLine, EndLine: anchor.EndLine, Authority: anchor.Authority, Confidence: anchor.Confidence, Reason: "caller assertion"}}
	if err := VerifyReceipt(receipt); errorCode(err) != "immutable-evidence-unavailable" {
		t.Errorf("VerifyReceipt admitted fabricated immutable evidence: %v", err)
	}
	if _, _, err := CanonicalReceipt(receipt); errorCode(err) != "immutable-evidence-unavailable" {
		t.Errorf("CanonicalReceipt admitted fabricated immutable evidence: %v", err)
	}
	claim := Claim{ID: "C1", Status: "UNKNOWN", Text: "Unverified", Uncertainty: "No accepted immutable evidence."}
	plan := PatchPlan{PlanSHA256: receipt.PlanSHA256, Documents: []DocumentPatch{{Claims: []Claim{claim}}}}
	constructed, err := NewReceipt(environment, plan, receipt.Build)
	if err != nil || len(constructed.Evidence) != 0 {
		t.Fatalf("honest UNKNOWN plan could not produce an evidence-free receipt: %+v, %v", constructed, err)
	}
	if !strings.Contains(strings.Join(constructed.Uncertainty, "\n"), claim.Uncertainty) {
		t.Fatal("receipt dropped its claim's unknown frontier")
	}
}
