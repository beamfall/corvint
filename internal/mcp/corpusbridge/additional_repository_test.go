package corpusbridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCPV1178BridgeVerifiesPinsInOwningCheckout: the memoized artifact stays valid only while each
// pinned revision resolves in the checkout that owns it, so an additional repository's revision is
// verified in its declared checkout rather than the primary object store.
func TestDCPV1178BridgeVerifiesPinsInOwningCheckout(t *testing.T) {
	commit := func(dir string) string {
		t.Helper()
		for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-qm", "c"}, {"rev-parse", "HEAD"}} {
			c := exec.Command("git", append([]string{"-C", dir}, args...)...)
			c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
			out, err := c.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
			if args[0] == "rev-parse" {
				return strings.TrimSpace(string(out))
			}
		}
		return ""
	}
	parent := t.TempDir()
	root, lib := filepath.Join(parent, "app"), filepath.Join(parent, "lib")
	for _, dir := range []string{root, lib} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(dir+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	primary, extra := commit(root), commit(lib)
	m := doccorpus.Manifest{Repository: doccorpus.Repository{ID: primary, Revision: primary}, AdditionalRepositories: []doccorpus.AdditionalRepository{{ID: extra, Revision: extra, Root: "../lib"}}}
	if err := verifyPins(context.Background(), root, m, []string{primary, extra}); err != nil {
		t.Fatalf("additional repository pin not verified in its own checkout: %v", err)
	}
	m.AdditionalRepositories = nil
	if err := verifyPins(context.Background(), root, m, []string{primary, extra}); err == nil {
		t.Fatal("undeclared revision verified in the primary object store")
	}
}
