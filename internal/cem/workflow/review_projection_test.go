package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEM-PILOT-028..030: both renderings of the stable review projection bind the
// same retained records, a JSON review writes nothing, and absent witnesses
// cannot become passing tests.
func TestReviewProjectionParityAndReadOnly(t *testing.T) {
	t.Parallel()
	t.Run("CEM-PILOT-028 CEM-PILOT-030 CEM-PILOT-031 immutable projection parity and witness gaps", func(t *testing.T) {
		root, base, target := makeRepo(t)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		citeAll(t, root)
		before, _ := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
		options := ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Format: "json",
			ArtifactRoot: root, ArtifactsGiven: true}
		result, err := openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		if result["mutates"] != false || result["ok"] != true {
			t.Fatalf("JSON verdict: %v", result)
		}
		projection := result["review"].(map[string]any)
		digest := projection["recordSetSha256"]
		delete(projection, "recordSetSha256")
		if reviewDigest(projection) != digest {
			t.Fatal("record set digest does not bind projected records")
		}
		projection["recordSetSha256"] = digest
		if got := projection["ocm"].(map[string]any)["reason"]; got != "ocm-not-supplied" {
			t.Fatalf("missing OCM reason: %v", got)
		}
		rows := projection["hunks"].([]any)
		if len(rows) != 2 {
			t.Fatalf("hunk denominator: %d", len(rows))
		}
		for _, item := range rows {
			row := item.(map[string]any)
			if row["testExecution"].(map[string]any)["state"] != "NOT_RUN" {
				t.Fatal("unrun test upgraded")
			}
			if row["coverageState"] != "NOT_PRODUCED" {
				t.Fatal("missing witness omitted")
			}
		}
		after, _ := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
		if string(before) != string(after) {
			t.Fatal("JSON review changed map bytes")
		}
		gitDir := gitCmd(t, root, "rev-parse", "--git-dir")
		if _, err := os.Stat(filepath.Join(root, gitDir, defaultReportRelative)); !os.IsNotExist(err) {
			t.Fatalf("JSON wrote report: %v", err)
		}
		options.Format = "markdown"
		human, err := openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		if human["recordSetSha256"] != digest {
			t.Fatal("human and JSON record digests differ")
		}
		text, err := os.ReadFile(human["report"].(string))
		if err != nil || !strings.Contains(string(text), digest.(string)) {
			t.Fatalf("human digest absent: %v", err)
		}
		options.Format, options.Output = "json", "forbidden.json"
		if _, err := openSession(t, root).Read(ctx(), "report", options); err == nil {
			t.Fatal("JSON output option accepted")
		}
	})
}
