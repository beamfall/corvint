package postmergeworkflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
)

// PMR-V0-940..942: the harness authors private commits from adapter content and
// records draft requests for them; nothing outward is performed and the product
// root gains no ref, object or worktree change.
func TestReplayAuthorAndDraftStages(t *testing.T) {
	root, ff, pf, f, p := setup(t, "authored")
	f.Expected.TestGaps = []string{"gap-1"}
	writeJSON(t, ff, f)
	p.DraftURL = "https://example.com/draft"
	writeJSON(t, pf, p)
	status, refs := git(t, root, "status", "--porcelain"), git(t, root, "for-each-ref")
	r, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil {
		t.Fatalf("replay: %v %+v", err, r)
	}
	if r.Status != "MATCH" || r.WorkflowQualification != "NOT_OBSERVED" {
		t.Fatalf("report: %+v", r)
	}
	names := []string{}
	for _, s := range r.ReplayedStages {
		if s.Status != "observed" || !hash.MatchString(s.ArtifactSHA256) {
			t.Fatalf("replayed stage %+v", s)
		}
		names = append(names, s.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"docs-author", "draft-requests", "tests-author"}) {
		t.Fatalf("replayed stages %v", names)
	}
	if !slices.Equal(r.DeferredStages, []string{"docs-scope", "docs-validation", "tests-scope", "tests-validation"}) {
		t.Fatalf("deferred stages %v", r.DeferredStages)
	}
	for _, limit := range []string{"authored-content-scope-validation-not-verified", "draft-requests-recorded-not-performed"} {
		if !slices.Contains(r.Limits, limit) {
			t.Fatalf("limit %s missing: %v", limit, r.Limits)
		}
	}
	revisions := regexp.MustCompile(`Revision: ([0-9a-f]{40})`).FindAllStringSubmatch(r.Recording, -1)
	if strings.Count(r.Recording, `"upsert-draft-change"`) != 2 || len(revisions) != 2 || !strings.Contains(r.Recording, "Drafts: 2") {
		t.Fatalf("recording lacks the two draft requests: %s", r.Recording)
	}
	merge := f.Connector.Forge.Change.Binding.Merge
	for _, m := range revisions {
		if m[1] == merge {
			t.Fatal("draft names the merge, not an authored commit")
		}
		cmd := exec.Command("git", "-C", root, "cat-file", "-e", m[1]+"^{commit}")
		if cmd.Run() == nil {
			t.Fatalf("authored commit %s leaked into the product object store", m[1])
		}
	}
	if git(t, root, "status", "--porcelain") != status || git(t, root, "for-each-ref") != refs {
		t.Fatal("product worktree or refs changed")
	}
	again, err := Replay(context.Background(), root, ff, pf, "change-1")
	if err != nil || again.RecordingSHA256 != r.RecordingSHA256 {
		t.Fatalf("a second replay authored different revisions: %v", err)
	}
}

func TestReplayAuthoredBlocks(t *testing.T) {
	for _, tc := range []struct{ mode, url, want string }{
		{"authored", "", "draft-url-unpinned"},
		{"authored", "https://elsewhere.example/draft", "draft-invalid"},
		{"authored-docs-untargeted", "https://example.com/draft", "authored-scope-invalid"},
		{"authored-tests-nontest", "https://example.com/draft", "authored-scope-invalid"},
		{"authored-no-gaps", "https://example.com/draft", "authored-scope-invalid"},
		{"draft", "https://example.com/draft", "draft-scope-validation-unavailable"},
	} {
		t.Run(tc.mode+"/"+tc.want, func(t *testing.T) {
			root, ff, pf, _, p := setup(t, tc.mode)
			p.DraftURL = tc.url
			writeJSON(t, pf, p)
			r, err := Replay(context.Background(), root, ff, pf, "change-1")
			if err == nil || r.Status != "BLOCKED" || !slices.Contains(r.Reasons, tc.want) {
				t.Fatalf("got %v %+v, want BLOCKED %s", err, r, tc.want)
			}
		})
	}
}

// PMR-V0-940: authoring updated content for an executable file keeps its
// executable mode; a new path is a regular file.
func TestAuthorCommitsPreservesExecutableMode(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execfile.WriteFile(filepath.Join(root, "scripts", "check_test.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "update-index", "--chmod=+x", "scripts/check_test.sh")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "base")
	change := connector.Change{MergedAt: "2026-10-31T00:00:00Z", Binding: connector.Binding{Change: "change-1", Merge: git(t, root, "rev-parse", "HEAD")}}
	authored := []Authored{{Kind: "tests", Files: []AuthoredFile{
		{Path: "scripts/check_test.sh", Content: "#!/bin/sh\nexit 1\n"},
		{Path: "scripts/new_test.sh", Content: "#!/bin/sh\n"},
	}}}
	repo, drafts, err := authorCommits(context.Background(), root, t.TempDir(), "https://example.com/draft", change, authored)
	if err != nil || len(drafts) != 1 {
		t.Fatalf("author: %v %v", err, drafts)
	}
	tree := git(t, repo, "ls-tree", "-r", drafts[0].Revision, "scripts")
	if !strings.Contains(tree, "100755 blob") || !strings.Contains(tree, "\tscripts/check_test.sh") ||
		!regexp.MustCompile(`100644 blob [0-9a-f]+\tscripts/new_test.sh`).MatchString(tree) ||
		regexp.MustCompile(`100644 blob [0-9a-f]+\tscripts/check_test.sh`).MatchString(tree) {
		t.Fatalf("authored modes:\n%s", tree)
	}
}
