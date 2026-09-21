package main

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
)

const publicationTestTag = "v0.5.0a1"

type publicationFixture struct {
	root    string
	release string
	tree    string
	witness ArchiveWitness
	path    string
}

func newPublicationFixture(t *testing.T) publicationFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repository")
	runGitTest(t, ".", "init", "-q", root)
	mustWrite(t, filepath.Join(root, "tracked.txt"), "candidate\n")
	commitAllPublicationTest(t, root, "candidate")
	runGitTest(t, root, "tag", publicationTestTag)
	release := gitOutputPublicationTest(t, root, "rev-parse", "HEAD")
	tree := gitOutputPublicationTest(t, root, "rev-parse", "HEAD^{tree}")
	witness := passWitnessPublicationTest(release, tree)
	path := filepath.Join(t.TempDir(), "release-go-archive-report.json")
	writeWitnessPublicationTest(t, path, witness)
	return publicationFixture{root: root, release: release, tree: tree, witness: witness, path: path}
}

func passWitnessPublicationTest(revision, tree string) ArchiveWitness {
	witness := ArchiveWitness{Schema: archiveWitnessSchema, Revision: revision, Tree: tree, Verdict: statusPass, Archives: []ArchiveWitnessDigest{}, Reasons: []string{}}
	names := []string{"corvint_darwin_amd64.tar.gz", "corvint_darwin_arm64.tar.gz", "corvint_linux_amd64.tar.gz", "corvint_linux_arm64.tar.gz", "corvint_windows_amd64.zip"}
	for index, name := range names {
		witness.Archives = append(witness.Archives, ArchiveWitnessDigest{Name: name, SHA256: strings.Repeat(string(rune('a'+index)), 64), Bytes: 1})
	}
	return witness
}

func writeWitnessPublicationTest(t *testing.T, path string, witness ArchiveWitness) {
	t.Helper()
	content, err := canonicalJSON(witness)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

// validReceiptPublicationTest attaches the four non-Windows archives and the gate SHA256SUMS, whose
// digest the test derives independently from the witness rows.
func validReceiptPublicationTest(fixture publicationFixture) PublicationReceipt {
	var sums strings.Builder
	for _, archive := range fixture.witness.Archives {
		sums.WriteString(archive.SHA256 + "  " + archive.Name + "\n")
	}
	digest := sha256.Sum256([]byte(sums.String()))
	files := []PublicationReceiptFile{{Name: publicationChecksumsName, SHA256: hex.EncodeToString(digest[:])}}
	for _, archive := range fixture.witness.Archives[:4] {
		files = append(files, PublicationReceiptFile{Name: archive.Name, SHA256: archive.SHA256})
	}
	return PublicationReceipt{
		Schema: publicationReceiptSchema, Approval: "repository-owner", Decision: "docs/decisions/0999-alpha-publication-receipt-2026-09-12.md",
		Destination: publicationDestinationPrefix + publicationTestTag, Tag: publicationTestTag, Revision: fixture.release, Tree: fixture.tree,
		PublisherIdentity: "NOT_VERIFIED", Files: files,
	}
}

func commitReceiptPublicationTest(t *testing.T, root string, receipt PublicationReceipt) string {
	t.Helper()
	content, err := canonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(root, "docs", "releases", publicationTestTag, "publication-receipt.json")
	for _, directory := range []string{filepath.Dir(receiptPath), filepath.Join(root, "docs", "decisions")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(root, "docs", "decisions", "0999-alpha-publication-receipt-2026-09-12.md"), "# receipt\n")
	mustWrite(t, receiptPath, string(content))
	commitAllPublicationTest(t, root, "receipt")
	return gitOutputPublicationTest(t, root, "rev-parse", "HEAD")
}

func commitAllPublicationTest(t *testing.T, root, message string) {
	t.Helper()
	runGitTest(t, root, "add", "-A")
	runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-q", "--allow-empty", "-m", message)
}

func gitOutputPublicationTest(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func requirePublicationStatus(t *testing.T, fixture publicationFixture, revision, want string) {
	t.Helper()
	got, err := publicationStatus(context.Background(), fixture.root, revision, publicationTestTag, fixture.path)
	if got != want {
		t.Fatalf("publication status = %s (%v), want %s", got, err, want)
	}
}

// ARTIFACT-RDY-V0-006: only a committed, closed, canonical owner receipt is read; anything else is INVALID.
func TestPublicationReceiptStrictFormRefusesMalformedReceipts(t *testing.T) {
	fixture := newPublicationFixture(t)
	valid := validReceiptPublicationTest(fixture)
	canonical, err := canonicalJSON(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePublicationReceipt(canonical, publicationTestTag); err != nil {
		t.Fatalf("valid receipt rejected: %v", err)
	}
	for name, mutate := range map[string]func(*PublicationReceipt){
		"approval":    func(r *PublicationReceipt) { r.Approval = "agent" },
		"identity":    func(r *PublicationReceipt) { r.PublisherIdentity = "VERIFIED" },
		"destination": func(r *PublicationReceipt) { r.Destination = "https://example.invalid/" + publicationTestTag },
		"tag":         func(r *PublicationReceipt) { r.Tag = "v0.4.0a5" },
		"decision":    func(r *PublicationReceipt) { r.Decision = "docs/decisions/../receipt.md" },
		"unsorted":    func(r *PublicationReceipt) { r.Files[0], r.Files[1] = r.Files[1], r.Files[0] },
		"no-sums":     func(r *PublicationReceipt) { r.Files = r.Files[1:] },
		"no-archive":  func(r *PublicationReceipt) { r.Files = r.Files[:1] },
	} {
		receipt := validReceiptPublicationTest(fixture)
		mutate(&receipt)
		content, _ := canonicalJSON(receipt)
		if _, err := decodePublicationReceipt(content, publicationTestTag); err == nil {
			t.Errorf("%s mutation accepted", name)
		}
	}
	for name, raw := range map[string]string{
		"unknown":       strings.Replace(string(canonical), `"schema":`, `"extra":1,"schema":`, 1),
		"duplicate":     strings.Replace(string(canonical), `"schema":`, `"schema":"x","schema":`, 1),
		"noncanonical":  strings.Replace(string(canonical), `,"approval"`, `, "approval"`, 1),
		"missing-final": strings.TrimSuffix(string(canonical), "\n"),
	} {
		if _, err := decodePublicationReceipt([]byte(raw), publicationTestTag); err == nil {
			t.Errorf("%s receipt accepted", name)
		}
	}
	head := commitReceiptPublicationTest(t, fixture.root, valid)
	requirePublicationStatus(t, fixture, fixture.release, publicationAbsent)
	requirePublicationStatus(t, fixture, head, publicationPass)
	missingDecision := validReceiptPublicationTest(fixture)
	missingDecision.Decision = "docs/decisions/0998-missing-2026-09-12.md"
	requirePublicationStatus(t, fixture, commitReceiptPublicationTest(t, fixture.root, missingDecision), publicationInvalid)
	commitReceiptPublicationTest(t, fixture.root, valid)
	runGitTest(t, fixture.root, "update-index", "--chmod=+x", "docs/releases/"+publicationTestTag+"/publication-receipt.json")
	runGitTest(t, fixture.root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-q", "-m", "executable")
	requirePublicationStatus(t, fixture, gitOutputPublicationTest(t, fixture.root, "rev-parse", "HEAD"), publicationInvalid)
}

// ARTIFACT-RDY-V0-007 (decision 0141): the receipt binds to its recorded tag revision, not HEAD.
func TestPublicationReceiptBindsToTheRecordedTagRevisionNotHead(t *testing.T) {
	fixture := newPublicationFixture(t)
	head := commitReceiptPublicationTest(t, fixture.root, validReceiptPublicationTest(fixture))
	requirePublicationStatus(t, fixture, head, publicationPass)

	runGitTest(t, fixture.root, "tag", "-f", publicationTestTag, head)
	requirePublicationStatus(t, fixture, head, publicationStale)
	runGitTest(t, fixture.root, "tag", "-d", publicationTestTag)
	requirePublicationStatus(t, fixture, head, publicationStale)
	runGitTest(t, fixture.root, "tag", publicationTestTag, fixture.release)
	requirePublicationStatus(t, fixture, head, publicationPass)

	wrongTree := validReceiptPublicationTest(fixture)
	wrongTree.Tree = strings.Repeat("e", 40)
	requirePublicationStatus(t, fixture, commitReceiptPublicationTest(t, fixture.root, wrongTree), publicationInvalid)

	runGitTest(t, fixture.root, "checkout", "-q", "--orphan", "unrelated")
	runGitTest(t, fixture.root, "rm", "-rfq", ".")
	unrelated := commitReceiptPublicationTest(t, fixture.root, validReceiptPublicationTest(fixture))
	requirePublicationStatus(t, fixture, unrelated, publicationStale)
}

// ARTIFACT-RDY-V0-008: every attached digest must equal the archive witness bound to the receipt revision.
func TestPublicationReceiptDigestsMustMatchTheRecordedRevisionWitness(t *testing.T) {
	fixture := newPublicationFixture(t)
	receipt := validReceiptPublicationTest(fixture)
	cases := []struct {
		name    string
		witness func(*ArchiveWitness)
		receipt func(*PublicationReceipt)
		want    string
	}{
		{"bound", nil, nil, publicationPass},
		{"other-revision", func(w *ArchiveWitness) { w.Revision = strings.Repeat("f", 40) }, nil, publicationUnwitnessed},
		{"companion", nil, func(r *PublicationReceipt) {
			r.Files = append([]PublicationReceiptFile{{Name: "corvint-companion.zip", SHA256: strings.Repeat("0", 64)}}, r.Files...)
		}, publicationUnwitnessed},
		{"archive-digest", nil, func(r *PublicationReceipt) { r.Files[1].SHA256 = strings.Repeat("9", 64) }, publicationMismatch},
		{"sums-digest", nil, func(r *PublicationReceipt) { r.Files[0].SHA256 = strings.Repeat("9", 64) }, publicationMismatch},
		{"mismatch-outranks-unwitnessed", nil, func(r *PublicationReceipt) {
			r.Files[0].SHA256 = strings.Repeat("9", 64)
			r.Files = append([]PublicationReceiptFile{{Name: "corvint-companion.zip", SHA256: strings.Repeat("0", 64)}}, r.Files...)
		}, publicationMismatch},
		{"fail-witness", func(w *ArchiveWitness) {
			w.Verdict, w.Archives, w.Reasons = statusFail, []ArchiveWitnessDigest{}, []string{"checksum-mismatch"}
		}, nil, publicationMismatch},
	}
	// Receipt files are byte-sorted, so SHA256SUMS is Files[0] and the first archive is Files[1].
	for _, testCase := range cases {
		witness := passWitnessPublicationTest(fixture.release, fixture.tree)
		changed := receipt
		changed.Files = append([]PublicationReceiptFile(nil), receipt.Files...)
		if testCase.witness != nil {
			testCase.witness(&witness)
		}
		if testCase.receipt != nil {
			testCase.receipt(&changed)
		}
		path := filepath.Join(t.TempDir(), "witness.json")
		writeWitnessPublicationTest(t, path, witness)
		if got, err := publicationWitnessStatus(path, changed); got != testCase.want {
			t.Errorf("%s: status %s (%v), want %s", testCase.name, got, err, testCase.want)
		}
	}
	if got, _ := publicationWitnessStatus(filepath.Join(t.TempDir(), "absent.json"), receipt); got != publicationUnwitnessed {
		t.Errorf("absent witness: status %s, want %s", got, publicationUnwitnessed)
	}
}

// ARTIFACT-RDY-V0-009: reading a receipt changes no worktree, index, ref, or Git-directory byte.
func TestPublicationStatusIsReadOnly(t *testing.T) {
	fixture := newPublicationFixture(t)
	head := commitReceiptPublicationTest(t, fixture.root, validReceiptPublicationTest(fixture))
	mustWrite(t, filepath.Join(fixture.root, "tracked.txt"), "dirty worktree\n")
	before := repositorySnapshotPublicationTest(t, fixture.root)
	requirePublicationStatus(t, fixture, head, publicationPass)
	requirePublicationStatus(t, fixture, fixture.release, publicationAbsent)
	if after := repositorySnapshotPublicationTest(t, fixture.root); after != before {
		t.Fatalf("publication status mutated the repository:\nbefore %s\nafter  %s", before, after)
	}
}

func repositorySnapshotPublicationTest(t *testing.T, root string) string {
	t.Helper()
	var snapshot strings.Builder
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content := []byte{}
		if info.Mode().IsRegular() {
			content, err = os.ReadFile(path)
		}
		digest := sha256.Sum256(content)
		snapshot.WriteString(path + " " + info.Mode().String() + " " + info.ModTime().String() + " " + hex.EncodeToString(digest[:]) + "\n")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.String()
}

func TestPublicationAncestryIgnoresGrafts(t *testing.T) {
	for _, location := range []string{"repository", "ambient"} {
		t.Run("ARTIFACT-RDY-V0-007 PUB-V0-019 "+location, func(t *testing.T) {
			fixture := newPublicationFixture(t)
			head := commitReceiptPublicationTest(t, fixture.root, validReceiptPublicationTest(fixture))
			tree := gitOutputPublicationTest(t, fixture.root, "rev-parse", "HEAD^{tree}")
			unrelated := gitOutputPublicationTest(t, fixture.root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit-tree", tree, "-m", "unrelated")
			graftPath := filepath.Join(fixture.root, ".git", "info", "grafts")
			if location == "ambient" {
				graftPath = filepath.Join(t.TempDir(), "grafts")
			}
			graft := head + "\n" + unrelated + " " + fixture.release + "\n"
			mustWrite(t, graftPath, graft)
			if location == "ambient" {
				t.Setenv("GIT_GRAFT_FILE", graftPath)
			}
			control := exec.Command("git", "-c", "advice.graftFileDeprecated=false", "merge-base", "--is-ancestor", fixture.release, unrelated)
			control.Dir = fixture.root
			control.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=" + graftPath}
			if raw, err := control.CombinedOutput(); err != nil {
				t.Fatalf("fixture did not forge ancestry: %v %s", err, raw)
			}
			before := repositorySnapshotPublicationTest(t, fixture.root)
			requirePublicationStatus(t, fixture, head, publicationPass)
			requirePublicationStatus(t, fixture, unrelated, publicationStale)
			if after := repositorySnapshotPublicationTest(t, fixture.root); after != before {
				t.Fatal("graft qualification changed repository bytes")
			}
			after, err := os.ReadFile(graftPath)
			if err != nil || string(after) != graft {
				t.Fatalf("graft evidence changed: %v", err)
			}
		})
	}
}
