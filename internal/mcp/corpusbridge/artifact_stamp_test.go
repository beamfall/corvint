// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCRV1417WarmCallsSkipUnchangedArtifactRead: once a call has read the configured artifact
// and matched the startup digest, later calls do not read or hash it again while its file keeps
// the same stat stamp; a rewrite changes the stamp and is refused as before, and a stamp inside
// the racy window always reads.
func TestDCRV1417WarmCallsSkipUnchangedArtifactRead(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	reads := 0
	saved := readArtifactFile
	t.Cleanup(func() { readArtifactFile = saved })
	readArtifactFile = func(root, path string) ([]byte, error) {
		reads++
		return saved(root, path)
	}
	root, _ := multiRepositoryFixture(t, [3]int{3, 3, 3})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`)
	if reads != 1 {
		t.Fatalf("first call read the artifact %d times, want once", reads)
	}
	for range 3 {
		freshnessCall(t, r, "corvint.docs_search", `{"query":"quokka"}`)
	}
	if reads != 1 {
		t.Fatalf("warm calls read an unchanged artifact: %d reads", reads)
	}
	path := filepath.Join(root, "corpus.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, failure, _, _ := r.CallTimed(t.Context(), "corvint.docs_info", []byte(`{}`)); failure == nil || failure.Code != "corpus-refused" || reads != 2 {
		t.Fatalf("rewritten artifact: %+v after %d reads", failure, reads)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	stampRacyWindow = 1 << 62
	freshnessCall(t, r, "corvint.docs_info", `{}`)
	freshnessCall(t, r, "corvint.docs_info", `{}`)
	if reads != 4 {
		t.Fatalf("racy artifact stamp skipped a read: %d reads", reads)
	}
	if doccorpus.Digest(data) != r.digest {
		t.Fatal("fixture digest drifted")
	}
}

// TestDCRV1417WarmStampRefusesParentSymlink: a warm call accepts an unchanged artifact stamp only
// through the artifact reader's no-follow walk. Replacing the artifact's directory with a symlink
// to the same directory keeps the file's inode, size and times, yet the next call reads the
// artifact and is refused as a fresh read is, instead of serving the memoized artifact.
func TestDCRV1417WarmStampRefusesParentSymlink(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	reads := 0
	saved := readArtifactFile
	t.Cleanup(func() { readArtifactFile = saved })
	readArtifactFile = func(root, path string) ([]byte, error) {
		reads++
		return saved(root, path)
	}
	root, _ := multiRepositoryFixture(t, [3]int{3, 3, 3})
	if err := os.Mkdir(filepath.Join(root, "artifacts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "corpus.json"), filepath.Join(root, "artifacts", "corpus.json")); err != nil {
		t.Fatal(err)
	}
	r, e := New(root, "artifacts/corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	freshnessCall(t, r, "corvint.docs_info", `{}`)
	freshnessCall(t, r, "corvint.docs_info", `{}`)
	if reads != 1 {
		t.Fatalf("warm call read an unchanged artifact: %d reads", reads)
	}
	if err := os.Rename(filepath.Join(root, "artifacts"), filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("moved", filepath.Join(root, "artifacts")); err != nil {
		t.Fatal(err)
	}
	if _, err := doccorpus.ReadCorpusFile(root, "artifacts/corpus.json"); err == nil {
		t.Fatal("a fresh read followed the parent symlink")
	}
	_, _, failure, _, _ := r.CallTimed(t.Context(), "corvint.docs_info", []byte(`{}`))
	if failure == nil || reads != 2 {
		t.Fatalf("parent symlink served the memoized artifact: %+v after %d reads", failure, reads)
	}
}
