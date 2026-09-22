package tcq

import (
	"encoding/json"
	"os"
	"testing"
)

// The whole-document vectors in testdata/oracle-documents.json are produced by
// src/context_corvint_test_claim.py with its Git boundary replaced by the same
// in-memory fixture tree modelled below. Nothing here is derived from this
// package.

type oracleDocuments struct {
	Base   string `json:"base"`
	Target string `json:"target"`
	Blobs  map[string]struct {
		OID    string `json:"oid"`
		Source string `json:"source"`
	} `json:"blobs"`
	Tree                map[string][]string `json:"tree"`
	OCM                 string              `json:"ocm"`
	CEM                 string              `json:"cem"`
	CEM01               string              `json:"cem01"`
	Static              string              `json:"static"`
	Command             string              `json:"command"`
	Observation         string              `json:"observation"`
	Report              string              `json:"report"`
	Dynamic             string              `json:"dynamic"`
	DirtyCommand        string              `json:"dirtyCommand"`
	DirtyObservation    string              `json:"dirtyObservation"`
	Dirty               string              `json:"dirty"`
	RepeatedReport      string              `json:"repeatedReport"`
	RepeatedObservation string              `json:"repeatedObservation"`
	Repeated            string              `json:"repeated"`
}

func loadDocuments(t *testing.T) oracleDocuments {
	t.Helper()
	raw, err := os.ReadFile("testdata/oracle-documents.json")
	if err != nil {
		t.Fatalf("read oracle documents: %v", err)
	}
	var documents oracleDocuments
	if err := json.Unmarshal(raw, &documents); err != nil {
		t.Fatalf("decode oracle documents: %v", err)
	}
	return documents
}

// fixtureRepository models the inherited Git boundary with an in-memory tree.
// It never reads a worktree path, which is the property TCQ-V0-001 depends on.
type fixtureRepository struct {
	blobs   map[string][]byte
	entries map[string]TreeEntry
}

func newFixtureRepository(documents oracleDocuments) *fixtureRepository {
	repository := &fixtureRepository{blobs: map[string][]byte{}, entries: map[string]TreeEntry{}}
	for _, blob := range documents.Blobs {
		repository.blobs[blob.OID] = []byte(blob.Source)
	}
	for path, entry := range documents.Tree {
		repository.entries[path] = TreeEntry{Mode: entry[0], Type: entry[1], Found: true}
	}
	return repository
}

func (repository *fixtureRepository) Blob(oid string) ([]byte, error) {
	data, ok := repository.blobs[oid]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (repository *fixtureRepository) TreeEntry(_, path string) (TreeEntry, error) {
	return repository.entries[path], nil
}

// fixtureVerifier stands in for the shared CEM/OCM verifier. TCQ-V0-042 forbids
// reordering validation internal to it, so the seam resolves the two OIDs and
// nothing else.
type fixtureVerifier struct {
	base   string
	target string
	err    error
}

func (verifier fixtureVerifier) VerifyOCM(_, _ []byte, _, _ string) (Resolved, error) {
	if verifier.err != nil {
		return Resolved{}, verifier.err
	}
	return Resolved{BaseRevision: verifier.base, TargetRevision: verifier.target}, nil
}

func newRequest(documents oracleDocuments) Request {
	return Request{
		CEM: []byte(documents.CEM), OCM: []byte(documents.OCM),
		ExpectedBase: documents.Base, Target: documents.Target,
	}
}

// TestDocumentsMatchOracle asserts the complete canonical `tcq/0` bytes — every
// claim row, unit row, issue row, observation summary, and the `tcq:sha256:`
// identity — against the oracle, in static and three dynamic modes.
func TestDocumentsMatchOracle(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	cases := []struct {
		name        string
		command     string
		observation string
		report      string
		want        string
	}{
		{"static", "", "", "", documents.Static},
		{"dynamic", documents.Command, documents.Observation, documents.Report, documents.Dynamic},
		{"not-attested-clean", documents.DirtyCommand, documents.DirtyObservation, documents.Report, documents.Dirty},
		{"repeated-rows", documents.Command, documents.RepeatedObservation, documents.RepeatedReport, documents.Repeated},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := newRequest(documents)
			if testCase.command != "" {
				request.Command = []byte(testCase.command)
				request.Observation = []byte(testCase.observation)
				request.Report = []byte(testCase.report)
			}
			result, err := Evaluate(repository, verifier, request)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if string(result.Raw()) != testCase.want {
				t.Fatalf("document mismatch\n got: %s\nwant: %s", result.Raw(), testCase.want)
			}
		})
	}
}

// TestCacheVerificationAcceptsOracleBytes exercises TCQ-V0-043: a cached result
// is verified by full recomputation, never trusted.
func TestCacheVerificationAcceptsOracleBytes(t *testing.T) {
	documents := loadDocuments(t)
	request := newRequest(documents)
	request.Expected = []byte(documents.Static)
	if _, err := Evaluate(newFixtureRepository(documents), fixtureVerifier{base: documents.Base, target: documents.Target}, request); err != nil {
		t.Fatalf("cache verification rejected the oracle document: %v", err)
	}
	tampered := append([]byte(nil), documents.Static...)
	tampered[len(tampered)-3] ^= 0x01
	request.Expected = tampered
	if _, err := Evaluate(newFixtureRepository(documents), fixtureVerifier{base: documents.Base, target: documents.Target}, request); err == nil {
		t.Fatal("tampered cache bytes were accepted")
	}
}
