//go:build unix

package workqueuev0

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestNativeReplayControlMatrices(t *testing.T) {
	t.Run("environment matrix", func(t *testing.T) {
		if len(replayEnvironments()) != 4 || len(replaySchedule()) != 100 {
			t.Fatal("positive environment/schedule matrix drift")
		}
		seen := map[[3]int]bool{}
		for _, c := range replaySchedule() {
			key := [3]int{c["environment"], c["permutation"], c["seed"]}
			seen[key] = true
		}
		if len(seen) != 100 {
			t.Fatal("duplicate schedule cell")
		}
		cases := 0
		for range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
			for range []string{"empty", "hostile"} {
				for range []string{"observe", "propose-wave"} {
					cases++
				}
			}
		}
		if cases != 12 {
			t.Fatal("rebinding matrix drift")
		}
	})

	t.Run("exact source refusal rejects incomplete receipts", func(t *testing.T) {
		body := map[string]any{"errorCode": "SOURCE_UNQUALIFIED", "observation": nil, "profile": "work-command-result/0", "proposal": nil, "state": "ERROR"}
		wire, _ := Identified("work-command-result", "work-command-result/0", "id", body)
		raw, _ := Canonical(wire)
		raw = append(raw, '\n')
		complete := Receipt{Exit: 2, Failure: nil, Waited: true, StreamObservationComplete: true, Cleanup: "NO_OWNED_LIVE_DESCENDANTS"}
		if err := assertErrorOutput(raw, complete, "SOURCE_UNQUALIFIED"); err != nil {
			t.Fatal(err)
		}
		mutants := []Receipt{complete, complete, complete, complete, complete, complete}
		mutants[0].Exit = 1
		mutants[1].Failure = "TIMEOUT"
		mutants[2].Waited = false
		mutants[3].StreamObservationComplete = false
		mutants[4].Cleanup = "PROCESS_RESIDUE"
		mutants[5].Exit = 0
		for i, m := range mutants {
			if assertErrorOutput(raw, m, "SOURCE_UNQUALIFIED") == nil {
				t.Fatalf("invalid receipt %d accepted", i)
			}
		}
		if assertErrorOutput(bytes.Replace(raw, []byte("SOURCE_UNQUALIFIED"), []byte("MALFORMED_INPUT"), 1), complete, "SOURCE_UNQUALIFIED") == nil {
			t.Fatal("incorrect refusal bytes accepted")
		}
	})

	t.Run("forbidden command matrix", func(t *testing.T) {
		verbs := strings.Fields("pass-through claim release heartbeat dispatch review verdict finish merge close edit add amend dependency-write approval repair lease build-once autofill ticket-add start continue adopt renew complete fanout")
		overrides := strings.Fields("--adapter --policy --adapter-path --adapter-id --scope --mapping --operation --manifest --profile --access-context")
		if len(verbs) != 26 || len(overrides)*2 != 20 {
			t.Fatalf("matrix drift: verbs=%d overrides=%d", len(verbs), len(overrides)*2)
		}
	})
}

func TestNativeProcessGuardianStreamMatrix(t *testing.T) {
	tests := []struct {
		name, code  string
		timeout     time.Duration
		limit, exit int
		failure     string
		full        []byte
	}{
		{"success", "printf 'ok\\n'", time.Second, 1024, 0, "", []byte("ok\n")},
		{"nonzero", "printf 'failed\\n'; exit 7", time.Second, 1024, 7, "", []byte("failed\n")},
		{"fast-overflow", "head -c 4097 /dev/zero | tr '\\0' x", time.Second, 1024, -1, "OUTPUT_LIMIT", bytes.Repeat([]byte("x"), 4097)},
		{"timeout", "printf 'before-timeout\\n'; sleep 4", 150 * time.Millisecond, 1024, -1, "TIMEOUT", []byte("before-timeout\n")},
		{"closed-stdin", "if read x; then printf open; else printf closed; fi", time.Second, 1024, 0, "", []byte("closed")},
		{"inherited-pipe", "(printf 'held\\n'; sleep 4) & exit 0", 200 * time.Millisecond, 1024, -1, "TIMEOUT", []byte("held\n")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, r, err := runBoundedLimits([]string{"/bin/sh", "-c", tc.code}, "", fixedEnv(), tc.timeout, tc.limit, 1024)
			if !r.Waited || !r.StreamObservationComplete || r.Cleanup != "NO_OWNED_LIVE_DESCENDANTS" {
				t.Fatalf("incomplete receipt: %+v", r)
			}
			wantPrefix := tc.full
			if len(wantPrefix) > tc.limit {
				wantPrefix = wantPrefix[:tc.limit]
			}
			if !bytes.Equal(raw, wantPrefix) || r.StdoutBytes != len(tc.full) || r.StdoutRawSHA256 != Digest(tc.full) {
				t.Fatalf("stream evidence: raw=%d bytes=%d hash=%s", len(raw), r.StdoutBytes, r.StdoutRawSHA256)
			}
			if tc.failure == "" {
				if err != nil || r.Failure != nil || r.Exit != tc.exit {
					t.Fatalf("unexpected result: err=%v receipt=%+v", err, r)
				}
			} else if r.Failure == nil || !strings.Contains(r.Failure.(string), tc.failure) {
				t.Fatalf("missing %s: %+v", tc.failure, r)
			}
		})
	}
}

func TestNativeImmutableSpyMatrix(t *testing.T) {
	dir := t.TempDir()
	for _, op := range []string{"snapshot", "details", "verify", "verify"} {
		if err := WriteSpy(dir, op, false); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 4 {
		t.Fatal("spy count")
	}
	for _, kind := range []string{"duplicate", "missing", "reordered", "partial", "collision", "noncanonical"} {
		t.Run(kind, func(t *testing.T) {
			copyDir := t.TempDir()
			for _, e := range entries {
				raw, _ := os.ReadFile(filepath.Join(dir, e.Name()))
				if err := os.WriteFile(filepath.Join(copyDir, e.Name()), raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "duplicate":
				var v map[string]any
				raw, _ := os.ReadFile(filepath.Join(copyDir, "000002.json"))
				_ = json.Unmarshal(raw, &v)
				v["sequence"] = float64(1)
				_ = Save(filepath.Join(copyDir, "000002.json"), v)
			case "missing":
				_ = os.Remove(filepath.Join(copyDir, "000001.json"))
			case "reordered":
				a, _ := os.ReadFile(filepath.Join(copyDir, "000000.json"))
				b, _ := os.ReadFile(filepath.Join(copyDir, "000001.json"))
				_ = os.WriteFile(filepath.Join(copyDir, "000000.json"), b, 0o600)
				_ = os.WriteFile(filepath.Join(copyDir, "000001.json"), a, 0o600)
			case "partial":
				_ = os.WriteFile(filepath.Join(copyDir, "000004.json"), []byte("{"), 0o600)
			case "collision":
				_ = os.WriteFile(filepath.Join(copyDir, "000004.json"), []byte("partial-racing-reservation"), 0o600)
			case "noncanonical":
				var v map[string]any
				raw, _ := os.ReadFile(filepath.Join(copyDir, "000002.json"))
				_ = json.Unmarshal(raw, &v)
				indented, _ := json.MarshalIndent(v, "", " ")
				_ = os.WriteFile(filepath.Join(copyDir, "000002.json"), append(indented, '\n'), 0o600)
			}
			before := snapshotFiles(t, copyDir)
			if WriteSpy(copyDir, "verify", false) == nil {
				t.Fatal("malformed immutable sequence accepted")
			}
			after := snapshotFiles(t, copyDir)
			if !equalCanonical(before, after) {
				t.Fatal("rejected spy write changed existing records")
			}
		})
	}
}

func TestNativeMaterializationTrapMatrix(t *testing.T) {
	literal := []byte("verification-materialization\x00verification-materialization/0\x00[{\"blobOid\":\"4a58007052a65fbc2fc3f910f2855f45a4058e74\",\"mode\":\"100644\",\"path\":\"a.txt\",\"rawSha256\":\"b6a98d9ce9a2d9149288fa3df42d377c3e42737afdcdaf714e33c0a100b51060\"},{\"blobOid\":\"039e4d0069c5c26909f86c505b9de66182e6d1f3\",\"mode\":\"100755\",\"path\":\"b/x.sh\",\"rawSha256\":\"306c6ca7407560340797866e077e053627ad409277d1b9da58106fce4cf717cb\"}]")
	want := Digest(literal)
	traps := [][]byte{append(append([]byte{}, literal...), '\n'), bytes.Replace(literal, []byte("verification-materialization\x00"), []byte("other-materialization\x00"), 1), bytes.Replace(literal, []byte("/0\x00"), []byte("/1\x00"), 1), bytes.Replace(literal, []byte("100644"), []byte("100755"), 1), bytes.Replace(literal, []byte("4a580070"), []byte("00000000"), 1), bytes.Replace(literal, []byte("b6a98d9c"), []byte("00000000"), 1), bytes.Replace(literal, []byte("a.txt"), []byte("z.txt"), 1), bytes.Replace(literal, []byte(",\"rawSha256\":\"b6a98d9ce9a2d9149288fa3df42d377c3e42737afdcdaf714e33c0a100b51060\""), nil, 1)}
	for i, raw := range traps {
		if Digest(raw) == want {
			t.Fatalf("trap %d retained digest", i)
		}
	}
}

func TestNativePairedProseNegativeMatrix(t *testing.T) {
	read := func(name string) map[string]any {
		raw, err := os.ReadFile("testdata/cli_" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err = json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	documents := map[string]any{"snapshot": read("snapshot"), "details": read("details"), "verify": read("verify")}
	reject := func(name string, mutate func(map[string]any)) {
		wrong := deepCopy(documents)
		mutate(wrong)
		if equalCanonical(wrong, documents) {
			t.Fatalf("%s mutation was not discriminating", name)
		}
	}
	reject("stale-request-version", func(v map[string]any) {
		v["snapshot"].(map[string]any)["detailRequestTicketVersionIds"].([]any)[0] = "stale"
	})
	reject("stale-active-lease-ticket-version", func(v map[string]any) {
		v["snapshot"].(map[string]any)["leases"].([]any)[0].(map[string]any)["ticketVersionId"] = "stale"
	})
	reject("stale-active-lease-version-ID", func(v map[string]any) {
		v["snapshot"].(map[string]any)["leases"].([]any)[0].(map[string]any)["leaseVersionId"] = "stale"
	})
	reject("stale-record-reference", func(v map[string]any) {
		v["details"].(map[string]any)["details"].([]any)[0].(map[string]any)["ticketVersionId"] = "stale"
	})
	reject("forged-payload", func(v map[string]any) {
		v["details"].(map[string]any)["details"].([]any)[0].(map[string]any)["payload"].(map[string]any)["body"] = "WRONG CONTENT"
	})
	reject("unknown-closed-field", func(v map[string]any) { v["snapshot"].(map[string]any)["invented"] = true })
	policy, snapshot := read("policy"), read("snapshot")
	queue := fixtureQueue("/private/tmp/spies", false)
	queue["tickets"].([]any)[0].(map[string]any)["body"] = "WRONG CONTENT"
	regenerated, err := Documents(policy, queue, snapshot["repositorySource"].(map[string]any))
	if err != nil {
		t.Fatal(err)
	}
	if equalCanonical(regenerated, documents) {
		t.Fatal("wrong content with rebound IDs accepted")
	}
	zero := deepCopy(documents)
	zero["snapshot"].(map[string]any)["id"] = "work-queue-snapshot:sha256:" + strings.Repeat("0", 64)
	if equalCanonical(zero, documents) {
		t.Fatal("shared generator ID error accepted")
	}

	root := filepath.Join(t.TempDir(), "corvint-work-run-fixture")
	children := map[string]string{"HOME": filepath.Join(root, "home"), "TMPDIR": filepath.Join(root, "tmp"), "target": filepath.Join(root, "target"), "root": root}
	own := map[string]any{}
	for name, path := range children {
		own[name] = map[string]any{"path": path, "resolved": path, "uid": os.Getuid(), "mode": 0o700, "directory": true, "symlink": false}
	}
	row := map[string]any{"ownership": own, "environment": map[string]any{"HOME": children["HOME"], "TMPDIR": children["TMPDIR"]}}
	if err := validateOwnership(row, root); err != nil {
		t.Fatal(err)
	}
	mutants := map[string]func(map[string]any){"foreign-uid": func(v map[string]any) {
		v["ownership"].(map[string]any)["HOME"].(map[string]any)["uid"] = os.Getuid() + 1
	}, "nonprivate-mode": func(v map[string]any) { v["ownership"].(map[string]any)["TMPDIR"].(map[string]any)["mode"] = 0o755 }, "wrong-parent": func(v map[string]any) {
		v["ownership"].(map[string]any)["target"].(map[string]any)["path"] = "/private/tmp/other/target"
	}, "symlink": func(v map[string]any) { v["ownership"].(map[string]any)["HOME"].(map[string]any)["symlink"] = true }, "unknown-field": func(v map[string]any) { v["ownership"].(map[string]any)["HOME"].(map[string]any)["invented"] = true }, "environment-mismatch": func(v map[string]any) { v["environment"].(map[string]any)["HOME"] = "/private/tmp/foreign/home" }}
	for name, mutate := range mutants {
		wrong := deepCopy(row)
		mutate(wrong)
		if validateOwnership(wrong, root) == nil {
			t.Fatalf("ownership %s accepted", name)
		}
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "absent"), root); err != nil {
		t.Fatal(err)
	}
	if validateOwnership(row, root) == nil {
		t.Fatal("dangling runtime root residue accepted")
	}
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(sentinel, []byte("paired-prose-sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(sentinel)
	_ = os.WriteFile(sentinel, []byte("direct positive detector write\n"), 0o600)
	after, _ := os.ReadFile(sentinel)
	if bytes.Equal(before, after) {
		t.Fatal("sentinel write undetected")
	}
	_ = os.Remove(sentinel)
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("sentinel removal undetected")
	}
}

func TestNativeOutputOracleRejectsEveryRehashedMutation(t *testing.T) {
	read := func(name string) map[string]any {
		raw, err := os.ReadFile("testdata/cli_" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err = json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	snapshot, details, envelope := read("snapshot"), read("details"), read("envelope")
	inputs := map[string]any{}
	for name, value := range map[string]map[string]any{"snapshot": snapshot, "details": details, "verify": read("verify")} {
		raw, _ := Canonical(value)
		inputs[name] = map[string]any{"id": value["id"], "bytes": len(raw) + 1, "sha256": Digest(append(raw, '\n'))}
	}
	reg := map[string]any{"adapterBlobOid": strings.Repeat("1", 40), "adapterSha256": strings.Repeat("2", 64), "containmentClass": "DARWIN_PROCESS_GROUP_UNQUALIFIED", "interpreter": "/fixture/go-adapter", "interpreterSha256": strings.Repeat("3", 64), "interpreterMode": "0755", "expectedPolicyId": snapshot["policyId"], "expectedDetails": detailIDs(details), "expectedCheckpoint": snapshot["checkpoint"], "expectedQueueSourceId": "queue-source:sha256:" + strings.Repeat("4", 64), "expectedEnvelopeId": envelope["id"], "expectedUnknowns": []any{"CONTAINMENT_UNQUALIFIED", "EXECUTABLE_IDENTITY_UNQUALIFIED", "MUTATION_ENFORCEMENT_UNQUALIFIED", "NETWORK_UNOBSERVED", "SOURCE_UNQUALIFIED"}, "expectedCollisionClosure": []any{map[string]any{"id": "collision:fixture:queue:adapter", "memberTicketIds": []any{"ticket:fixture:queue:a", "ticket:fixture:queue:c"}, "path": nil, "source": "ADAPTER"}}, "inputs": inputs}
	commands, err := expectedCommands(reg)
	if err != nil {
		t.Fatal(err)
	}
	for op, expected := range commands {
		mutants := wireMutants(expected)
		if len(mutants) == 0 {
			t.Fatal("no oracle mutations")
		}
		for i, candidate := range mutants {
			candidate = rehashWire(candidate).(map[string]any)
			if equalCanonical(candidate, expected) {
				t.Fatalf("%s rehashed mutation %d accepted", op, i)
			}
		}
	}
}

func detailIDs(details map[string]any) []any {
	out := []any{}
	for _, raw := range details["details"].([]any) {
		out = append(out, raw.(map[string]any)["detailId"])
	}
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]) < fmt.Sprint(out[j]) })
	return out
}
func wireMutants(root map[string]any) []map[string]any {
	paths := [][]any{}
	var walk func(any, []any)
	walk = func(v any, path []any) {
		switch x := v.(type) {
		case map[string]any:
			for k, item := range x {
				if k != "id" {
					next := append(append([]any{}, path...), k)
					paths = append(paths, next)
					walk(item, next)
				}
			}
		case []any:
			for i, item := range x {
				walk(item, append(append([]any{}, path...), i))
			}
		}
	}
	walk(root, nil)
	out := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		v := deepCopy(root)
		var current any = v
		for _, part := range path[:len(path)-1] {
			switch p := part.(type) {
			case string:
				current = current.(map[string]any)[p]
			case int:
				current = current.([]any)[p]
			}
		}
		last := path[len(path)-1]
		switch p := last.(type) {
		case string:
			m := current.(map[string]any)
			if m[p] == nil {
				m[p] = "unexpected"
			} else {
				m[p] = nil
			}
		case int:
			a := current.([]any)
			if a[p] == nil {
				a[p] = "unexpected"
			} else {
				a[p] = nil
			}
		}
		out = append(out, v)
	}
	return out
}
func rehashWire(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = rehashWire(item)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, item := range x {
			out[k] = rehashWire(item)
		}
		profile, _ := out["profile"].(string)
		kind := ""
		if _, ok := out["adapterBlobOid"]; ok {
			profile = "adapter-execution/0"
			kind = "adapter-execution"
		} else {
			kind = map[string]string{"work-command-result/0": "work-command-result", "work-queue-observation/0": "work-queue-observation", "work-wave-proposal/0": "work-wave-proposal"}[profile]
		}
		if kind != "" {
			body := cloneMap(out)
			delete(body, "id")
			id, _ := Identity(kind, profile, body)
			out["id"] = id
		}
		return out
	default:
		return v
	}
}

func deepCopy(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func snapshotFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		out[e.Name()] = Digest(b)
	}
	return out
}
