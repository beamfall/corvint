// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testevidence"
)

func evidenceFixture() testevidence.Record {
	r := testevidence.New("repo", "p/TestA", strings.Repeat("a", 40))
	r.Runner = testevidence.RunnerIdentity{Plugin: "plugin", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "1", ConfigurationDigest: strings.Repeat("c", 64)}
	r.Execution = testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "trace", TracingVersion: "1", EnvironmentDigest: strings.Repeat("d", 64)}
	r.CollectionCount = 1
	r.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("e", 64), VerifierRevision: strings.Repeat("f", 40), ConfigurationDigest: strings.Repeat("c", 64), Ref: "refs/heads/main", Run: "run-1", ContentDigest: strings.Repeat("1", 64)}}
	return r
}

// RSP-V0-005, RSP-V0-008: old gob schemas (Go and JS, /2 and /3) have executed
// evidence only. No new collector, target, provenance or trace identity is invented.
func TestRSPV0008LegacyIndexes(t *testing.T) {
	type oldIndex struct {
		Format, Language, Module, Commit, Runner string
		Tests                                    []Test
		Files                                    []File
	}
	for _, language := range []string{"go", "javascript"} {
		for _, format := range []string{FormatV2, Format} {
			t.Run(language+format, func(t *testing.T) {
				old := oldIndex{format, language, "module", strings.Repeat("a", 40), "runner", []Test{{Package: "p", Name: "TestA"}, {Package: "p", Name: "TestB"}}, []File{{Path: "src", OID: strings.Repeat("2", 40), Blocks: []Block{{Start: 1, StartCol: 1, End: 2, EndCol: 2, Tests: []int32{0}}, {Start: 3, StartCol: 1, End: 3, EndCol: 2, Tests: []int32{1}}}}}}
				var data bytes.Buffer
				if err := gob.NewEncoder(&data).Encode(old); err != nil {
					t.Fatal(err)
				}
				ix, err := Decode(&data)
				if err != nil {
					t.Fatal(err)
				}
				records, err := ix.EvidenceRecords("repo")
				if err != nil || len(records) != 2 {
					t.Fatalf("legacy: %v", err)
				}
				for _, r := range records {
					if r.Executed.State != testevidence.Present || len(r.Executed.Items) != 1 || len(r.Executed.Items[0].Executed) != 1 || len(r.Executed.Items[0].Unexecuted) != 1 {
						t.Fatalf("coverage lost: %+v", r.Executed)
					}
					if r.Regions.State != testevidence.Absent || r.Loaded.State != testevidence.Absent || r.Reads.State != testevidence.Absent || r.Listings.State != testevidence.Absent || r.Lookups.State != testevidence.Absent || r.Writes.State != testevidence.Absent || r.Execs.State != testevidence.Absent || r.External.State != testevidence.Absent {
						t.Fatal("invented legacy component")
					}
					if r.Validate() == nil {
						t.Fatal("invented legacy identity")
					}
				}
				ix.Tests[0].Incomplete = true
				records, err = ix.EvidenceRecords("repo")
				if err != nil || records[0].Executed.State != testevidence.Incomplete {
					t.Fatal("legacy incomplete hidden", err)
				}
				ix.Files[0].OID = "bad"
				if _, err := ix.EvidenceRecords("repo"); err == nil {
					t.Fatal("malformed legacy blob admitted")
				}
			})
		}
	}
}

// RSP-V0-008: real Go profiles contain zero-width blocks for empty bodies and
// clauses. The legacy adapter retains their file but abstains on execution.
func TestRSPV0008NativeZeroWidth(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"go.mod": "module example.com/empty\n\ngo 1.22\n",
		"empty.go": `package empty
func Empty() {}
func Uncalled() {}
func Clauses(n int) {
	switch n {
	case 0:
	default:
	}
	select {
	default:
	}
}
`,
		"empty_test.go": `package empty
import "testing"
func TestEmpty(t *testing.T) { Empty(); Clauses(0) }
`,
	})
	profilePath := filepath.Join(dir, "coverage.out")
	cmd := exec.CommandContext(t.Context(), "go", "test", "-count=1", "-timeout", "30m", "-coverprofile="+profilePath, ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Build("example.com/empty", strings.Repeat("a", 40), "go test", map[string]string{"empty.go": strings.Repeat("2", 40)}, []Record{{Package: ".", Name: "TestEmpty", Profile: profile}})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := ix.Encode(&data); err != nil {
		t.Fatal(err)
	}
	ix, err = Decode(&data)
	if err != nil {
		t.Fatal(err)
	}
	zeroHit, zeroMiss, nonzero := 0, 0, 0
	for _, b := range ix.Files[0].Blocks {
		if b.Start == b.End && b.StartCol == b.EndCol {
			if len(b.Tests) > 0 {
				zeroHit++
			} else {
				zeroMiss++
			}
		} else {
			nonzero++
		}
	}
	if zeroHit == 0 || zeroMiss == 0 || nonzero == 0 {
		t.Fatalf("fixture missing coverage forms: %s", profile)
	}
	for _, language := range []string{"", "go"} {
		ix.Language = language
		records, err := ix.EvidenceRecords("repo")
		if err != nil || len(records) != 1 {
			t.Fatalf("native profile rejected: %v", err)
		}
		r := records[0]
		if r.Executed.State != testevidence.Incomplete || r.Executed.Reason != "legacy-record-incomplete" || len(r.Executed.Items) != 1 {
			t.Fatalf("empty coverage treated as complete: %+v", r.Executed)
		}
		f := r.Executed.Items[0]
		if f.Blob.OID != ix.Files[0].OID || f.Blob.Path.Resolved != "empty.go" || len(f.Executed)+len(f.Unexecuted) != nonzero {
			t.Fatalf("file or nonempty coverage lost: %+v", f)
		}
		var executed, unexecuted []testevidence.Range
		for _, b := range ix.Files[0].Blocks {
			if b.Start == b.End && b.StartCol == b.EndCol {
				continue
			}
			span := testevidence.Range{Start: b.Start, StartColumn: b.StartCol, End: b.End, EndColumn: b.EndCol}
			if len(b.Tests) > 0 {
				executed = append(executed, span)
			} else {
				unexecuted = append(unexecuted, span)
			}
		}
		if !reflect.DeepEqual(f.Executed, executed) || !reflect.DeepEqual(f.Unexecuted, unexecuted) {
			t.Fatalf("nonempty block classification changed: %+v", f)
		}
		if r.Reads.State != testevidence.Absent || r.ValidateComponents() != nil {
			t.Fatal("invalid legacy adaptation")
		}
	}
	// The normalization allowlist is native Go, positive coordinates and equal
	// endpoints. An unknown language or malformed shape must still fail closed.
	ix.Language = "future-language"
	if _, err := ix.EvidenceRecords("repo"); err == nil {
		t.Fatal("unknown zero-width semantics admitted")
	}
	ix.Language = "go"
	for _, b := range []Block{{Start: 1, End: 1}, {Start: 1, StartCol: 3, End: 1, EndCol: 2}} {
		ix.Files[0].Blocks = []Block{b}
		if _, err := ix.EvidenceRecords("repo"); err == nil {
			t.Fatalf("malformed shape normalized: %+v", b)
		}
	}
}

// RSP-V0-008: the extension survives encode/decode and sharded merge; malformed
// records are refused by both writers and readers, never silently dropped.
func TestRSPV0008Extension(t *testing.T) {
	r := evidenceFixture()
	ix := &Index{Format: Format, Module: "module", Commit: r.Commit, Runner: "runner", Evidence: []testevidence.Record{r}}
	var data bytes.Buffer
	if err := ix.Encode(&data); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(&data)
	if err != nil || len(got.Evidence) != 1 {
		t.Fatal(err)
	}
	records, err := got.EvidenceRecords("repo")
	if err != nil {
		t.Fatal(err)
	}
	records[0].Sources[0].Run = "changed"
	if got.Evidence[0].Sources[0].Run == "changed" {
		t.Fatal("adapter mutated index")
	}
	second := *ix
	second.Evidence = []testevidence.Record{r}
	second.Evidence[0].Test = "p/TestB"
	merged, err := Merge(ix, &second)
	if err != nil || len(merged.Evidence) != 2 {
		t.Fatal("merge lost extension", err)
	}
	// A partial extension must not hide tests still represented only by linecov.
	got.Tests = []Test{{Package: "p", Name: "TestB"}}
	records, err = got.EvidenceRecords("repo")
	if err != nil || len(records) != 2 {
		t.Fatal("mixed legacy test lost", err)
	}
	_, err = Merge(ix, ix)
	if err == nil {
		t.Fatal("duplicate evidence target accepted")
	}
	for _, mutate := range []func(*testevidence.Record){func(r *testevidence.Record) { r.Commit = strings.Repeat("2", 40) }, func(r *testevidence.Record) { r.Reads.State = "unknown" }, func(r *testevidence.Record) { r.CollectionCount = 0 }} {
		bad := *ix
		bad.Evidence = []testevidence.Record{r}
		mutate(&bad.Evidence[0])
		data.Reset()
		if err := bad.Encode(&data); err == nil || data.Len() != 0 {
			t.Fatal("bad extension written")
		}
		if err := gob.NewEncoder(&data).Encode(&bad); err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(&data); err == nil {
			t.Fatal("bad extension read")
		}
	}
}

// RSP-V0-008: measured representative storage remains under the inherited size
// floor. Selection latency on the P3 reference host is a separate qualification.
func TestRSPV0008IndexSize(t *testing.T) {
	r := evidenceFixture()
	ix := &Index{Format: Format, Module: "module", Commit: r.Commit, Runner: "runner"}
	for i := 0; i < 1000; i++ {
		copy := r
		copy.Test = fmt.Sprintf("p/Test%d", i)
		ix.Evidence = append(ix.Evidence, copy)
	}
	var b bytes.Buffer
	start := time.Now()
	if err := ix.EncodeBounded(&b, 0); err != nil {
		t.Fatal(err)
	}
	if int64(b.Len()) >= IndexSizeLimit(0) {
		t.Fatal("index over size bound")
	}
	if _, err := Decode(bytes.NewReader(b.Bytes())); err != nil {
		t.Fatal(err)
	}
	t.Logf("records=1000 bytes=%d bound=%d encode+decode=%s", b.Len(), IndexSizeLimit(0), time.Since(start))
	if IndexSizeLimit(10_000_000_000) != 100_000_000 {
		t.Fatal("packed-size bound changed")
	}
	var out bytes.Buffer
	capWriter := indexBoundWriter{w: &out, remaining: 4}
	if _, err := capWriter.Write([]byte("12345")); err == nil || out.Len() != 0 {
		t.Fatal("cap admitted a partial write")
	}
	if err := ix.EncodeBounded(&out, -1); err == nil || out.Len() != 0 {
		t.Fatal("unknown packed size admitted")
	}
}
