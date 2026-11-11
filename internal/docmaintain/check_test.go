package docmaintain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// DMF-V1-003: Check reads the committed page and fails closed: a fresh block is fresh, a missing
// selected block is missing, a page absent at HEAD and a malformed page are never fresh, and an
// uncommitted worktree edit does not change the answer.
func TestDMFV1003CheckStates(t *testing.T) {
	root := fixtureRepo(t, digestV1)
	ctx := context.Background()
	selector := Selector{Source: "owner.md", Package: "widget"}
	writePage(t, root, "# Page\n\n<!-- corvint:docmaintain:insertion-point -->\n")
	if _, err := Run(ctx, root, "docs/page.md", []Selector{selector}, Policy{Enabled: true, Apply: true}); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "page")
	report, err := Check(ctx, root, []CheckPage{{Page: "docs/page.md"}})
	if err != nil || report.Stale != 0 || len(report.Blocks) != 1 || report.Blocks[0].State != BlockFresh || report.Blocks[0].Line != 4 || len(report.Commit) < 40 {
		t.Fatalf("fresh %+v %v", report, err)
	}
	writePage(t, root, "# Edited in the worktree only\n")
	if report, err = Check(ctx, root, []CheckPage{{Page: "docs/page.md"}}); err != nil || report.Stale != 0 {
		t.Fatalf("a worktree edit changed the committed answer %+v %v", report, err)
	}
	other := Selector{Source: "owner.md", Package: "absent"}
	report, err = Check(ctx, root, []CheckPage{{Page: "docs/page.md", Selectors: []Selector{other}}, {Page: "docs/none.md"}})
	if err != nil || report.Stale != 2 || report.Blocks[0].State != BlockUnchecked || report.Blocks[1].State != BlockUnchecked || report.Blocks[1].Reason != "page-absent-at-head" {
		t.Fatalf("unchecked %+v %v", report, err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "bad.md"), []byte("<!-- corvint:docmaintain begin source=\"x\" -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plain.md"), []byte("# Plain\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "bad page")
	report, err = Check(ctx, root, []CheckPage{{Page: "docs/bad.md"}, {Page: "docs/plain.md", Selectors: []Selector{selector}}})
	if err != nil || report.Stale != 2 || report.Blocks[0].Reason != "malformed-block" || report.Blocks[1].State != BlockMissing {
		t.Fatalf("malformed or missing %+v %v", report, err)
	}
	if _, err = Check(ctx, root, []CheckPage{{Page: "../outside.md"}}); err == nil {
		t.Fatal("a page outside the root was checked")
	}
}
