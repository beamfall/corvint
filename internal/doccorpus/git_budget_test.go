package doccorpus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// phaseGitOps is the Git operation bound every corpus phase must meet
// regardless of its input count (DCP-V1-155, issue #1240: "<= 16").
const phaseGitOps = 16

// boundCorpusGitOps gives every corpus phase opened during the test a budget
// of ops Git operations; a phase that needs more fails with the budget error.
func boundCorpusGitOps(t *testing.T, ops int) {
	t.Helper()
	previous := newGitBudget
	t.Cleanup(func() { newGitBudget = previous })
	newGitBudget = func() *gitrun.Budget { return gitrun.NewBudget(ops, gitrun.DefaultTotalBudget) }
}

// writeBulkFixture commits files under root in one commit with fixed dates.
func writeBulkFixture(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	t.Setenv("GIT_AUTHOR_DATE", "2026-10-19T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-10-19T00:00:00Z")
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	for p, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	return git(t, root, "rev-parse", "HEAD")
}

// nonIndexedFile is an html, erb or json input the source index does not
// load as text, so it is pinned through Git (DCP-V1-156).
func nonIndexedFile(i int) (string, string) {
	dir := fmt.Sprintf("docs/web%d/", i/100)
	switch i % 3 {
	case 0:
		return dir + fmt.Sprintf("p%d.html", i), fmt.Sprintf("<p>page %d</p>\n", i)
	case 1:
		return dir + fmt.Sprintf("p%d.erb", i), fmt.Sprintf("<%%= item %d %%>\n", i)
	default:
		return dir + fmt.Sprintf("p%d.json", i), fmt.Sprintf("{\"page\": %d}\n", i)
	}
}

// pinAll declares every tracked file under scope at rev in a /2 manifest.
func pinAll(t *testing.T, root, rev, scope string) Manifest {
	t.Helper()
	ctx := context.Background()
	id, err := contextindex.CorpusRepositoryID(ctx, root, rev)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{Schema: ManifestSchemaV2, Repository: Repository{id, rev}, BuiltAt: "2026-10-19T00:00:00Z", Profile: Profile{ID: "native", Revision: "1", Title: "Repository evidence", Format: "markdown", Groups: []string{}}, Scopes: []Scope{{scope, rev}}, Inputs: []Input{}, Providers: []Provider{{ID: "native", Kind: "native", Version: "1", Revision: rev}}, MergeRule: "none"}
	for _, line := range strings.Split(git(t, root, "ls-tree", "-r", rev, "--", scope), "\n") {
		meta, p, _ := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		data, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		m.Inputs = append(m.Inputs, Input{Path: p, Revision: rev, Blob: fields[2], SHA256: Digest(data), Provider: "native", Purpose: "source"})
	}
	return m
}

func TestDCPV1155ReadFreshnessGitOpsIndependentOfInputCount(t *testing.T) {
	t.Run("DCP-V1-155 DCP-V1-156 10,000 pinned inputs build, read and go stale on one change within a constant Git budget", func(t *testing.T) {
		root := t.TempDir()
		files := map[string]string{}
		for i := range 8000 {
			files[fmt.Sprintf("docs/d%d/t%d.md", i/100, i)] = fmt.Sprintf("# Topic %d\n\nThe widget %d explains the alpha subsystem.\n", i, i)
		}
		for i := range 2000 {
			p, data := nonIndexedFile(i)
			files[p] = data
		}
		rev := writeBulkFixture(t, root, files)
		m := pinAll(t, root, rev, "docs")
		if len(m.Inputs) != 10000 {
			t.Fatalf("fixture pins %d inputs", len(m.Inputs))
		}
		boundCorpusGitOps(t, phaseGitOps)
		ctx := context.Background()
		a, err := Build(ctx, root, m)
		if err != nil {
			t.Fatalf("build of 10,000 inputs within %d Git operations: %v", phaseGitOps, err)
		}
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := ReadQuery(ctx, root, raw, Request{Operation: "search", Query: "widget", Limit: 5})
		if err != nil {
			t.Fatalf("read of a 10,000-input corpus within %d Git operations per phase: %v", phaseGitOps, err)
		}
		if receipt.Envelope == nil || receipt.Envelope.Freshness != "fresh" || len(receipt.Results) == 0 {
			t.Fatalf("read receipt: %+v", receipt.Envelope)
		}
		changed, _ := nonIndexedFile(1234)
		if err := os.WriteFile(filepath.Join(root, changed), []byte("<p>changed</p>\n"), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "commit", "-qam", "change one input")
		overlay, err := LiveFreshness(ctx, root, a)
		if err != nil {
			t.Fatal(err)
		}
		if overlay.State != "stale" || !strings.Contains(strings.Join(overlay.Limitations, "\n"), "input changed or absent: "+changed) {
			t.Fatalf("one changed input of 10,000: %s %v", overlay.State, overlay.Limitations)
		}
		for _, limit := range overlay.Limitations {
			if strings.HasPrefix(limit, "input changed or absent: ") && limit != "input changed or absent: "+changed {
				t.Fatalf("unchanged input reported stale: %s", limit)
			}
		}
	})
}

func TestDCPV1156ManifestAndBuildBatchNonIndexedInputs(t *testing.T) {
	t.Run("DCP-V1-156 2,000 html/erb/json inputs inventory and build within a constant Git budget", func(t *testing.T) {
		root := t.TempDir()
		files := map[string]string{}
		for i := range 2000 {
			p, data := nonIndexedFile(i)
			files[p] = data
		}
		rev := writeBulkFixture(t, root, files)
		boundCorpusGitOps(t, phaseGitOps)
		ctx := context.Background()
		m, err := Inventory(ctx, root, rev, "docs", "2026-10-19T00:00:00Z")
		if err != nil {
			t.Fatalf("manifest of 2,000 non-indexed inputs within %d Git operations: %v", phaseGitOps, err)
		}
		if len(m.Inputs) != 2000 {
			t.Fatalf("manifest pins %d inputs", len(m.Inputs))
		}
		for _, in := range m.Inputs {
			if in.SHA256 != Digest([]byte(files[in.Path])) {
				t.Fatalf("%s pinned the wrong content", in.Path)
			}
		}
		if _, err := Build(ctx, root, m); err != nil {
			t.Fatalf("build of 2,000 non-indexed inputs within %d Git operations: %v", phaseGitOps, err)
		}
	})
}

func TestDCPV1157CapacityRefusalsAreTypedAndEarly(t *testing.T) {
	t.Run("DCP-V1-157 an over-limit input count is refused before any Git work, naming limit and count", func(t *testing.T) {
		root, m := fixture(t)
		m.Schema = ManifestSchemaV2
		sample := m.Inputs[0]
		m.Inputs = make([]Input, MaxCorpusRecords+1)
		for i := range m.Inputs {
			m.Inputs[i] = sample
			m.Inputs[i].Path = fmt.Sprintf("src/generated/f%06d.md", i)
		}
		boundCorpusGitOps(t, 0) // any Git operation would fail with the budget error instead
		_, err := Build(context.Background(), root, m)
		var refused *Error
		if !errors.As(err, &refused) || refused.Code != CapacityCode || !strings.Contains(refused.Message, fmt.Sprint(MaxCorpusRecords+1)) || !strings.Contains(refused.Message, fmt.Sprint(MaxCorpusRecords)) {
			t.Fatalf("over-limit manifest: %v", err)
		}
	})
	t.Run("DCP-V1-158 an exhausted Git budget is typed corpus-capacity-exceeded, never internal-error", func(t *testing.T) {
		root := t.TempDir()
		p, data := nonIndexedFile(0)
		rev := writeBulkFixture(t, root, map[string]string{p: data, "docs/readme.md": "# Readme\n"})
		m := pinAll(t, root, rev, "docs")
		boundCorpusGitOps(t, 0) // the Git-pinned html input needs at least one operation
		_, err := Build(context.Background(), root, m)
		var refused *Error
		if !errors.As(err, &refused) || refused.Code != CapacityCode || !strings.Contains(refused.Message, "Git operation budget") || !strings.Contains(refused.Message, fmt.Sprintf("pinning %d inputs", len(m.Inputs))) {
			t.Fatalf("exhausted budget: %v", err)
		}
		raw := cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted")
		if !errors.As(Typed(raw), &refused) || refused.Code != CapacityCode {
			t.Fatalf("Typed(%v) = %v", raw, Typed(raw))
		}
		other := cemcode.New(cemcode.RepositoryObjectUnavailable, "missing")
		if Typed(other) != other || Typed(nil) != nil {
			t.Fatal("Typed changed a non-capacity error")
		}
	})
}

func TestDCPV1159BatchedPinningKeepsArtifactBytes(t *testing.T) {
	t.Run("DCP-V1-159 a mixed indexed and Git-pinned corpus encodes to the pre-batching bytes", func(t *testing.T) {
		root := t.TempDir()
		files := map[string]string{
			"go.mod":                  "module example.invalid/corpus\n\ngo 1.27.1\n",
			"src/readme.md":           "# Count\n\nCounts a value.\n",
			"src/value.go":            "package value\n\n// Count returns one.\nfunc Count() int { return 1 }\n",
			"src/view/page.html":      "<h1>Count</h1>\n<p>Shows the count.</p>\n",
			"src/view/item.erb":       "<%= count %>\n",
			"src/view/deep/data.json": "{\"count\": 1}\n",
			"src/view/deep/notes.md":  "# Notes\n\nThe count is one.\n",
		}
		rev := writeBulkFixture(t, root, files)
		ctx := context.Background()
		m, err := Inventory(ctx, root, rev, "src", "2026-10-19T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		a, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		a.Builder, a.SHA256 = Builder{}, ""
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		// Captured from the per-input loader at 99dac523459bdf131c0ca688acc16edbcbacda2c.
		const want = "ee42ecc3e972a67d43ae68fded33495caafd843076ffabf97ccd24fd74d3f1e5"
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Fatalf("artifact bytes changed: sha256 %s, want %s", got, want)
		}
	})
}
