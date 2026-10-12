package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1164ConsoleVerifiedStableChain (CEM-V1-164 to CEM-V1-166): started
// with an artifact root, the chain pane draws a sealed cem/1.0 change only on
// a stable verifier ACCEPT against the request's base, and a missing base, a
// wrong base, edited sealed bytes, a tampered artifact or a missing artifact
// root each replace the chain with a refusal. Without the root the
// display-only NOT_RUN pane is unchanged.
func TestCEMV1164ConsoleVerifiedStableChain(t *testing.T) {
	change := stablefixture.NewChange(t)
	stable := change.Stable(t)
	sealed := changesDir + "/" + stable.Target + ".cem.json"
	seal := func(data []byte, message string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(change.Root, filepath.FromSlash(changesDir)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(change.Root, filepath.FromSlash(sealed)), data, 0o644); err != nil {
			t.Fatal(err)
		}
		change.Git("add", "-f", sealed)
		change.Git("commit", "-qm", message)
	}
	change.Git("rm", "-q", "--cached", wire.ExcludedCEMPath)
	seal(stable.Map, "seal")
	page := func(artifacts, query string) string {
		t.Helper()
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: change.Root, Binary: "atm", StableArtifacts: artifacts})
		if err != nil {
			t.Fatal(err)
		}
		return mainOf(get(t, server, "/chain?change="+stable.Target+query))
	}
	accepted := page(stable.ArtifactRoot, "&base="+change.Base)
	for _, want := range []string{"stable verifier ACCEPT", "changeIntegrity VERIFIED", "sidecar EXACT", "nativeAuthority NOT_OBSERVED"} {
		if !strings.Contains(accepted, want) {
			t.Fatalf("verified chain lacks %q:\n%s", want, accepted)
		}
	}
	if strings.Contains(accepted, "stable verification is NOT_RUN here") {
		t.Fatal("verified chain kept the NOT_RUN envelope")
	}
	if display := page("", ""); !strings.Contains(display, "stable verification is NOT_RUN here") || strings.Contains(display, "stable verifier ACCEPT") {
		t.Fatalf("display-only pane changed:\n%s", display)
	}
	refused := func(name, body, reason string) {
		t.Helper()
		if !strings.Contains(body, "cannot be drawn as a chain") || !strings.Contains(body, reason) || strings.Contains(body, "<h3>Change binding</h3>") {
			t.Fatalf("%s did not refuse with %q:\n%s", name, reason, body)
		}
	}
	refused("missing base", page(stable.ArtifactRoot, ""), "add base=")
	refused("wrong base", page(stable.ArtifactRoot, "&base="+change.LegacyTarget), "the stable verifier returned")
	refused("missing artifact root", page(filepath.Join(t.TempDir(), "absent"), "&base="+change.Base), "the stable verifier returned")

	artifact := filepath.Join(stable.ArtifactRoot, filepath.FromSlash(stable.ArtifactPaths[0]))
	original, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refused("tampered artifact", page(stable.ArtifactRoot, "&base="+change.Base), "the stable verifier returned")
	if err := os.WriteFile(artifact, original, 0o644); err != nil {
		t.Fatal(err)
	}
	seal([]byte(strings.Replace(string(stable.Map), "{", "{ ", 1)), "edit sealed bytes")
	refused("edited sealed bytes", page(stable.ArtifactRoot, "&base="+change.Base), "no chain is drawn")
}
