package corpusbridge

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// searchRankingFixture is the committed GitHub #1597 ranking fixture: a small
// source tree, compact supplied records anchored at its first lines, and a
// docs corpus eval case file of file-lookup and claim-lookup cases.
const searchRankingFixture = "testdata/search-ranking-gh1597"

type fixtureRecords struct {
	Subjects []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Path string `json:"path"`
		Name string `json:"name"`
	} `json:"subjects"`
	Claims []struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		Path    string `json:"path"`
		Text    string `json:"text"`
	} `json:"claims"`
	// Owned lists source inputs the fixture provider claims in place of the native one.
	Owned []string `json:"owned,omitempty"`
}

// searchRankingCorpus builds the fixture corpus in a fresh repository: the
// source tree is committed and inventoried natively, then the supplied
// records are expanded into a provider record whose anchors are pinned to
// the source revision's blobs and span digests. Inputs the records list as owned
// are handed to the fixture provider.
func searchRankingCorpus(t *testing.T, fixture string) string {
	t.Helper()
	root := t.TempDir()
	tree := filepath.Join(fixture, "repo")
	if err := filepath.WalkDir(tree, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tree, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@invalid"}, args...)...)
		c.Dir = root
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("add", "src")
	git("commit", "-qm", "source")
	source := git("rev-parse", "HEAD")
	m, err := doccorpus.Inventory(context.Background(), root, source, "src", "2026-10-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "records.json"))
	if err != nil {
		t.Fatal(err)
	}
	var records fixtureRecords
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	evidence := func(p string) doccorpus.Evidence {
		data, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		line, _, _ := bytes.Cut(data, []byte("\n"))
		anchor := doccorpus.Anchor{Repository: m.Repository.ID, Revision: source, Path: p, Blob: git("rev-parse", source+":"+p), SHA256: doccorpus.Digest(data), Start: 1, End: 1, SpanSHA256: doccorpus.Digest(append(line, '\n')), Authority: "external-provider", Kind: "declared", Reason: "fixture provider assertion"}
		return doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{anchor}, Limitations: []string{"fixture declaration; does not prove behavior"}}
	}
	record := doccorpus.ProviderRecord{Schema: doccorpus.ProviderSchema, ID: "fixture", Version: "1", Source: m.Repository, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{}, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "fixture subjects"}, {Name: "claims", State: "present", Reason: "fixture claims"}}}
	for _, s := range records.Subjects {
		record.Subjects = append(record.Subjects, doccorpus.Subject{ID: s.ID, Kind: s.Kind, Name: s.Name, Provider: "fixture", Evidence: evidence(s.Path)})
	}
	for _, c := range records.Claims {
		record.Claims = append(record.Claims, doccorpus.Claim{ID: c.ID, Subject: c.Subject, Text: c.Text, Provider: "fixture", Evidence: evidence(c.Path)})
	}
	data, err := doccorpus.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/provider.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "evidence/provider.json")
	git("commit", "-qm", "provider")
	revision := git("rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "fixture", Kind: "records", Version: "1", Revision: revision, Record: "evidence/provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "evidence/provider.json", Revision: revision})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "evidence/provider.json", Revision: revision, Blob: git("rev-parse", revision+":evidence/provider.json"), SHA256: doccorpus.Digest(data), Provider: "fixture", Purpose: "provider"})
	for i, in := range m.Inputs {
		if in.Purpose == "source" && slices.Contains(records.Owned, in.Path) {
			m.Inputs[i].Provider = "fixture"
		}
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// DCS-V1-404 (GitHub #1597): the committed fixture's file-lookup cases pass
// without a kind filter and its claim-lookup cases pass with
// include:["claims"], every case answered through the docs_search MCP tool
// path, deterministically across runs.
func TestDCSV1404SearchRankingEvalFixture(t *testing.T) {
	root := searchRankingCorpus(t, searchRankingFixture)
	cases, err := os.ReadFile(filepath.Join(searchRankingFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := evalRun(t, root, string(cases))
	for _, c := range report.Cases {
		if c.Status != "pass" {
			t.Errorf("%s: %s (rank %v, unresolved %v)", c.ID, c.Status, c.Rank, c.Unresolved)
		}
	}
	for _, tag := range []string{"file-lookup", "claim-lookup"} {
		if g := report.Summary.ByTag[tag]; g.Cases != 6 || g.Passed != g.Cases {
			t.Errorf("%s: %d of %d passed", tag, g.Passed, g.Cases)
		}
	}
	again := evalRun(t, root, string(cases))
	for i, c := range report.Cases {
		if c.Status != again.Cases[i].Status || (c.Rank == nil) != (again.Cases[i].Rank == nil) || c.Rank != nil && *c.Rank != *again.Cases[i].Rank {
			t.Fatalf("%s not deterministic: %+v vs %+v", c.ID, c, again.Cases[i])
		}
	}
}
