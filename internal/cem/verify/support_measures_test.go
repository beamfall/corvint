package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// citationMeasureVectors is protocol/cem-1.0/stable/citation-vectors/measures.json:
// library-level inputs of the CEM-V1-082 block and its expected bytes.
// interop/cem01-go asserts the same file.
type citationMeasureVectors struct {
	Profile string `json:"profile"`
	Cases   []struct {
		Name     string `json:"name"`
		Evidence []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
			Span string `json:"span"`
		} `json:"evidence"`
		Hunks []struct {
			ID          string      `json:"id"`
			Path        string      `json:"path"`
			Deleted     bool        `json:"deleted"`
			Disposition string      `json:"disposition"`
			Added       []string    `json:"added"`
			Basis       [][2]string `json:"basis"`
		} `json:"hunks"`
		Citation string `json:"citation"`
	} `json:"cases"`
}

// TestCEMV1082MeasureVectorsNative pins concentratedHunks at maxFanOut 1
// (V1-1331), the annotation stop terms on short code lines (V1-1335) and the
// base path of a deletion hunk in samePathCitations (V1-1332).
func TestCEMV1082MeasureVectorsNative(t *testing.T) {
	t.Parallel()
	top := repositoryRoot(t)
	checkStableCaseManifest(t, top, "citation-vectors/")
	raw, err := os.ReadFile(filepath.Join(top, "protocol/cem-1.0/stable/citation-vectors/measures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vectors citationMeasureVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if vectors.Profile != "cem-citation-measure-vectors/1" || len(vectors.Cases) == 0 {
		t.Fatalf("measures.json profile %q with %d cases", vectors.Profile, len(vectors.Cases))
	}
	for _, tc := range vectors.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			document := &wire.Map{Spec: wire.StableSpec}
			parsed := &patch.Patch{}
			spans := map[string][]byte{}
			for _, record := range tc.Evidence {
				document.Evidence = append(document.Evidence, wire.Evidence{ID: record.ID, Path: record.Path})
				spans[record.ID] = []byte(record.Span)
			}
			for _, h := range tc.Hunks {
				oldPath, newPath := h.Path, h.Path
				hunk := &patch.Hunk{ID: h.ID, OldPath: &oldPath, NewPath: &newPath, Body: []patch.BodyLine{{Prefix: ' ', Payload: []byte("context\n")}}}
				if h.Deleted {
					hunk.NewPath = nil
				}
				for _, line := range h.Added {
					hunk.Body = append(hunk.Body, patch.BodyLine{Prefix: '+', Payload: []byte(line)})
				}
				parsed.Hunks = append(parsed.Hunks, hunk)
				mapped := wire.Hunk{ID: h.ID, Path: h.Path, Disposition: h.Disposition}
				for _, entry := range h.Basis {
					mapped.Basis = append(mapped.Basis, wire.Basis{EvidenceID: entry[0], Relation: entry[1]})
				}
				document.Hunks = append(document.Hunks, mapped)
			}
			if got := string(supportJSON(t, ComputeCitation(document, parsed, spans))); got != tc.Citation {
				t.Fatalf("citation\n got %s\nwant %s", got, tc.Citation)
			}
		})
	}
}
