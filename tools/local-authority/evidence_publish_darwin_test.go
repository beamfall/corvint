//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
	"golang.org/x/sys/unix"
)

func stageFixture(t *testing.T) (authoritystore.EvidenceManifest, map[string][]byte, []byte) {
	t.Helper()
	blob := closureObject("blob", []byte("valid\n"), false)
	tree := closureTree("file", "100644", blob, false)
	commit := closureCommit(tree, "same", false)
	original := capsule{"corvint-source-capsule/0", []capsuleObject{blob, tree, commit}}
	c, _, err := completeCapsule(original, commit.OID, commit.OID, tree.OID)
	if err != nil {
		t.Fatal(err)
	}
	in := enrolledInput{root: authoritystore.RootDocument{RootID: "unit-only", Epoch: "0", Generation: "0", AuthorityGID: strconv.Itoa(os.Getegid())}, enrollment: localauthority.Enrollment{Binding: localauthority.Binding{Base: commit.OID, Target: commit.OID, Tree: tree.OID}}, raw: map[string][]byte{"objects.json": mustJSONLine(original)}, capsule: c}
	m, files, err := evidenceFiles(in, digest([]byte("fixture")))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := localauthority.Canonical(m)
	if err != nil {
		t.Fatal(err)
	}
	return m, files, raw
}
func TestEvidencePublicationUsesExactModesUnderRestrictiveUmask(t *testing.T) {
	old := unix.Umask(077)
	defer unix.Umask(old)
	root := t.TempDir()
	parent, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	m, files, raw := stageFixture(t)
	if err := stageAndPublishEvidence(parent, m.EnrollmentHandle, m, files, raw, uint32(os.Getuid()), uint32(os.Getegid())); err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(root, m.EnrollmentHandle)
	t.Cleanup(func() {
		held, err := os.Open(published)
		if err == nil {
			_ = removeEvidenceStageFD(held)
			held.Close()
			_ = os.Remove(published)
		}
	})
	if err := filepath.WalkDir(published, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := os.FileMode(0440)
		if d.IsDir() {
			mode = 0550
		}
		if info.Mode().Perm() != mode {
			t.Fatalf("%s has %o", p, info.Mode().Perm())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := stageAndPublishEvidence(parent, m.EnrollmentHandle, m, files, raw, uint32(os.Getuid()), uint32(os.Getegid())); err == nil {
		t.Fatal("existing publication overwritten")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != m.EnrollmentHandle {
		t.Fatalf("publication namespace: %v %v", entries, err)
	}
}
func TestEvidencePublicationFailureRunsStageCleanup(t *testing.T) {
	root := t.TempDir()
	parent, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	m, files, raw := stageFixture(t)
	files[".git/HEAD"] = []byte("wrong bytes")
	if err := stageAndPublishEvidence(parent, m.EnrollmentHandle, m, files, raw, uint32(os.Getuid()), uint32(os.Getegid())); err == nil {
		t.Fatal("bad stage published")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failure left stage/publication: %v %v", entries, err)
	}
}
