package testvaliditydoc

// Shared discovery test helpers. This file carries no build constraint so
// every test in the package, including ones that must vet on Windows, can use
// them (V1-1810). Tests that need Unix-only behaviour (FIFOs, Discover's
// receipt reads, which read_other.go refuses) stay in files constrained to
// darwin || linux, such as discover_test.go.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// discoveryRoot is a symlink-resolved worktree root, so bound absolute paths
// compare against the same spelling Discover confines to.
func discoveryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeAt(t *testing.T, path, body string, modified time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func evidencePath(root, name string) string {
	return filepath.Join(root, filepath.FromSlash(EvidenceDirectory), name)
}

// boundReceipt is a JavaScript receipt binding one test file's current digest.
func boundReceipt(t *testing.T, root, testName string) string {
	t.Helper()
	source := filepath.Join(root, "src", "add.test.js")
	writeAt(t, source, "test('adds')\n", time.Unix(1_700_000_000, 0))
	sum := sha256.Sum256([]byte("test('adds')\n"))
	key, err := json.Marshal(source) // a Windows path's backslashes must be escaped
	if err != nil {
		t.Fatal(err)
	}
	return `{"receipt":{"kind":"unit","identity":{"testFileDigests":{` + string(key) + `:"` + hex.EncodeToString(sum[:]) +
		`"}},"tests":[{"name":"` + testName + `","state":"passed"}]}}`
}

func discover(t *testing.T, root string) Document {
	t.Helper()
	document, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if document.Discovery == nil || document.Discovery.Location != EvidenceDirectory {
		t.Fatalf("discovery=%+v", document.Discovery)
	}
	return document
}

// V1-1810: the shared helpers compile and behave on every platform; this test
// is deliberately unconstrained so `GOOS=windows go vet` covers its use of them.
func TestDiscoveryHelpersArePortable(t *testing.T) {
	root := discoveryRoot(t)
	modified := time.Unix(1_800_000_000, 0)
	path := evidencePath(root, "receipt.json")
	if want := filepath.Join(root, ".corvint", "test-evidence", "receipt.json"); path != want {
		t.Fatalf("evidencePath=%q want %q", path, want)
	}
	body := boundReceipt(t, root, "adds")
	writeAt(t, path, body, modified)
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(modified) {
		t.Fatalf("stat=%v err=%v", info, err)
	}
	var receipt struct {
		Receipt struct {
			Identity struct {
				TestFileDigests map[string]string `json:"testFileDigests"`
			} `json:"identity"`
		} `json:"receipt"`
	}
	if err := json.Unmarshal([]byte(body), &receipt); err != nil {
		t.Fatalf("boundReceipt is not JSON: %v", err)
	}
	sum := sha256.Sum256([]byte("test('adds')\n"))
	source := filepath.Join(root, "src", "add.test.js")
	if digests := receipt.Receipt.Identity.TestFileDigests; len(digests) != 1 || digests[source] != hex.EncodeToString(sum[:]) {
		t.Fatalf("digests=%v", digests)
	}
}
