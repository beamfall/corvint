package evalrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

type fixtureDocument struct {
	SchemaVersion int                 `json:"schema_version"`
	Repositories  []fixtureRepository `json:"repositories"`
}

type fixtureRepository struct {
	ScoredRevision string            `json:"scored_revision"`
	Traces         []json.RawMessage `json:"traces"`
}

type fixtureTrace struct {
	SchemaVersion int      `json:"schema_version"`
	Revision      string   `json:"revision"`
	TraceID       string   `json:"trace_id"`
	Task          string   `json:"task"`
	OpenedPaths   []string `json:"opened_paths"`
	ChangedPaths  []string `json:"changed_paths"`
	Verification  []string `json:"verification"`
	Outcome       string   `json:"outcome"`
}

type loadedFixture struct {
	path, sha256 string
	traces       []contextindex.QueryTrace
	producers    []string
}

// readFixture refuses a fixture larger than the trace store bound after reading one
// byte past it, instead of allocating the whole file (V1-0747).
func readFixture(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, trace.MaxTraceStoreBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > trace.MaxTraceStoreBytes {
		return nil, fmt.Errorf("fixture exceeds %d bytes", trace.MaxTraceStoreBytes)
	}
	return raw, nil
}

func loadTraceFixture(path string, index *contextindex.Index, cases []goldenCase) (loadedFixture, error) {
	raw, err := readFixture(path)
	if err != nil {
		return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document fixtureDocument
	if err := decoder.Decode(&document); err != nil {
		return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: trailing JSON value")
	}
	if document.SchemaVersion != 1 || document.Repositories == nil {
		return loadedFixture{}, fmt.Errorf("learned trace fixture must contain schema_version: 1 and repositories[]")
	}
	var selected *fixtureRepository
	seen := make(map[string]struct{}, len(document.Repositories))
	for position := range document.Repositories {
		repository := &document.Repositories[position]
		if !exactLowerHex(repository.ScoredRevision, len(index.CommitRevision)) || repository.Traces == nil {
			return loadedFixture{}, fmt.Errorf("invalid learned trace fixture repository")
		}
		if _, duplicate := seen[repository.ScoredRevision]; duplicate {
			return loadedFixture{}, fmt.Errorf("learned trace fixture contains duplicate scored revision: %s", repository.ScoredRevision)
		}
		seen[repository.ScoredRevision] = struct{}{}
		// The frozen fixture envelope validates every repository's row shape,
		// even when only one repository is selected for scoring below.
		for _, rawRow := range repository.Traces {
			var header struct {
				SchemaVersion int    `json:"schema_version"`
				Revision      string `json:"revision"`
			}
			if err := json.Unmarshal(rawRow, &header); err != nil {
				return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
			}
			if header.SchemaVersion == trace.SchemaVersionV2 || header.SchemaVersion == trace.SchemaVersionV3 {
				if _, err := trace.DecodeTyped(rawRow, header.Revision); err != nil {
					return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
				}
			} else {
				var fixture fixtureTrace
				decoder := json.NewDecoder(bytes.NewReader(rawRow))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&fixture); err != nil {
					return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
				}
			}
		}
		if repository.ScoredRevision == index.CommitRevision {
			selected = repository
		}
	}
	if selected == nil {
		return loadedFixture{}, fmt.Errorf("learned trace fixture does not contain scored revision: %s", index.CommitRevision)
	}
	if len(selected.Traces) > trace.MaxTraces {
		return loadedFixture{}, fmt.Errorf("learned trace fixture exceeds %d traces", trace.MaxTraces)
	}
	tracked := make([]string, 0, len(index.Sources))
	for path := range index.Sources {
		tracked = append(tracked, path)
	}
	sort.Strings(tracked)
	scoredTasks := make(map[string]struct{}, len(cases))
	for _, item := range cases {
		if task := scoredTask(item); task != "" {
			scoredTasks[task] = struct{}{}
		}
	}
	queries := make([]contextindex.QueryTrace, 0, len(selected.Traces))
	traceIDs := make(map[string]struct{}, len(selected.Traces))
	producers := make([]string, 0, len(selected.Traces))
	for _, rawRow := range selected.Traces {
		var header struct {
			SchemaVersion int    `json:"schema_version"`
			Revision      string `json:"revision"`
		}
		if err := json.Unmarshal(rawRow, &header); err != nil {
			return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
		}
		row := []byte(rawRow)
		if header.SchemaVersion == trace.SchemaVersionV2 || header.SchemaVersion == trace.SchemaVersionV3 {
			// Validate the original object before canonicalizing it for JSONL;
			// fixtures may indent rows, but duplicate members must still refuse.
			record, err := trace.DecodeTyped(rawRow, header.Revision)
			if err != nil {
				return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
			}
			row, err = trace.Encode(record)
			row = bytes.TrimSuffix(row, []byte("\n"))
			if err != nil {
				return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
			}
		} else {
			// Preserve the frozen v1 fixture decoder and its normalization.
			var fixture fixtureTrace
			decoder := json.NewDecoder(bytes.NewReader(rawRow))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&fixture); err != nil {
				return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
			}
			var err error
			row, err = json.Marshal(fixture)
			if err != nil {
				return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", err)
			}
		}
		records, decodeErr := trace.DecodeStore(append(row, '\n'), header.Revision, tracked)
		if decodeErr != nil {
			return loadedFixture{}, fmt.Errorf("invalid learned trace fixture: %v", decodeErr)
		}
		record := records[0]
		if _, duplicate := traceIDs[record.TraceID]; duplicate {
			return loadedFixture{}, fmt.Errorf("learned trace fixture contains duplicate trace ids")
		}
		traceIDs[record.TraceID] = struct{}{}
		if record.Revision == index.CommitRevision {
			return loadedFixture{}, fmt.Errorf("learned trace fixture trace shares a scored outcome commit: %s", record.Revision)
		}
		if _, contaminated := scoredTasks[contextindex.TrimPythonSpace(record.Task)]; contaminated {
			return loadedFixture{}, fmt.Errorf("learned trace fixture trace shares a scored task: %s", record.Task)
		}
		producers = append(producers, record.ProducerName())
		queries = append(queries, contextindex.QueryTrace{
			TraceID: record.TraceID, Task: record.Task, Outcome: record.Outcome, Revision: record.Revision,
			OpenedPaths: record.OpenedPaths, ChangedPaths: record.ChangedPaths,
		})
	}
	digest := sha256.Sum256(raw)
	return loadedFixture{path: path, sha256: fmt.Sprintf("%x", digest), traces: queries, producers: producers}, nil
}

func scoredTask(item goldenCase) string {
	switch item.Mode {
	case "query":
		return contextindex.TrimPythonSpace(item.Text)
	case "feature":
		return contextindex.TrimPythonSpace(item.FeatureID)
	default:
		return ""
	}
}

func exactLowerHex(value string, length int) bool {
	if len(value) != length || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
