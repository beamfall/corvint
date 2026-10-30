// Package testfixture supplies a real committed, uncovered denominator for adapter parity tests.
package testfixture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowcoverage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func Repository(t testing.TB) string {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = root
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git: %s %v", b, e)
		}
		return strings.TrimSpace(string(b))
	}
	put := func(p string, v any) {
		b, e := flowcoverage.Encode(v)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(root, p), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	for _, p := range []string{"flows", "receipts"} {
		if e = os.Mkdir(filepath.Join(root, p), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e = os.WriteFile(filepath.Join(root, "receipts/.keep"), []byte("empty declared run scope\n"), 0600); e != nil {
		t.Fatal(e)
	}
	put("flows/search.json", appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: "search", Revision: 1, Kind: "ui", Actor: "visitor", Preconditions: []string{}, Steps: []appflows.FlowStep{{StepID: "search", Action: "search"}}, Outcomes: []appflows.FlowOutcome{{OutcomeID: "found", Behavior: "found", Matcher: "toBeVisible", Locator: "#results", Value: "visible"}}, Variations: []appflows.FlowVariation{}, Links: []appflows.FlowLink{}})
	git("add", ".")
	git("commit", "-qm", "accepted uncovered intent")
	rev := git("rev-parse", "HEAD")
	id, e := contextindex.CorpusRepositoryID(context.Background(), root, rev)
	if e != nil {
		t.Fatal(e)
	}
	set, e := appflows.LoadIntentsAt(context.Background(), root, "flows", rev)
	if e != nil {
		t.Fatal(e)
	}
	b, e := flowcoverage.Encode(set.Flows)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	put("denominator.json", flowcoverage.Denominator{Schema: flowcoverage.DenominatorSchema, Source: doccorpus.Repository{ID: id, Revision: rev}, Intents: flowcoverage.IntentDirectory{Directory: "flows", Revision: rev}, Inventory: flowcoverage.Inventory{Kind: "intents", Ref: flowcoverage.Ref{Path: "flows", Revision: rev, SHA256: hex.EncodeToString(h[:])}}, Flows: []flowcoverage.Mapping{{DocumentedID: "search", IntentFlowIDs: []string{"search"}}}, Variations: []flowcoverage.Variation{}})
	put("runs.json", flowcoverage.Runs{Schema: flowcoverage.RunsSchema, Directory: "receipts", Revision: rev, Runs: []flowcoverage.Run{}})
	git("add", ".")
	git("commit", "-qm", "coverage inventory")
	return root
}
