package main

import (
	"bytes"
	"encoding/json"
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
				Producer     string                       `json:"producer"`
				Verification []map[string]json.RawMessage `json:"verification"`
			} `json:"trace"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Trace.Schema != 3 || payload.Trace.Producer != "cli" || len(payload.Trace.Verification) != 3 {
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
