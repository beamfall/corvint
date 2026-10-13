package lrf

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func TestDeletionDoesNotUseOldBasenameTerms(t *testing.T) {
	context := canonicalOCMContext()
	hunkID := "hunk:sha256:" + strings.Repeat("1", 64)
	evidenceID := "evidence:sha256:" + strings.Repeat("a", 64)
	oldPath := "src/alpha_bravo.go"
	request := Request{
		Context:  context,
		Evidence: []Evidence{{ID: evidenceID, Path: "docs/alpha.md", Span: []byte("alpha")}},
		Hunks: []Hunk{{
			ID: hunkID, OldPath: &oldPath, Disposition: "supported", Ordinal: 0,
			Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: "specification"}},
		}},
		Obligations: []Obligation{{
			ID: "LRF-V0-001", Statement: []byte("- `LRF-V0-001`: alpha.\n"),
			HunkIDs: []string{hunkID}, Ordinal: 0,
		}},
	}
	limits := DefaultLimits()
	limits.Terms = 1
	result, err := EvaluateWithLimits(request, limits)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Results()[1][5]; got != "rejected" {
		t.Fatalf("deletion OCM outcome = %q, want rejected", got)
	}
	if got := result.Issues()[1][4]; got != "obligation-hunk-mismatch" {
		t.Fatalf("deletion OCM issue = %q, want obligation-hunk-mismatch", got)
	}
}

func TestContentIDBesideNonASCIIByteIsNotRedacted(t *testing.T) {
	hunkID := "hunk:sha256:" + strings.Repeat("1", 64)
	evidenceID := "evidence:sha256:" + strings.Repeat("a", 64)
	contentID := "hunk:sha256:cafe" + strings.Repeat("0", 60)
	newPath := "src/a.py"
	for name, added := range map[string]string{"leading": "\xc3\xa9" + contentID + "\n", "trailing": contentID + "\xc3\xa9\n"} {
		t.Run(name, func(t *testing.T) {
			result, err := Evaluate(Request{
				Context:  canonicalOCMContext(),
				Evidence: []Evidence{{ID: evidenceID, Path: "docs/id.md", Span: []byte("cafe")}},
				Hunks: []Hunk{{
					ID: hunkID, OldPath: &newPath, NewPath: &newPath, Added: []byte(added), Disposition: "supported",
					Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: "specification"}},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := result.Results()[0][5]; got != "cem-lexical-v0" {
				t.Fatalf("content ID beside a non-ASCII byte was redacted: outcome %q", got)
			}
		})
	}
}

func TestRequirementIDInAddedBytesSuppliesNoTerm(t *testing.T) {
	hunkID := "hunk:sha256:" + strings.Repeat("1", 64)
	evidenceID := "evidence:sha256:" + strings.Repeat("a", 64)
	newPath := "src/a.py"
	for added, want := range map[string]string{
		"see WIDGET-001\n":         "rejected",
		"see WIDGET-001-0023\n":    "cem-lexical-v0",
		"see xWIDGET-001\n":        "cem-lexical-v0",
		"see \xc3\xa9WIDGET-001\n": "rejected",
	} {
		result, err := Evaluate(Request{
			Context:  canonicalOCMContext(),
			Evidence: []Evidence{{ID: evidenceID, Path: "docs/id.md", Span: []byte("widget")}},
			Hunks: []Hunk{{
				ID: hunkID, OldPath: &newPath, NewPath: &newPath, Added: []byte(added), Disposition: "supported",
				Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: "specification"}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Results()[0][5]; got != want {
			t.Fatalf("added %q: outcome %q, want %q", added, got, want)
		}
	}
}

func TestFrozenDefaultsAndIssuedBytes(t *testing.T) {
	mutated := DefaultLimits()
	mutated.Terms = maxTerms + 1
	if got := DefaultLimits().Terms; got != maxTerms {
		t.Fatalf("fresh default term cap = %d, want %d", got, maxTerms)
	}
	if raw := CanonicalBytes(Result{}); raw != nil {
		t.Fatalf("caller-constructed result serialized as %q", raw)
	}
	tooSmall := DefaultLimits()
	tooSmall.OutputBytes = 1
	_, err := EvaluateWithLimits(Request{Context: canonicalOCMContext()}, tooSmall)
	typed, ok := err.(*Error)
	if !ok || typed.Code != "invalid-lrf-request" {
		t.Fatalf("too-small bound-document limit error = %v", err)
	}
}

func canonicalOCMContext() Context {
	target := strings.Repeat("2", 40)
	excluded := wire.ExcludedCEMPath
	ocmSpec := OCMSpec
	ocmDigest := strings.Repeat("c", 64)
	intentPath := "docs/intent.md"
	intentBlob := strings.Repeat("3", 40)
	start, end := int64(0), int64(32)
	intentDigest := strings.Repeat("d", 64)
	return Context{
		CEMSpec: frozenContextCEM02, CEMMapSHA256: strings.Repeat("a", 64), PatchSource: "canonical-derived",
		BaseRevision: strings.Repeat("1", 40), TargetRevision: &target, PatchSHA256: strings.Repeat("b", 64),
		ExcludedPath: &excluded, OCMSpec: &ocmSpec, OCMMapSHA256: &ocmDigest, IntentPath: &intentPath,
		IntentBlobOID: &intentBlob, IntentStart: &start, IntentEnd: &end, IntentSpanSHA256: &intentDigest,
	}
}
