package cem2proto

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// fixture is a hermetic repository with one change and its cem/1.0 map.
type fixture struct {
	root, base, target string
	original           []byte
	stable             *wire.StableMap
}

var fixtureBase = map[string]string{
	"docs/spec.md": "# Widget\n\nREQ-1: the widget MUST frob on start.\nREQ-2: the widget MUST NOT crash on empty input.\n",
	"main.go":      "package main\n\nfunc main() {\n\tfrob()\n}\n\nfunc frob() {}\n\n\n\n\n\n\nfunc helper() int {\n\treturn 1\n}\n",
	"notes.txt":    "alpha\nbeta\ngamma\n",
	"old.txt":      "retired\n",
}

var fixtureTarget = map[string]string{
	"docs/spec.md": fixtureBase["docs/spec.md"],
	"main.go":      "package main\n\nfunc main() {\n\tfrob()\n\tfrob()\n}\n\nfunc frob() {}\n\n\n\n\n\n\nfunc helper() int {\n\treturn 2\n}\n",
	"notes.txt":    "alpha\nbeta  \ngamma\n",
	"new.txt":      "fresh\n",
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-11-01T00:00:00Z",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-11-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if entry.Name() != ".git" {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// span locates text in a base file as a byte span.
func span(t *testing.T, path, text string) wire.Span {
	t.Helper()
	start := strings.Index(fixtureBase[path], text)
	if start < 0 {
		t.Fatalf("%q not in %s", text, path)
	}
	return wire.Span{Start: int64(start), End: int64(start + len(text))}
}

// newFixture commits the base and target trees and builds the cem/1.0 map:
// the first hunk is evidence-backed by two spec spans, whitespace hunks are
// mechanical, and the rest are no-evidence.
func newFixture(t *testing.T, mutate func(*wire.StableMap)) *fixture {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	writeTree(t, root, fixtureBase)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "base")
	base := git(t, root, "rev-parse", "HEAD")
	writeTree(t, root, fixtureTarget)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "target")
	target := git(t, root, "rev-parse", "HEAD")

	ctx := context.Background()
	src := mustSource(t, root)
	patchBytes, err := src.Patch(ctx, base, target)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := src.Blobs(ctx, base, []string{"docs/spec.md", "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	cite := func(path, text string) wire.Evidence {
		s := span(t, path, text)
		blob := blobs[path]
		digest := Sha256Hex(blob.Data[s.Start:s.End])
		return wire.Evidence{ID: wire.EvidenceIdentity(blob.OID, path, s, digest), BlobOid: blob.OID, Path: path, Span: s, SpanSha256: digest}
	}
	evidence := []wire.Evidence{
		cite("docs/spec.md", "REQ-1: the widget MUST frob on start."),
		cite("main.go", "func frob() {}"),
		cite("docs/spec.md", "REQ-2: the widget MUST NOT crash on empty input."),
	}
	m := &wire.StableMap{
		Change:   wire.StableChange{Spec: wire.StableSpec, BaseRevision: base, PatchSha256: Sha256Hex(patchBytes), ExcludedPath: wire.ExcludedCEMPath, Evidence: evidence},
		Criteria: []wire.StableCriterion{}, Receipts: []wire.StableReceipt{}, Links: []wire.StableLink{}, Artifacts: []wire.StableArtifact{},
	}
	for i, h := range parsed.Hunks {
		hunk := wire.Hunk{ID: h.ID, Path: h.DisplayPath, OldRange: h.OldRange, NewRange: h.NewRange, Disposition: "unknown", Reason: "no-evidence", Basis: []wire.Basis{}}
		switch {
		case i == 0:
			hunk.Disposition, hunk.Reason = "supported", "evidence-backed"
			hunk.Basis = []wire.Basis{{EvidenceID: evidence[0].ID, Relation: "specification"}, {EvidenceID: evidence[1].ID, Relation: "implementation"}}
		case i == 1:
			hunk.Disposition, hunk.Reason = "supported", "evidence-backed"
			hunk.Basis = []wire.Basis{{EvidenceID: evidence[2].ID, Relation: "test-claim"}}
		case h.DisplayPath == "notes.txt":
			hunk.Disposition, hunk.Reason = "mechanical", "whitespace-only"
		}
		m.Change.Hunks = append(m.Change.Hunks, hunk)
	}
	if len(m.Change.Hunks) < 4 {
		t.Fatalf("fixture patch has %d hunks; want at least 4", len(m.Change.Hunks))
	}
	if mutate != nil {
		mutate(m)
	}
	original, err := wire.EncodeStable(m)
	if err != nil {
		t.Fatalf("fixture map: %v", err)
	}
	stable, err := wire.ParseStable(original)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{root: root, base: base, target: target, original: original, stable: stable}
}

func mustSource(t *testing.T, root string) *GitSource {
	t.Helper()
	src, err := NewGitSource(root)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func (f *fixture) encode(t *testing.T, pinned bool) (header, body []byte) {
	t.Helper()
	header, body, err := Encode(f.stable, pinned)
	if err != nil {
		t.Fatal(err)
	}
	return header, body
}

func (f *fixture) project(t *testing.T, header, body []byte) ([]byte, error) {
	t.Helper()
	return Project(context.Background(), mustSource(t, f.root), f.target, header, body)
}

// reseal re-encodes a tampered body and a header that commits to it.
func reseal(t *testing.T, header []byte, b Body) ([]byte, []byte) {
	t.Helper()
	body, err := EncodeBody(b)
	if err != nil {
		t.Fatal(err)
	}
	h, err := DecodeHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	h.BodySha256 = Sha256Hex(body)
	header, err = EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	return header, body
}

func decoded(t *testing.T, body []byte) Body {
	t.Helper()
	b, err := DecodeBody(body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error = %v; want %s", err, code)
	}
}
