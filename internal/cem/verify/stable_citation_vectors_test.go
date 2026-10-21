package verify

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
// cover (CEM-V1-111); the portable consumer checks the same list.
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

type citationVectors struct {
	Profile        string               `json:"profile"`
	ArtifactRoot   string               `json:"artifactRoot"`
	BaseRevision   string               `json:"baseRevision"`
	TargetRevision string               `json:"targetRevision"`
	ResultMembers  []string             `json:"resultMembers"`
	Cases          []citationVectorCase `json:"cases"`
}

// TestCEMV1111CitationVectorsNative runs the packet citation vectors through
// the native verifier, whose output they were generated from, and asserts
// each pinned verdict: exit, outcome, stage, closed code, issue codes, drift
// statuses, the verdictSha256 digest, the top-level result member order and,
// for the ACCEPT, the exact bytes of the citation block (CEM-V1-108..112).
// interop/cem01-go asserts the same file.
func TestCEMV1111CitationVectorsNative(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	top := repositoryRoot(t)
	checkStableCaseManifest(t, top, "citation-vectors/")
	c := &neutralCase{root: filepath.Join(top, "protocol/cem-1.0/stable/citation-vectors")}
	data, err := os.ReadFile(filepath.Join(c.root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want citationVectors
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-citation-vectors/1" || want.ArtifactRoot != "." {
		t.Fatalf("EXPECTED.json profile %q artifactRoot %q", want.Profile, want.ArtifactRoot)
	}
	checkCitationVectorCoverage(t, c.root, want)
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: filepath.Join(c.root, want.ArtifactRoot)}
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			result, exit := Stable(context.Background(), raw, options)
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			checkResultBytes(t, filepath.Join(c.root, "results", tc.Name+".json"), result)
			if members := topLevelMembers(t, encoded); !reflect.DeepEqual(members, want.ResultMembers) {
				t.Fatalf("result members %v, want %v", members, want.ResultMembers)
			}
			var got struct {
				VerdictSha256 string          `json:"verdictSha256"`
				Citation      json.RawMessage `json:"citation"`
			}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			drift := make([][2]string, 0, len(result.Drift))
			for _, d := range result.Drift {
				drift = append(drift, [2]string{d.Path, string(d.Status)})
			}
			wantCitation := "null"
			if tc.Citation != nil {
				wantCitation = *tc.Citation
			}
			if exit != tc.Exit || result.Outcome != tc.Outcome || result.Stage != tc.Stage ||
				!reflect.DeepEqual(result.Code, tc.Code) || !reflect.DeepEqual(append([]string{}, result.IssueCodes...), append([]string{}, tc.IssueCodes...)) ||
				!reflect.DeepEqual(drift, append([][2]string{}, tc.Drift...)) || got.VerdictSha256 != tc.VerdictSha256 ||
				!bytes.Equal(got.Citation, []byte(wantCitation)) {
				t.Fatalf("Stable exit=%d citation=%s result=%s, want the pinned %s verdict", exit, got.Citation, stableResultDebug(result), tc.Name)
			}
		})
	}
}

// checkCitationVectorCoverage holds the vectors to what they exist to pin
// (CEM-V1-111): every class, and a clean ACCEPT with anchored hunks, nonzero
// specificity, same-path citations and a whole-file deletion hunk anchored
// through its base path.
func checkCitationVectorCoverage(t *testing.T, root string, want citationVectors) {
	t.Helper()
	covered := map[string]bool{}
	for _, tc := range want.Cases {
		covered[tc.Class] = true
		if tc.Class != "control" {
			continue
		}
		var citation Citation
		if tc.Citation == nil || json.Unmarshal([]byte(*tc.Citation), &citation) != nil {
			t.Fatalf("control case %s has no citation block", tc.Name)
		}
		if citation.HunksByClass.Anchored == 0 || citation.Specificity == 0 || citation.SamePathCitations == 0 {
			t.Fatalf("control citation %+v lacks anchored hunks, nonzero specificity or same-path citations", citation)
		}
		if !citationVectorDeletionAnchored(t, filepath.Join(root, filepath.FromSlash(tc.Map)), *tc.Citation) {
			t.Fatalf("control case %s has no whole-file deletion hunk anchored through its path", tc.Name)
		}
	}
	for _, class := range citationVectorClasses {
		if !covered[class] {
			t.Errorf("citation vectors have no %s case", class)
		}
	}
}

// citationVectorDeletionAnchored reports whether the map at path has a
// whole-file deletion hunk (new range 0,0) whose citation row is anchored with
// a path anchor.
func citationVectorDeletionAnchored(t *testing.T, path, citation string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
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
	var block struct {
		Hunks []struct {
			HunkID  string   `json:"hunkId"`
			Tier    string   `json:"tier"`
			Anchors []string `json:"anchors"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(raw, &stable); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(citation), &block); err != nil {
		t.Fatal(err)
	}
	deleted := map[string]bool{}
	for _, h := range stable.Hunks {
		deleted[h.ID] = h.NewRange.Start == 0 && h.NewRange.Count == 0
	}
	for _, row := range block.Hunks {
		if deleted[row.HunkID] && row.Tier == "anchored" && slices.Contains(row.Anchors, "path") {
			return true
		}
	}
	return false
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

func topLevelMembers(t *testing.T, encoded []byte) []string {
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
