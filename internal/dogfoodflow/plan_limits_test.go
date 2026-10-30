package dogfoodflow

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/observations"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-035: both row bounds are inclusive and overflow has its own stable code.
func TestPlanRowLimits(t *testing.T) {
	for _, kind := range []string{"citations", "links"} {
		for _, rows := range []int{256, 257} {
			t.Run(kind+strings.Repeat("+", rows-256), func(t *testing.T) {
				root := t.TempDir()
				if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(root, "plan.tsv")
				line := "1\tintent.md\t1:1\tspecification\n"
				if kind == "links" {
					line = "intent.md\tFIXTURE-001\t1\tcase_test.go\ttest:TestCase\n"
				}
				if err := os.WriteFile(source, []byte(strings.Repeat(line, rows)), 0600); err != nil {
					t.Fatal(err)
				}
				var stderr bytes.Buffer
				c := &change{flow: flow{ctx: context.Background(), root: root, stderr: &stderr}, evidence: root, cemPrepared: true, intentSnapshot: []byte("intent.md\n")}
				code := "citation-plan-row-limit-exceeded"
				if kind == "citations" {
					c.options.Citations = source
					if rows == 256 {
						if _, valid := c.validateCitationPlan(); !valid {
							t.Fatal("boundary refused")
						}
						return
					}
					c.citeStep()
				} else {
					code = "ocm-link-plan-row-limit-exceeded"
					c.options.OCMLinks = source
					c.loadOCMLinkPlan()
					if rows == 256 {
						if !c.linksReady || c.linkPlanRows != 256 {
							t.Fatalf("boundary refused: %+v", c.rows)
						}
						return
					}
				}
				if len(c.rows) != 1 || c.rows[0].reason != code {
					t.Fatalf("steps=%+v", c.rows)
				}
				if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/\n"), 0600); err != nil {
					t.Fatal(err)
				}
				row := c.rows[0]
				if err := observations.Append(root, observations.Event{Kind: "dogfood-step", Step: row.name, Status: row.status, Reason: row.reason}); err != nil {
					t.Fatalf("emitted row lost at observation boundary: %v", err)
				}
				data, err := os.ReadFile(filepath.Join(root, ".corvint/self-observations.jsonl"))
				if err != nil || !bytes.Contains(data, []byte(code)) {
					t.Fatalf("observation=%s err=%v", data, err)
				}
				hint := c.fixHint(c.rows[0])
				if !strings.Contains(hint, "limit=256 observed=257") {
					t.Fatalf("fix=%s", hint)
				}
			})
		}
	}
}
