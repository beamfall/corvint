package verify

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Native parity over the structural corpus (decision 0551, CEM-V1-050): the
// portable consumer (interop/cem01-go/structural_corpus_test.go) records each
// perturbation pair as segments of its base file with its four-predicate
// verdicts in both directions; this consumer rebuilds every image and must
// reach the same verdicts. A "*.crlf.go.txt" case is observed with every LF
// widened to CRLF, as the portable consumer reads it.

var corpusReasons = []string{"formatter-only", "import-reorder", "move", "rename"}

type corpusLine struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
	Pairs  []struct {
		Kind     string `json:"kind"`
		Segments []struct {
			Start int     `json:"s"`
			End   int     `json:"e"`
			Text  *string `json:"t"`
		} `json:"segments"`
		Sha256   string `json:"sha256"`
		Verdicts string `json:"verdicts"`
	} `json:"pairs"`
}

// TestStructuralCorpusParity runs over the extract CEM_STRUCTURAL_CORPUS and
// the portable observations CEM_STRUCTURAL_CORPUS_OBSERVATIONS when both are
// set.
func TestStructuralCorpusParity(t *testing.T) {
	dir, observations := os.Getenv("CEM_STRUCTURAL_CORPUS"), os.Getenv("CEM_STRUCTURAL_CORPUS_OBSERVATIONS")
	if dir == "" || observations == "" {
		t.Skip("CEM_STRUCTURAL_CORPUS and CEM_STRUCTURAL_CORPUS_OBSERVATIONS name the corpus extract and the portable observations")
	}
	files, pairs := checkCorpusParity(t, dir, observations)
	if files == 0 || pairs == 0 {
		t.Fatalf("%s: %d files, %d pairs; an empty observation set proves nothing", observations, files, pairs)
	}
	t.Logf("%s %s/%s: %d files, %d pairs, native verdicts equal portable", runtime.Version(), runtime.GOOS, runtime.GOARCH, files, pairs)
}

// emptyCases are the zero-byte cases. CEM 1.0 cannot bind an empty-file
// creation (CEM-V1-300), so they are declared here instead of committed.
var emptyCases = []string{"empty-file.go.txt"}

// TestStructuralCorpusCasesParity checks the committed version-sensitive
// cases against their qualified portable observations on every admitted
// structural tuple.
func TestStructuralCorpusCasesParity(t *testing.T) {
	if !structuralRuntimeAdmitted(runtime.Version()) {
		t.Skipf("%s is not an admitted structural tuple", runtime.Version())
	}
	root := filepath.Join("..", "..", "..", "protocol", "cem-1.0", "structural-corpus")
	files, pairs := checkCorpusParity(t, filepath.Join(root, "cases"), filepath.Join(root, "CASES.observations.jsonl"), emptyCases...)
	if files == 0 || pairs == 0 {
		t.Fatalf("cases parity observed %d files and %d pairs", files, pairs)
	}
}

// TestStructuralTuplesMatchQualification: the native verifier admits exactly
// the tuples the structural-corpus qualification admits (CEM-V1-370).
func TestStructuralTuplesMatchQualification(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "cem-1.0", "structural-corpus", "QUALIFICATION.json"))
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		AdmittedTuples []string `json:"admittedTuples"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tuple := range q.AdmittedTuples {
		if !structuralTuples[tuple] || seen[tuple] {
			t.Fatalf("QUALIFICATION.json admittedTuples %v differ from structuralTuples %v", q.AdmittedTuples, structuralTuples)
		}
		seen[tuple] = true
	}
	if len(seen) != len(structuralTuples) {
		t.Fatalf("QUALIFICATION.json admittedTuples %v differ from structuralTuples %v", q.AdmittedTuples, structuralTuples)
	}
}

func checkCorpusParity(t *testing.T, dir, observations string, empty ...string) (files, pairs int) {
	t.Helper()
	f, err := os.Open(observations)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 1<<20), 1<<26)
	mismatches := 0
	for scan.Scan() {
		var line corpusLine
		if err := json.Unmarshal(scan.Bytes(), &line); err != nil {
			t.Fatal(err)
		}
		src := []byte{}
		if !slices.Contains(empty, line.Path) {
			if src, err = os.ReadFile(filepath.Join(dir, filepath.FromSlash(line.Path))); err != nil {
				t.Fatal(err)
			}
		}
		if strings.HasSuffix(line.Path, ".crlf.go.txt") {
			src = bytes.ReplaceAll(src, []byte("\n"), []byte("\r\n"))
		}
		if corpusSha(src) != line.Sha256 {
			t.Fatalf("%s: extract digest %s, observed %s", line.Path, corpusSha(src), line.Sha256)
		}
		files++
		for _, pair := range line.Pairs {
			var image bytes.Buffer
			for _, s := range pair.Segments {
				if s.Text != nil {
					image.WriteString(*s.Text)
				} else {
					image.Write(src[s.Start:s.End])
				}
			}
			if corpusSha(image.Bytes()) != pair.Sha256 {
				t.Fatalf("%s %s: rebuilt image digest %s, observed %s", line.Path, pair.Kind, corpusSha(image.Bytes()), pair.Sha256)
			}
			var verdicts []byte
			for _, reason := range corpusReasons {
				verdicts = append(verdicts, corpusBit(structuralPredicates[reason](src, image.Bytes())), corpusBit(structuralPredicates[reason](image.Bytes(), src)))
			}
			pairs++
			if string(verdicts) != pair.Verdicts {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s: native verdicts %s, portable %s", line.Path, pair.Kind, verdicts, pair.Verdicts)
				}
			}
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if mismatches > 0 {
		t.Fatal(fmt.Sprintf("%d of %d pairs disagree", mismatches, pairs))
	}
	return files, pairs
}

func corpusSha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func corpusBit(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}
