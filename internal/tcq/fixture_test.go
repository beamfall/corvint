package tcq

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// testdata/stable-fixture.json carries the in-memory Git tree, the dynamic
// artifacts and the per-section expectations that
// src/context_corvint_test_claim.py produced over that tree. The tcq/0 whole
// documents and the cem/0.2 map they bound were retired with the cem/0.x wire
// (CEM-V1-217); the OCM now binds the cem/1.0 map digest and a fixed patch
// digest, and only the oracle's claims, units, issues and observation sections
// are kept, because those are computed by the unchanged TCQ machinery that
// tcq-stable/0 re-envelopes. Nothing here is derived from this package.

type stableDocuments struct {
	Base   string `json:"base"`
	Target string `json:"target"`
	Blobs  map[string]struct {
		OID    string `json:"oid"`
		Source string `json:"source"`
	} `json:"blobs"`
	Tree                map[string][]string                   `json:"tree"`
	OCM                 string                                `json:"ocm"`
	Command             string                                `json:"command"`
	Observation         string                                `json:"observation"`
	Report              string                                `json:"report"`
	DirtyCommand        string                                `json:"dirtyCommand"`
	DirtyObservation    string                                `json:"dirtyObservation"`
	RepeatedReport      string                                `json:"repeatedReport"`
	RepeatedObservation string                                `json:"repeatedObservation"`
	Oracle              map[string]map[string]json.RawMessage `json:"oracle"`
}

func loadDocuments(t *testing.T) stableDocuments {
	t.Helper()
	raw, err := os.ReadFile("testdata/stable-fixture.json")
	if err != nil {
		t.Fatalf("read stable fixture: %v", err)
	}
	var documents stableDocuments
	if err := json.Unmarshal(raw, &documents); err != nil {
		t.Fatalf("decode stable fixture: %v", err)
	}
	return documents
}

// fixtureRepository models the inherited Git boundary with an in-memory tree.
// It never reads a worktree path, which is the property TCQ-V0-001 depends on.
type fixtureRepository struct {
	blobs   map[string][]byte
	entries map[string]TreeEntry
}

func newFixtureRepository(documents stableDocuments) *fixtureRepository {
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

// fixtureVerifier stands in for the shipped stable verifier. TCQ-V0-042 forbids
// reordering validation internal to it, so the seam resolves the two OIDs and
// the accepted digests and nothing else.
type fixtureVerifier struct {
	base   string
	target string
	err    error
}

func (verifier fixtureVerifier) VerifyStableOCM(_, _ []byte, _, _ string) (StableBinding, error) {
	if verifier.err != nil {
		return StableBinding{}, verifier.err
	}
	return StableBinding{
		BaseRevision: verifier.base, TargetRevision: verifier.target,
		MapSha256: sha256Hex(stableTestCEM), PatchSha256: strings.Repeat("a", 64),
	}, nil
}

func newRequest(documents stableDocuments) Request {
	return Request{
		CEM: stableTestCEM, OCM: []byte(documents.OCM),
		ExpectedBase: documents.Base, Target: documents.Target,
	}
}

// TestStableSectionsMatchOracle asserts every claim row, unit row, issue row,
// observation summary and revision binding of the tcq-stable/0 result against
// the independent oracle, in static and three dynamic modes.
func TestStableSectionsMatchOracle(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	cases := []struct {
		name        string
		command     string
		observation string
		report      string
		oracle      string
	}{
		{"static", "", "", "", "static"},
		{"dynamic", documents.Command, documents.Observation, documents.Report, "dynamic"},
		{"not-attested-clean", documents.DirtyCommand, documents.DirtyObservation, documents.Report, "dirty"},
		{"repeated-rows", documents.Command, documents.RepeatedObservation, documents.RepeatedReport, "repeated"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := newRequest(documents)
			if testCase.command != "" {
				request.Command = []byte(testCase.command)
				request.Observation = []byte(testCase.observation)
				request.Report = []byte(testCase.report)
			}
			result, err := EvaluateStable(repository, verifier, request)
			if err != nil {
				t.Fatalf("EvaluateStable: %v", err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(result.Raw(), &got); err != nil {
				t.Fatal(err)
			}
			want := documents.Oracle[testCase.oracle]
			if len(want) == 0 {
				t.Fatalf("oracle %q is empty", testCase.oracle)
			}
			for key, expected := range want {
				if normalizedJSON(t, got[key]) != normalizedJSON(t, expected) {
					t.Errorf("%s mismatch\n got: %s\nwant: %s", key, got[key], expected)
				}
			}
		})
	}
}

func normalizedJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if raw == nil {
		return "<absent>"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestCacheVerificationAcceptsComputedBytes exercises TCQ-V0-043 on the stable
// route: a cached result is verified by full recomputation, never trusted.
func TestCacheVerificationAcceptsComputedBytes(t *testing.T) {
	documents := loadDocuments(t)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	request := newRequest(documents)
	computed, err := EvaluateStable(newFixtureRepository(documents), verifier, request)
	if err != nil {
		t.Fatal(err)
	}
	request.Expected = computed.Raw()
	if _, err := EvaluateStable(newFixtureRepository(documents), verifier, request); err != nil {
		t.Fatalf("cache verification rejected the computed result: %v", err)
	}
	tampered := append([]byte(nil), computed.Raw()...)
	tampered[len(tampered)-3] ^= 0x01
	request.Expected = tampered
	if _, err := EvaluateStable(newFixtureRepository(documents), verifier, request); err == nil {
		t.Fatal("tampered cache bytes were accepted")
	}
}
