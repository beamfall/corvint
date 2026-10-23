// SPDX-License-Identifier: AGPL-3.0-or-later

package corpusbridge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCPV1417WarmCallsSkipUnchangedArtifactRead: once a call has read the configured artifact
// and matched the startup digest, later calls do not read or hash it again while its file keeps
// the same stat stamp; a rewrite changes the stamp and is refused as before, and a stamp inside
// the racy window always reads.
func TestDCPV1417WarmCallsSkipUnchangedArtifactRead(t *testing.T) {
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
