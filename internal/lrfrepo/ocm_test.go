package lrfrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/analyzerpython"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func TestRequirementEnumerationAcceptsArtifactGoPrefix(t *testing.T) {
	t.Run("ARTIFACT-GO-V0-001 accepted requirement prefix", func(t *testing.T) {
		data := []byte("# Archive intent\n\n## Requirements\n\n- `ARTIFACT-GO-V0-001`: archive inputs remain pinned.\n\n## Non-goals\n")
		_, requirements, _, err := requirementsFromBlob("docs/specs/archive.md", "0123456789abcdef", data)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"ARTIFACT-GO-V0-001"}; !reflect.DeepEqual(requirements, want) {
			t.Fatalf("requirements=%v want=%v", requirements, want)
		}
	})
}

func TestRequirementEnumerationAcceptsBoldIDDelimiters(t *testing.T) {
	data := []byte("# Snapshot batch intent\n\n## Requirements\n\nProse mentioning **PROSE-V0-001.** is not a requirement.\n\n- `**CODE-V0-001.**` is an inline code span, not a requirement.\n- **SBQ-V0-001:** snapshot inputs remain pinned.\n- **SBQ-V0-002.** snapshot outputs remain pinned.\n\n## Non-goals\n")
	_, requirements, _, err := requirementsFromBlob("docs/specs/snapshot-batch-v0.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"SBQ-V0-001", "SBQ-V0-002"}; !reflect.DeepEqual(requirements, want) {
		t.Fatalf("requirements=%v want=%v", requirements, want)
	}
}

func TestOCMLinkedVerificationAcceptsRequirementLineForms(t *testing.T) {
	const (
		hunkID  = "hunk:sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		claimID = "claim:sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)
	cases := []struct {
		id        string
		statement string
	}{
		{id: "OCM-FORM-001", statement: "- `OCM-FORM-001`: inline code remains accepted.\n"},
		{id: "OCM-FORM-002", statement: "- **OCM-FORM-002:** bold colon remains accepted.\n"},
		{id: "OCM-FORM-003", statement: "- **OCM-FORM-003.** bold period remains accepted.\n"},
	}
	for _, test := range cases {
		t.Run(test.id, func(t *testing.T) {
			document := &ocmDocument{obligations: []ocmObligation{{
				id: test.id, disposition: "linked", reason: "change-and-test-linked",
				hunkIDs: []string{hunkID}, claimIDs: []string{claimID},
			}}}
			cem := &wire.Map{Hunks: []wire.Hunk{{ID: hunkID, Disposition: "supported"}}}
			got, err := verifyObligations(document, cem, []string{test.id}, []byte(test.statement), nil,
				map[string][]byte{claimID: []byte("test " + test.id)}, map[string]string{claimID: "example_test.go"})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || string(got[0].Statement) != test.statement {
				t.Fatalf("obligations=%v want statement %q", got, test.statement)
			}
		})
	}
}

func TestRequirementEnumerationMissingHeadingNamesExactHeading(t *testing.T) {
	data := []byte("# Go live-test provider\n\n## 11. AT-14 qualification requirements\n\n- **GLTP-V0-001:** test inputs remain pinned.\n")
	_, _, _, err := requirementsFromBlob("docs/specs/go-live-test-provider-v0.md", "0123456789abcdef", data)
	if got, want := CodeOf(err), "invalid-requirements-section"; got != want {
		t.Fatalf("code=%q want=%q err=%v", got, want, err)
	}
	if got, want := err.Error(), "invalid-requirements-section: intent must contain exactly one ## Requirements heading"; got != want {
		t.Fatalf("error=%q want=%q", got, want)
	}
}

func TestClaimReExtractionRejectsWeakAndNonSyntaxCandidates(t *testing.T) {
	t.Run("weak Go case", func(t *testing.T) {
		blob := []byte("package p\nfunc TestX(t any) { _ = struct{ name string }{name: \"ABC-001\"} }\n")
		anchor := []byte("ABC-001")
		start := bytes.Index(blob, anchor)
		claim := ocmClaim{
			path: "x_test.go", selector: "test:TestX/case:abc",
			span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
		}
		if claimExtractable(claim, blob) {
			t.Fatal("weak table case was accepted")
		}
	})
	t.Run("commented Go case", func(t *testing.T) {
		blob := []byte("package p\nfunc TestX(t any) {}\n// name: \"LRF-CLI-001 widget renderer\"\n")
		anchor := []byte("LRF-CLI-001 widget renderer")
		start := bytes.Index(blob, anchor)
		claim := ocmClaim{
			path: "x_test.go", selector: "test:TestX/case:lrf-cli-widget-renderer",
			span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
		}
		if claimExtractable(claim, blob) {
			t.Fatal("commented table case was accepted")
		}
	})
	t.Run("invalid Python indentation", func(t *testing.T) {
		blob := []byte("def test_widget():\n\"\"\"LRF-CLI-001 widget renderer\"\"\"\n")
		anchor := []byte("\"\"\"LRF-CLI-001 widget renderer\"\"\"")
		start := bytes.Index(blob, anchor)
		claim := ocmClaim{
			path: "test_widget.py", selector: "test:test_widget#doc",
			span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
		}
		if claimExtractable(claim, blob) {
			t.Fatal("invalid Python docstring was accepted")
		}
	})
	t.Run("unrelated Python syntax error", func(t *testing.T) {
		blob := []byte("def test_widget():\n    \"\"\"LRF-CLI-001 widget renderer\"\"\"\n\ndef broken(:\n")
		anchor := []byte("\"\"\"LRF-CLI-001 widget renderer\"\"\"")
		start := bytes.Index(blob, anchor)
		claim := ocmClaim{
			path: "test_widget.py", selector: "test:test_widget#doc",
			span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
		}
		if claimExtractable(claim, blob) {
			t.Fatal("claim from a syntactically invalid Python blob was accepted")
		}
	})
	// DR-0014: verifyOptionalOCM (the OCM/lrf leg, rejectPythonClaims=true)
	// and VerifyUniverse (the frontier/TCQ leg, rejectPythonClaims=false, in
	// internal/lrfrepo/universe.go) both reach claimExtractableIn, which
	// gates every '.py' claim through this same pythonSyntaxValid ->
	// pythonsyntax.SourceSyntaxValid call. A grammar class that
	// SourceSyntaxValid used to false-accept is refused identically on both
	// legs once the shared function is tightened — no lrfrepo-side branch
	// decides Python validity independently.
	t.Run("grammar class SourceSyntaxValid now rejects", func(t *testing.T) {
		blob := []byte("def test_widget():\n    \"\"\"LRF-CLI-001 widget renderer\"\"\"\n    x = 1e2.3\n")
		anchor := []byte("\"\"\"LRF-CLI-001 widget renderer\"\"\"")
		start := bytes.Index(blob, anchor)
		claim := ocmClaim{
			path: "test_widget.py", selector: "test:test_widget#doc",
			span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
		}
		if claimExtractable(claim, blob) {
			t.Fatal("claim from a blob with a decimal point after 'e' in a number literal was accepted")
		}
	})
	t.Run("valid Python docstring", func(t *testing.T) {
		blob := []byte("def test_widget():\n    \"\"\"LRF-CLI-001 widget renderer\"\"\"\n")
		anchor := []byte("\"\"\"LRF-CLI-001 widget renderer\"\"\"")
		start := bytes.Index(blob, anchor)
		claim := ocmClaim{
			path: "test_widget.py", selector: "test:test_widget#doc",
			span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
		}
		if !claimExtractable(claim, blob) {
			t.Fatal("valid Python docstring was rejected")
		}
	})
}

func TestPythonClaimReExtractionHonorsOCMBlobCeiling(t *testing.T) {
	for _, testcase := range []struct {
		name  string
		size  int
		valid bool
		want  bool
	}{
		{"previously rejected band", analyzerpython.MaxInputBytes + 1, true, true},
		{"invalid in previously rejected band", analyzerpython.MaxInputBytes + 1, false, false},
		{"at OCM ceiling", maxOCMBlob, true, true},
		{"over OCM ceiling", maxOCMBlob + 1, true, false},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			blob, claim := paddedPythonClaim(t, testcase.size, testcase.valid)
			if got := claimExtractable(claim, blob); got != testcase.want {
				t.Fatalf("claimExtractable(size=%d)=%t want=%t", len(blob), got, testcase.want)
			}
		})
	}
}

func paddedPythonClaim(t *testing.T, size int, valid bool) ([]byte, ocmClaim) {
	t.Helper()
	prefix := []byte("def test_widget():\n    \"\"\"LRF-CLI-001 widget renderer\"\"\"\n")
	suffix := []byte("\n")
	if !valid {
		suffix = []byte("\ndef broken(:\n")
	}
	if size < len(prefix)+len(suffix) {
		t.Fatalf("size=%d is too small", size)
	}
	blob := append([]byte(nil), prefix...)
	blob = append(blob, '#')
	blob = append(blob, bytes.Repeat([]byte{'x'}, size-len(prefix)-len(suffix)-1)...)
	blob = append(blob, suffix...)
	anchor := []byte("\"\"\"LRF-CLI-001 widget renderer\"\"\"")
	start := bytes.Index(blob, anchor)
	return blob, ocmClaim{
		path: "test_widget.py", selector: "test:test_widget#doc",
		span: ocmSpan{start: int64(start), end: int64(start + len(anchor))},
	}
}

func TestRequirementEnumerationIgnoresHeadingsInsideFences(t *testing.T) {
	t.Run("OCM-V0-001 a fenced ## line does not end the Requirements section", func(t *testing.T) {
		data := []byte("# Intent\n\n## Requirements\n\n- `FEN-V0-001`: first.\n\n````markdown\n```\n## Example\n```\n````\n\n- `FEN-V0-002`: second.\n\n## Non-goals\n")
		_, requirements, scope, err := requirementsFromBlob("docs/specs/fence.md", "0123456789abcdef", data)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"FEN-V0-001", "FEN-V0-002"}; !reflect.DeepEqual(requirements, want) {
			t.Fatalf("requirements=%v want=%v", requirements, want)
		}
		if bytes.Contains(scope, []byte("## Non-goals")) {
			t.Fatalf("scope %q runs past the unfenced heading", scope)
		}
	})
	t.Run("OCM-V0-001 a fenced ## Requirements heading does not count toward exactly one", func(t *testing.T) {
		data := []byte("# Intent\n\n~~~\n## Requirements\n~~~\n\n## Requirements\n\n- `FEN-V0-003`: third.\n")
		_, requirements, _, err := requirementsFromBlob("docs/specs/fence.md", "0123456789abcdef", data)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"FEN-V0-003"}; !reflect.DeepEqual(requirements, want) {
			t.Fatalf("requirements=%v want=%v", requirements, want)
		}
	})
}

func TestRequirementEnumerationBareATXHeadingEndsSection(t *testing.T) {
	t.Run("OCM-V0-001 a bare ## line ends the Requirements section", func(t *testing.T) {
		data := []byte("# Intent\n\n## Requirements\n\n- `ATX-V0-001`: inside.\n\n##\n\n- `ATX-V0-002`: outside.\n")
		_, requirements, _, err := requirementsFromBlob("docs/specs/atx.md", "0123456789abcdef", data)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"ATX-V0-001"}; !reflect.DeepEqual(requirements, want) {
			t.Fatalf("requirements=%v want=%v", requirements, want)
		}
	})
}

// GPK-V0-037: a report output naming a `.git` segment in any letter case is
// refused before publication, so a report cannot overwrite Git metadata.
func TestPublishOCMReportRefusesGitMetadataOutput(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".git", "config")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("[core]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, requested := range []string{".git/config", ".GIT/config"} {
		_, err := PublishOCMReport(context.Background(), root, requested, []byte("# report\n"), true)
		if CodeOf(err) != "unsupported-ocm-output-path" {
			t.Fatalf("%s: err=%v, want unsupported-ocm-output-path", requested, err)
		}
	}
	if data, err := os.ReadFile(config); err != nil || string(data) != "[core]\n" {
		t.Fatalf("config=%q err=%v", data, err)
	}
}
