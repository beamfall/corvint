package corpusbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// replaceArtifact writes data to a new file beside corpus.json and renames it over the served one,
// as a rebuild does.
func replaceArtifact(t *testing.T, root string, data []byte) {
	t.Helper()
	next := filepath.Join(root, "corpus.json.next")
	if err := os.WriteFile(next, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, filepath.Join(root, "corpus.json")); err != nil {
		t.Fatal(err)
	}
}

// rebuiltArtifact commits one more page and builds the corpus at the new HEAD.
func rebuiltArtifact(t *testing.T, root string) []byte {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "docs", "p3.md"), []byte("# Page 3\n\nCheckout step 3.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("add", "docs")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "page 3")
	m, err := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "docs", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func searchDigest(t *testing.T, r *Registry) (string, *ToolFailure) {
	t.Helper()
	_, text, failure, protocol := r.Call(context.Background(), "corvint.docs_search", []byte(`{"query":"Checkout"}`))
	if protocol != nil {
		t.Fatalf("protocol error %v", protocol)
	}
	if failure != nil {
		return "", failure
	}
	var receipt struct {
		ArtifactSHA256 string `json:"artifact_sha256"`
	}
	if err := json.Unmarshal([]byte(text), &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt.ArtifactSHA256, nil
}

// DMF-V1-011: a server that answered one docs_search call answers the next one, after the artifact
// is rebuilt and renamed over the served file, from the new artifact without a restart, and its
// receipt names the new digest; the tool list follows the new header.
func TestDMFV1011RebuiltArtifactServedWithoutRestart(t *testing.T) {
	root, data := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	first, failure := searchDigest(t, r)
	if failure != nil || first == "" || r.ArtifactSHA256() != doccorpus.Digest(data) {
		t.Fatalf("first call %q %+v", first, failure)
	}
	rebuilt := rebuiltArtifact(t, root)
	replaceArtifact(t, root, rebuilt)
	second, failure := searchDigest(t, r)
	fresh, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	want, _ := searchDigest(t, fresh)
	if failure != nil || second != want || second == first || r.ArtifactSHA256() != doccorpus.Digest(rebuilt) {
		t.Fatalf("after the rebuild %q (a fresh server answers %q) %+v", second, want, failure)
	}
	if third, failure := searchDigest(t, r); failure != nil || third != second {
		t.Fatalf("reuse after the reload %q %+v", third, failure)
	}
	if len(r.Tools()) == 0 {
		t.Fatal("no tools after the reload")
	}
}

// DMF-V1-011: an artifact replaced by one that fails verification is refused with the reason, and
// keeps being refused: the previous artifact is never answered from.
func TestDMFV1011CorruptRebuildRefused(t *testing.T) {
	root, data := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	served, failure := searchDigest(t, r)
	if failure != nil {
		t.Fatal(failure)
	}
	corrupt := bytes.Replace(data, []byte("Checkout step 1"), []byte("Checkout step X"), 1)
	if bytes.Equal(corrupt, data) {
		t.Fatal("fixture text not found")
	}
	replaceArtifact(t, root, corrupt)
	for range 2 {
		digest, failure := searchDigest(t, r)
		if failure == nil || failure.Code != "corpus-refused" || !strings.Contains(failure.Message, "failed verification") || !strings.Contains(failure.Message, "not served") || digest != "" {
			t.Fatalf("corrupt rebuild answered %q %+v", digest, failure)
		}
	}
	if r.ArtifactSHA256() != doccorpus.Digest(data) {
		t.Fatal("a refused artifact replaced the served digest")
	}
	replaceArtifact(t, root, data)
	if digest, failure := searchDigest(t, r); failure != nil || digest != served {
		t.Fatalf("restored artifact %q %+v", digest, failure)
	}
}
