package verify

// Corpus harness: constructs the frozen interop and conformance repositories
// per their manifests and runs every valid, invalid, drift, and mutation
// vector through the full verification flow.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/execfile"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gitAt(t *testing.T, dir, author, email, date string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+os.TempDir(), "XDG_CONFIG_HOME="+os.TempDir(),
		"GIT_AUTHOR_NAME="+author, "GIT_AUTHOR_EMAIL="+email, "GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_NAME="+author, "GIT_COMMITTER_EMAIL="+email, "GIT_COMMITTER_DATE="+date)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- inherited drift-rule mutation cases ---

// TestClassifySpanCountsAMatchEndingAtTheEnd checks the ALGORITHMS.md drift
// rule: overlapping matches count, including one that ends at the last target
// byte, so two occurrences are ambiguous rather than relocated.
func TestClassifySpanCountsAMatchEndingAtTheEnd(t *testing.T) {
	t.Parallel()
	status, span := ClassifySpan(true, false, []byte("xx"), []byte("x"), wire.Span{Start: 0, End: 1})
	if status != DriftAmbiguous || span != nil {
		t.Fatalf("ClassifySpan = %v, %v; want %v with no span", status, span, DriftAmbiguous)
	}
}

// TestDriftSymlinkSameOIDIsStable is CEM-PILOT-019 (decision 0098): when the
// target path becomes a symlink whose target string equals the base blob
// bytes, the blob OID is unchanged and driftItem reports stable, because the
// frozen same-path rule tests OID identity before any entry kind.
func TestDriftSymlinkSameOIDIsStable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	content := "target.txt"
	if err := os.WriteFile(filepath.Join(root, "f"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "base")
	blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:f")
	if err := os.Remove(filepath.Join(root, "f")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(content, filepath.Join(root, "f")); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", "-A")
	gitAt(t, root, author, email, date, "commit", "-qm", "symlink")
	target := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	item, err := driftItem(context.Background(), repository,
		wire.Evidence{ID: "e1", BlobOid: blob, Path: "f", Span: wire.Span{Start: 0, End: 6}}, nil, target)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != DriftStable {
		t.Errorf("status = %q, want %q (blob OID unchanged is stable per ALGORITHMS.md:154-155)", item.Status, DriftStable)
	}
	if item.TargetBlobOid != blob {
		t.Errorf("target blob oid = %q, want %q", item.TargetBlobOid, blob)
	}
}

// TestDriftNonRegularTargetEntryIsDeleted is CEM-PILOT-019 (decision 0098): a
// same-path target entry with a different OID that is not a regular-file blob
// (symlink, directory, gitlink) has no file content to search, so it reports
// the frozen `deleted` status with no target blob OID or span. The symlink's
// link text contains the cited bytes once, so a blob search would have said
// relocated.
func TestDriftNonRegularTargetEntryIsDeleted(t *testing.T) {
	t.Parallel()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	replacements := map[string]func(t *testing.T, root, path string){
		"symlink": func(t *testing.T, root, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../evidence", path); err != nil {
				t.Fatal(err)
			}
			gitAt(t, root, author, email, date, "add", "-A")
		},
		"directory": func(t *testing.T, root, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "x"), []byte("evidence"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitAt(t, root, author, email, date, "add", "-A")
		},
		"gitlink": func(t *testing.T, root, path string) {
			commit := gitAt(t, root, author, email, date, "rev-parse", "HEAD")
			gitAt(t, root, author, email, date, "rm", "-q", "--cached", "f")
			gitAt(t, root, author, email, date, "update-index", "--add", "--cacheinfo", "160000,"+commit+",f")
		},
	}
	for kind, replace := range replacements {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "f")
			gitAt(t, root, author, email, date, "init", "-q")
			if err := os.WriteFile(path, []byte("evidence bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitAt(t, root, author, email, date, "add", ".")
			gitAt(t, root, author, email, date, "commit", "-qm", "base")
			blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:f")
			replace(t, root, path)
			target := gitAt(t, root, author, email, date, "write-tree")

			repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
			if err != nil {
				t.Fatal(err)
			}
			item, err := driftItem(context.Background(), repository,
				wire.Evidence{ID: "e1", BlobOid: blob, Path: "f", Span: wire.Span{Start: 0, End: 8}}, nil, target)
			if err != nil {
				t.Fatal(err)
			}
			if item.Status != DriftDeleted || item.TargetBlobOid != "" || item.TargetSpan != nil {
				t.Errorf("item = %+v, want deleted with no target blob OID or span", item)
			}
		})
	}
}

// TestCheckTargetSidecarRejectsDrifted checks CEM-CB-009: a present
// target-side .corvint/change.cem.json blob whose bytes differ from the
// verified CEM input is refused, not silently accepted.
func TestCheckTargetSidecarRejectsDrifted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	stored := []byte(`{"stored":true}`)
	if err := os.WriteFile(filepath.Join(root, ".corvint", "change.cem.json"), stored, 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "sidecar")
	target := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	err = checkTargetSidecar(context.Background(), repository, target, []byte(`{"verified":true}`))
	if cemcode.CodeOf(err) != cemcode.ExcludedArtifactMismatch {
		t.Fatalf("checkTargetSidecar = %v, want %s", err, cemcode.ExcludedArtifactMismatch)
	}
}

// TestInheritedRejectsBaseRevisionThatIsNotACommitOID checks that Resolve's
// `^{commit}` peel must equal document.BaseRevision exactly: an annotated tag
// OID used as baseRevision must be refused, not silently peeled and accepted.
func TestInheritedRejectsBaseRevisionThatIsNotACommitOID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "base")
	gitAt(t, root, author, email, date, "tag", "-a", "-m", "annotated", "v1")
	tagOID := gitAt(t, root, author, email, date, "rev-parse", "v1")

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	document := &wire.Map{BaseRevision: tagOID}
	if _, err := inherited(context.Background(), repository, document, nil, ""); cemcode.CodeOf(err) != cemcode.BaseRevisionMismatch {
		t.Fatalf("inherited = %v, want %s", err, cemcode.BaseRevisionMismatch)
	}
}

// TestCheckEvidenceAcceptsExecutableBlob checks that an evidence path
// resolving to a regular 100755 (executable) blob is accepted per
// ALGORITHMS.md, not refused as though only 100644 were a regular file.
func TestCheckEvidenceAcceptsExecutableBlob(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	content := []byte("#!/bin/sh\necho hi\n")
	path := filepath.Join(root, "run.sh")
	if err := execfile.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "executable")
	base := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")
	blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:run.sh")
	if mode := strings.Fields(gitAt(t, root, author, email, date, "ls-tree", "HEAD", "run.sh"))[0]; mode != "100755" {
		t.Fatalf("fixture mode = %s, want 100755 (core.fileMode must be honored)", mode)
	}

	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	span := wire.Span{Start: 0, End: int64(len(content))}
	digest := sha256.Sum256(content)
	spanSha256 := hex.EncodeToString(digest[:])
	document := &wire.Map{Evidence: []wire.Evidence{{
		ID:         wire.EvidenceIdentity(blob, "run.sh", span, spanSha256),
		BlobOid:    blob,
		Path:       "run.sh",
		Span:       span,
		SpanSha256: spanSha256,
	}}}
	if _, err := checkEvidence(context.Background(), repository, document, base); err != nil {
		t.Fatalf("checkEvidence rejected a 100755 evidence blob: %v", err)
	}
}

// TestMechanicalProvenRefusesIdenticalRemovedAndAdded checks that a hunk
// whose removed and added bytes are byte-identical never proves a mechanical
// claim: there is nothing to prove.
func TestMechanicalProvenRefusesIdenticalRemovedAndAdded(t *testing.T) {
	t.Parallel()
	hunk := &patch.Hunk{Body: []patch.BodyLine{
		{Prefix: '-', Payload: []byte("same\n")},
		{Prefix: '+', Payload: []byte("same\n")},
	}}
	for _, reason := range []string{"whitespace-only", "line-ending-only"} {
		if mechanicalProven(hunk, reason) {
			t.Errorf("mechanicalProven(%q) = true for identical removed/added bytes", reason)
		}
	}
}
