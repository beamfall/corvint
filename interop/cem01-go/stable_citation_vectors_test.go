package main

// CEM-V1-108..112 portable conformance: every case of the packet citation
// vectors under protocol/cem-1.0/stable/citation-vectors returns its pinned
// verdict, including the exact citation block bytes and verdictSha256 that
// the native verifier produced. internal/cem/verify asserts the same file.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
)

// citationVectorClasses are the classes the packet citation vectors must
// cover; the native verifier checks the same list.
var citationVectorClasses = []string{"control", "EvidenceDriftStale", "EvidenceDriftDeleted", "BaseRevisionMismatch",
	"UnsupportedSpec", "MissingField", "InvalidJSON", "NotAnObject"}

type citationVectorCase struct {
	Name          string      `json:"name"`
	Class         string      `json:"class"`
	Map           string      `json:"map"`
	Exit          int         `json:"exit"`
	Outcome       string      `json:"outcome"`
	Stage         string      `json:"stage"`
	Code          *string     `json:"code"`
	IssueCodes    []string    `json:"issueCodes"`
	Drift         [][2]string `json:"drift"`
	VerdictSha256 string      `json:"verdictSha256"`
	Citation      *string     `json:"citation"`
}

// TestCEMV1111CitationVectorsPortable runs every citation vector through the
// portable canonical verifier and asserts its pinned verdict: exit, outcome,
// stage, closed code, issue codes, drift statuses, verdictSha256, the
// top-level member order and the exact citation block bytes.
func TestCEMV1111CitationVectorsPortable(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := stableCaseFiles(t, "citation-vectors")
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Profile        string               `json:"profile"`
		ArtifactRoot   string               `json:"artifactRoot"`
		BaseRevision   string               `json:"baseRevision"`
		TargetRevision string               `json:"targetRevision"`
		ResultMembers  []string             `json:"resultMembers"`
		Cases          []citationVectorCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-citation-vectors/1" || want.ArtifactRoot != "." {
		t.Fatalf("EXPECTED.json profile %q artifactRoot %q", want.Profile, want.ArtifactRoot)
	}
	covered := map[string]bool{}
	for _, tc := range want.Cases {
		covered[tc.Class] = true
		if tc.Class == "control" {
			checkCitationVectorControl(t, dir, tc)
		}
	}
	for _, class := range citationVectorClasses {
		if !covered[class] {
			t.Errorf("citation vectors have no %s case", class)
		}
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			mapBytes, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, artifacts, nil)
			encoded, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			checkResultBytes(t, filepath.Join(dir, "results", tc.Name+".json"), r)
			if members := citationVectorMembers(t, encoded); !reflect.DeepEqual(members, want.ResultMembers) {
				t.Fatalf("result members %v, want %v", members, want.ResultMembers)
			}
			var got struct {
				VerdictSha256 string          `json:"verdictSha256"`
				Citation      json.RawMessage `json:"citation"`
			}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			drift := make([][2]string, 0, len(r.Drift))
			for _, d := range r.Drift {
				drift = append(drift, [2]string{d.Path, d.Status})
			}
			wantCitation := "null"
			if tc.Citation != nil {
				wantCitation = *tc.Citation
			}
			if exit != tc.Exit || r.Outcome != tc.Outcome || r.Stage != tc.Stage || !reflect.DeepEqual(r.Code, tc.Code) ||
				!reflect.DeepEqual(append([]string{}, r.IssueCodes...), append([]string{}, tc.IssueCodes...)) ||
				!reflect.DeepEqual(drift, append([][2]string{}, tc.Drift...)) || got.VerdictSha256 != tc.VerdictSha256 ||
				!bytes.Equal(got.Citation, []byte(wantCitation)) {
				t.Fatalf("runStableCanonical exit=%d result=%s, want the pinned %s verdict", exit, encoded, tc.Name)
			}
		})
	}
}

// checkCitationVectorControl holds the clean case to what CEM-V1-111 requires
// of it: anchored hunks, nonzero specificity and same-path citations, and a
// whole-file deletion hunk (new range 0,0) anchored through its path.
func checkCitationVectorControl(t *testing.T, dir string, tc citationVectorCase) {
	t.Helper()
	if tc.Citation == nil {
		t.Fatalf("control case %s has no citation block", tc.Name)
	}
	var block struct {
		Hunks []struct {
			HunkID  string   `json:"hunkId"`
			Tier    string   `json:"tier"`
			Anchors []string `json:"anchors"`
		} `json:"hunks"`
		HunksByClass struct {
			Anchored int `json:"anchored"`
		} `json:"hunksByClass"`
		SamePathCitations int `json:"samePathCitations"`
		Specificity       int `json:"specificity"`
	}
	if err := json.Unmarshal([]byte(*tc.Citation), &block); err != nil {
		t.Fatal(err)
	}
	if block.HunksByClass.Anchored == 0 || block.Specificity == 0 || block.SamePathCitations == 0 {
		t.Fatalf("control citation %s lacks anchored hunks, nonzero specificity or same-path citations", *tc.Citation)
	}
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.Map)))
	if err != nil {
		t.Fatal(err)
	}
	var stable struct {
		Hunks []struct {
			ID       string `json:"id"`
			NewRange struct {
				Start int `json:"start"`
				Count int `json:"count"`
			} `json:"newRange"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(raw, &stable); err != nil {
		t.Fatal(err)
	}
	deleted := map[string]bool{}
	for _, h := range stable.Hunks {
		deleted[h.ID] = h.NewRange.Start == 0 && h.NewRange.Count == 0
	}
	for _, row := range block.Hunks {
		if deleted[row.HunkID] && row.Tier == "anchored" && slices.Contains(row.Anchors, "path") {
			return
		}
	}
	t.Fatalf("control case %s has no whole-file deletion hunk anchored through its path", tc.Name)
}

func citationVectorMembers(t *testing.T, encoded []byte) []string {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		t.Fatalf("result is not an object: %v", err)
	}
	var members []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, token.(string))
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return members
}

// checkResultBytes renders result as both consumers write it, one compact
// object and an LF, and compares the whole line, nested members included, with
// the packet's pinned bytes (CEM-V1-113). The one environment member that can
// vary is runtime.goVersion: the pin carries go1.27.2, and another admitted
// toolchain's name is substituted before comparing.
func checkResultBytes(t *testing.T, path string, result any) {
	t.Helper()
	var line bytes.Buffer
	if err := json.NewEncoder(&line).Encode(result); err != nil {
		t.Fatal(err)
	}
	got := bytes.Replace(line.Bytes(), []byte(`"goVersion":"`+runtime.Version()+`"`), []byte(`"goVersion":"go1.27.2"`), 1)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("result bytes differ from %s:\ngot  %s\nwant %s", path, got, want)
	}
}
