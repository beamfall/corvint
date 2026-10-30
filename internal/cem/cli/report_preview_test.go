package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-345 (V1-2811): `cem report --preview` renders the same Markdown the
// published report would write into the envelope, reports mutates=false and
// writes no file; it refuses --output and --format json, which would make the
// request ambiguous. The default publication of CEM-V1-035 is unchanged.
func TestDCWV0345ReportPreviewWritesNothing(t *testing.T) {
	f := materializeStableRead(t)
	reportPath := filepath.Join(f.repo, ".git", "corvint", "cem-review.md")
	arguments := coreStableArguments("report", f, f.artifacts)
	_, preview, refusal := runCoreRead(t, f.repo, append(arguments, "--preview")...)
	if refusal != nil {
		t.Fatalf("preview refused: %v", refusal)
	}
	markdown, _ := preview["markdown"].(string)
	if preview["mutates"] != false || !strings.Contains(markdown, "- Spec: `cem/1.0`\n") || preview["report"] != nil {
		t.Fatalf("preview envelope mutates=%v report=%v markdown=%q", preview["mutates"], preview["report"], markdown)
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("preview wrote %s: %v", reportPath, err)
	}
	// The preview bytes equal what the default report then publishes.
	_, published, _ := runCoreRead(t, f.repo, arguments...)
	written, err := os.ReadFile(reportPath)
	if err != nil || string(written) != markdown || published["recordSetSha256"] != preview["recordSetSha256"] {
		t.Fatalf("published report differs from the preview (%v)", err)
	}
	for _, extra := range [][]string{{"--output", "elsewhere.md"}, {"--format", "json"}} {
		exit, envelope, refusal := runCoreRead(t, f.repo, append(append(arguments, "--preview"), extra...)...)
		message, _ := refusal["error"].(string)
		if exit != 2 || envelope != nil || refusal["code"] != "invalid-arguments" || !strings.Contains(message, "--preview writes nothing") {
			t.Errorf("--preview %v: exit %d envelope %v refusal %v", extra, exit, envelope, refusal)
		}
	}
}
