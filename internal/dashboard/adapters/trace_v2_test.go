package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/model"
	"github.com/Beamfall/corvint/internal/trace"
)

// LOD-V0-035: one physical mixed member, one cohort, no byte/row duplication.
func TestLODV0035MixedTraceSnapshot(t *testing.T) {
	t.Run("LOD-V0-035 mixed physical trace snapshot", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".context-corvint", "traces")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		v1 := traceFixture(t, "legacy task", "passed", []string{"internal/a.go"}, nil, nil)
		record, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: testRevision, Task: "typed task", OpenedPaths: []string{"internal/a.go"}, Outcome: "passed", VerificationArgv: [][]string{{"printf", "a b", "", `{"key":1}`, "|"}}}, []string{"internal/a.go"})
		if err != nil {
			t.Fatal(err)
		}
		v2, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(dir, testRevision+".jsonl")
		if err := os.WriteFile(name, v1, 0600); err != nil {
			t.Fatal(err)
		}
		before, err := Scan(context.Background(), scanRequest(root, qualifiedAuthority()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(before, []byte("local-trace-v2")) {
			t.Fatal("v1 snapshot gained v2 registry")
		}
		if record.SchemaVersion != trace.SchemaVersionV3 {
			t.Fatalf("new typed row is schema %d", record.SchemaVersion)
		}
		mixed := append(append(append([]byte{}, v1...), legacyTypedRow(t, testRevision, "legacy typed task")...), v2...)
		if err := os.WriteFile(name, mixed, 0600); err != nil {
			t.Fatal(err)
		}
		raw, err := Scan(context.Background(), scanRequest(root, qualifiedAuthority()))
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := model.VerifyCanonical(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Sources) != 1 || len(snapshot.Cohorts) != 1 {
			t.Fatalf("sources=%d cohorts=%d", len(snapshot.Sources), len(snapshot.Cohorts))
		}
		source := snapshot.Sources[0]
		if source.AdapterID != "local-trace-v2" || source.Profile != "corvint-local-trace/2" || source.ByteCount == nil || *source.ByteCount != strconv.Itoa(len(mixed)) || source.Members == nil || len(*source.Members) != 1 {
			t.Fatalf("mixed source %+v", source)
		}
		member := (*source.Members)[0]
		if member.ContentSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(mixed)) {
			t.Fatal("physical member digest changed")
		}
		summary, _, code, _ := preflightTraceArtifact(mixed, testRevision)
		if code != "" || summary.RetainedRows != 3 || !summary.hasV2 {
			t.Fatalf("rows: %+v %s", summary, code)
		}
		if strings.Contains(string(raw), "typed task") || strings.Contains(string(raw), "printf") || strings.Contains(string(raw), `"producer":`) || strings.Contains(string(raw), `"cli"`) {
			t.Fatal("snapshot leaked retained contents")
		}
		if err := os.WriteFile(name, v1, 0600); err != nil {
			t.Fatal(err)
		}
		after, err := Scan(context.Background(), scanRequest(root, qualifiedAuthority()))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("v1 snapshot bytes changed: %v", err)
		}

	})
}

// legacyTypedRow spells a schema-2 row as writers before schema 3 stored it:
// typed verification, no producer, identity over the basis without trace_id.
func legacyTypedRow(t *testing.T, revision, task string) []byte {
	t.Helper()
	prefix := `{"changed_paths":[],"opened_paths":["internal/a.go"],"outcome":"passed","revision":"` + revision + `","schema_version":2,"task":"` + task + `"`
	verification := `"verification":[{"argv":["go","test"],"kind":"argv"}]}`
	id := sha256.Sum256([]byte(prefix + "," + verification))
	return []byte(fmt.Sprintf("%s,\"trace_id\":\"%x\",%s\n", prefix, id, verification))
}

func FuzzTypedRowsAgreeWithWriter(f *testing.F) {
	f.Add(`["go","test","a b"]`)
	f.Add(`["x","",";","é"]`)
	f.Fuzz(func(t *testing.T, raw string) {
		argv, err := trace.ParseVerificationArgv([]byte(raw))
		if err != nil {
			return
		}
		record, err := trace.NewRecord(trace.Input{Producer: trace.ProducerCLI, Revision: testRevision, Task: "fuzz task", Outcome: "passed", VerificationArgv: [][]string{argv}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		row, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		_, code := parseTraceRow(bytes.TrimSpace(row), testRevision)
		if code != "" {
			t.Fatalf("writer/reader disagreement: %s", code)
		}
	})
}
