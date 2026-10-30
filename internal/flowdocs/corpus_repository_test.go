package flowdocs

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestFlowDocsCorpusRepositoryMismatchIsTypedUpFront builds a corpus whose primary repository is
// an evidence repository R0 and that declares the documented application A as an additional
// repository at the generation revision (issue 1479). Generate refuses it before rederiving the
// corpus against A, with a typed code naming both repositories and the additional declaration;
// a corpus of A itself at another revision is refused the same way (AFU-V1-091).
func TestFlowDocsCorpusRepositoryMismatchIsTypedUpFront(t *testing.T) {
	ctx := context.Background()
	app, appRev := fixture(t, map[string]string{"app/view.ts": "export function View() {\n fetch('/api');\n}\n"})
	evidenceRoot, evidenceRev := fixture(t, map[string]string{"docs/readme.md": "# Evidence\n"})
	appID, e := contextindex.CorpusRepositoryID(ctx, app, appRev)
	if e != nil {
		t.Fatal(e)
	}
	evidenceID, e := contextindex.CorpusRepositoryID(ctx, evidenceRoot, evidenceRev)
	if e != nil {
		t.Fatal(e)
	}
	rel, e := filepath.Rel(evidenceRoot, app)
	if e != nil {
		t.Fatal(e)
	}
	m, e := doccorpus.Inventory(ctx, evidenceRoot, evidenceRev, "docs", "2026-10-21T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	m.AdditionalRepositories = []doccorpus.AdditionalRepository{{ID: appID, Revision: appRev, Root: filepath.ToSlash(rel)}}
	artifact, e := doccorpus.Build(ctx, evidenceRoot, m)
	if e != nil {
		t.Fatal(e)
	}
	corpus, e := doccorpus.Encode(artifact)
	if e != nil {
		t.Fatal(e)
	}
	_, e = Generate(ctx, app, Options{Revision: appRev, Scope: "app", Corpus: corpus})
	var coded *doccorpus.Error
	if !errors.As(e, &coded) || coded.Code != CorpusRepositoryMismatchCode {
		t.Fatalf("multi-repository corpus with another primary: got %v, want %s", e, CorpusRepositoryMismatchCode)
	}
	for _, want := range []string{"corpus primary repository is " + evidenceID + " at " + evidenceRev, "generation repository is " + appID + " at " + appRev, "additional repository at " + appRev} {
		if !strings.Contains(coded.Message, want) {
			t.Fatalf("refusal %q lacks %q", coded.Message, want)
		}
	}

	// A corpus of the documented repository at an older revision is the same mismatch, without
	// the additional-repository clause.
	git(t, app, "commit", "-q", "--allow-empty", "-m", "next")
	next := git(t, app, "rev-parse", "HEAD")
	own, e := doccorpus.Inventory(ctx, app, appRev, "app", "2026-10-21T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	ownArtifact, e := doccorpus.Build(ctx, app, own)
	if e != nil {
		t.Fatal(e)
	}
	ownCorpus, e := doccorpus.Encode(ownArtifact)
	if e != nil {
		t.Fatal(e)
	}
	_, e = Generate(ctx, app, Options{Revision: next, Scope: "app", Corpus: ownCorpus})
	if !errors.As(e, &coded) || coded.Code != CorpusRepositoryMismatchCode || strings.Contains(coded.Message, "additional repository") || !strings.Contains(coded.Message, appID+" at "+appRev) || !strings.Contains(coded.Message, appID+" at "+next) {
		t.Fatalf("cross-revision corpus: got %v", e)
	}
	if _, e = Generate(ctx, app, Options{Revision: appRev, Scope: "app", Corpus: ownCorpus}); e != nil {
		t.Fatalf("matching corpus refused: %v", e)
	}
}
