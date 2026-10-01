package main

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// LTPM-V0-013/014: public literal JSON transport and refusal-before-mutation.
func TestRecordTypedArgvRoundTripAndRefusal(t *testing.T) {
	t.Run("LTPM-V0-013 literal argv roundtrip and refusal", func(t *testing.T) {
		root := newRecordFixtureAt(t, filepath.Join(t.TempDir(), "repo"))
		sentinel := filepath.Join(t.TempDir(), "not-executed")
		argv := []string{"sh", "-c", "touch " + sentinel, "", ";", "{\"key\":1}"}
		encoded, _ := json.Marshal(argv)
		args := append(recordArguments(root, "typed CLI task", "passed"), "--verify-argv-json", string(encoded), "--verify-argv-json="+string(encoded))
		var stdout, stderr bytes.Buffer
		if exit := runContext(t.Context(), args, nil, &stdout, &stderr); exit != 0 {
			t.Fatalf("exit%d %s", exit, &stderr)
		}
		var payload struct {
			Trace struct {
				Schema       int                          `json:"schema_version"`
				Verification []map[string]json.RawMessage `json:"verification"`
			} `json:"trace"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Trace.Schema != 2 || len(payload.Trace.Verification) != 3 {
			t.Fatalf("typed response %s", &stdout)
		}
		var actual []string
		if err := json.Unmarshal(payload.Trace.Verification[0]["argv"], &actual); err != nil || !reflect.DeepEqual(actual, argv) {
			t.Fatalf("argv: %v %v", actual, err)
		}
		if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
			t.Fatal("verification executed")
		}
		before := traceTreeSnapshot(t, root)
		for _, bad := range []string{`["x",null]`, `["x","--\u0074oken","secret value"]`, `["x","` + strings.Repeat("a", 513) + `"]`} {
			stdout.Reset()
			stderr.Reset()
			args := append(recordArguments(root, "refused", "passed"), "--verify-argv-json", bad)
			if exit := runContext(t.Context(), args, nil, &stdout, &stderr); exit != 2 || !bytes.Contains(stderr.Bytes(), []byte("unsupported-verify-argv")) {
				t.Fatalf("refusal exit%d %s", exit, &stderr)
			}
			if !reflect.DeepEqual(before, traceTreeSnapshot(t, root)) {
				t.Fatal("refusal changed trace store")
			}
		}

	})
}

func TestQueryTypedArgvAdmissionAndBatchDigest(t *testing.T) {
	root := queryCLIRepository(t)
	recorded, err := tracerecordrepo.Record(t.Context(), root, tracerecordrepo.Input{Task: "token parser", ChangedPaths: []string{"internal/parser/token.go"}, Outcome: "passed", VerificationArgv: [][]string{{"printf", "a b", ""}}})
	if err != nil {
		t.Fatal(err)
	}
	before := repositoryBytesDigest(t, root)
	candidate := runRepositoryQueryProcess(t, root)
	if candidate.exit != 0 {
		t.Fatalf("query: %s", candidate.stderr)
	}
	context := decodeRepositoryQueryContext(t, candidate.stdout)
	learning := objectField(t, context, "learning")
	assertJSONNumber(t, learning, "local_trace_count", 1)
	assertJSONNumber(t, learning, "matched_local_traces", 1)
	assertLocalTraceCandidate(t, context, recorded.Record.TraceID, recorded.Record.Revision, "internal/parser/token.go", 260, "changed")
	if after := repositoryBytesDigest(t, root); after != before {
		t.Fatal("typed query mutated repository")
	}
	record := recorded.Record
	digest := batchTraceDigest([]trace.Record{record}, "ready")
	record.TypedVerification[0].Argv[1] = "changed literal"
	if batchTraceDigest([]trace.Record{record}, "ready") == digest {
		t.Fatal("batch digest omitted typed verification")
	}
}
