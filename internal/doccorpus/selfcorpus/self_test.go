// Package selfcorpus_test holds the one documentation-corpus test that reads
// Corvint's own checkout. It lives apart from internal/doccorpus so that only
// this test, not the whole package, is selected for every repository change.
package selfcorpus_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func TestCorpusSelfDocumentation(t *testing.T) {
	t.Run("DCP-V1-019 self", func(t *testing.T) {
		root, err := filepath.Abs("../../..")
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git rev-parse HEAD: %v %s", err, out)
		}
		rev := strings.TrimSpace(string(out))
		m, err := doccorpus.Inventory(context.Background(), root, rev, "internal/doccompiler/draft.go", "2026-09-19T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		a, err := doccorpus.Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := doccorpus.Encode(a)
		r, err := doccorpus.ReadQuery(context.Background(), root, raw, doccorpus.Request{Operation: "search", Query: "DraftSources"})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Results) == 0 {
			t.Fatal("Corvint could not query its own committed documentation source")
		}
		t.Logf("self-corpus source=%s artifact=%s records=%d receipt_results=%d", rev, a.SHA256, len(a.Subjects), len(r.Results))
	})
}
