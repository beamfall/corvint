package verify

import (
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"os"
	"testing"
)

func TestStableLiteralRepository(t *testing.T) {
	p := os.Getenv("CEM_STABLE_FIXTURE_ROOT")
	if p == "" {
		t.Skip("literal fixtures required")
	}
	r, close, e := stableOpenRoot(p + "/repositories/sha1/.git")
	if e != nil {
		t.Fatal("root", e)
	}
	defer close()
	_, e = stableReadArtifact(r, "config", 4096)
	if e != nil {
		t.Fatal("read", e)
	}
	if e = stableRepositoryEnvelope(p + "/repositories/sha1"); e != nil {
		t.Fatal("envelope", e)
	}
	_, e = gitauth.Open(p+"/repositories/sha1", gitrun.NewDefaultBudget())
	if e != nil {
		t.Fatal("repository", e)
	}
}
