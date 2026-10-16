package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCEMV1082MeasureVectorsPortable computes every library-level case of
// protocol/cem-1.0/stable/citation-vectors/measures.json and requires the
// native verifier's block bytes: concentratedHunks at maxFanOut 1 (V1-1331),
// the annotation stop terms on short code lines (V1-1335) and the base path
// of a deletion hunk in samePathCitations (V1-1332). internal/cem/verify
// asserts the same file.
func TestCEMV1082MeasureVectorsPortable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(stableCaseFiles(t, "citation-vectors"), "measures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
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
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if vectors.Profile != "cem-citation-measure-vectors/1" || len(vectors.Cases) == 0 {
		t.Fatalf("measures.json profile %q with %d cases", vectors.Profile, len(vectors.Cases))
	}
	for _, tc := range vectors.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			var records []evidence
			for _, e := range tc.Evidence {
				records = append(records, evidence{ID: e.ID, Path: e.Path, data: []byte(e.Span)})
			}
			var hunks []mappedHunk
			patch := map[string]*patchHunk{}
			for _, h := range tc.Hunks {
				oldPath, newPath := h.Path, h.Path
				p := &patchHunk{ID: h.ID, OldPath: &oldPath, NewPath: &newPath, Body: []bodyLine{{' ', []byte("context\n")}}}
				if h.Deleted {
					p.NewPath = nil
				}
				for _, line := range h.Added {
					p.Body = append(p.Body, bodyLine{'+', []byte(line)})
				}
				patch[h.ID] = p
				m := mappedHunk{ID: h.ID, Path: h.Path, Disposition: h.Disposition, Basis: []basis{}}
				for _, b := range h.Basis {
					m.Basis = append(m.Basis, basis{b[0], b[1]})
				}
				hunks = append(hunks, m)
			}
			got, err := json.Marshal(computeCitation(hunks, patch, records))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.Citation {
				t.Fatalf("citation\n got %s\nwant %s", got, tc.Citation)
			}
		})
	}
}
