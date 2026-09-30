package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func bundleFixture(t *testing.T) (string, string, string, handoffBundleDocument) {
	t.Helper()
	root, key := handoffRepository(t)
	state := handoffTaskState{TaskID: "TASK-1", Scope: []string{"main.go"}, Decisions: []string{"Keep exact candidate"}, Feedback: []string{"Boundary test remains owed"}, NextActions: []string{"touch SHOULD_NOT_EXIST"}, Unknowns: []string{"Cross-host behavior NOT_OBSERVED"}}
	raw, _ := json.Marshal(state)
	file := writeHandoffFile(t, raw)
	code, out, errout := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--anchors", "LCP-TEST-001", "--task-state", file)
	if code != 0 {
		t.Fatalf("export: %d %s", code, errout)
	}
	var document handoffBundleDocument
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatal(err)
	}
	return root, key, writeHandoffFile(t, []byte(out)), document
}
func consumeBundle(t *testing.T, root, key, file string) (int, map[string]any, string) {
	t.Helper()
	code, out, errout := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--bundle", file)
	result := map[string]any{}
	if out != "" {
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
	}
	return code, result, errout
}
func writeBundle(t *testing.T, b handoffBundle) string {
	t.Helper()
	d, err := frameHandoffBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = emit(&out, d); err != nil {
		t.Fatal(err)
	}
	return writeHandoffFile(t, out.Bytes())
}
func TestDogfoodHandoffBundleRoundTrip(t *testing.T) {
	root, key, file, doc := bundleFixture(t)
	before := dogfoodPrivateFiles(t, root)
	code, result, errout := consumeBundle(t, root, key, file)
	if code != 0 || result["state"] != "reresolved" || result["taskStateAuthority"] != "caller-reported-unverified" {
		t.Fatalf("%d %#v %s", code, result, errout)
	}
	packet, err := base64.StdEncoding.DecodeString(result["packetBase64"].(string))
	if err != nil || dogfoodSHA(packet) != doc.Bundle.Receipt.Packet.SHA256 {
		t.Fatal("packet identity lost")
	}
	state, _ := json.Marshal(result["taskState"])
	want, _ := json.Marshal(doc.Bundle.State)
	var got handoffTaskState
	json.Unmarshal(state, &got)
	if !reflect.DeepEqual(got, doc.Bundle.State) {
		t.Fatalf("state lost %s %s", state, want)
	}
	if !reflect.DeepEqual(before, dogfoodPrivateFiles(t, root)) {
		t.Fatal("private state changed")
	}
	if _, err = os.Stat(filepath.Join(root, "SHOULD_NOT_EXIST")); !os.IsNotExist(err) {
		t.Fatal("next action executed")
	}
	if code, _, _ := consumeBundle(t, root, localcompletion.HashSession("other"), file); code != 2 {
		t.Fatal("foreign key accepted")
	}
}
func TestDogfoodHandoffBundleDriftWithholdsState(t *testing.T) {
	root, key, _, doc := bundleFixture(t)
	for _, field := range []string{"root", "revision", "enrollment", "anchor", "packet"} {
		t.Run(field, func(t *testing.T) {
			b := doc.Bundle
			b.Receipt.Anchors = append([]handoffAnchor{}, b.Receipt.Anchors...)
			switch field {
			case "root":
				b.Receipt.Root = filepath.Join(root, "other")
			case "revision":
				b.Receipt.Revision.Commit = strings.Repeat("a", 40)
			case "enrollment":
				b.Receipt.Enrollment.PlanDigest = strings.Repeat("a", 64)
			case "anchor":
				b.Receipt.Anchors[0].SHA256 = strings.Repeat("a", 64)
			case "packet":
				b.Receipt.Packet.SHA256 = strings.Repeat("a", 64)
			}
			code, result, errout := consumeBundle(t, root, key, writeBundle(t, b))
			if code != 1 || result["state"] != "drifted" {
				t.Fatalf("%d %#v %s", code, result, errout)
			}
			for _, key := range []string{"taskState", "packetBase64", "envelope"} {
				if _, ok := result[key]; ok {
					t.Fatalf("drift leaked %s", key)
				}
			}
		})
	}
	gitOutput(t, root, "commit", "--allow-empty", "-qm", "new candidate")
	if code, result, _ := consumeBundle(t, root, key, writeBundle(t, doc.Bundle)); code != 1 || result["taskState"] != nil {
		t.Fatal("HEAD drift accepted")
	}
	// Actual enrollment drift, rather than only a forged document.
	if _, err := localcompletion.Cancel(context.Background(), root, key); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := consumeBundle(t, root, key, writeBundle(t, doc.Bundle)); code != 1 || result["taskState"] != nil {
		t.Fatal("enrollment drift accepted")
	}
}
func TestDogfoodHandoffBundleMalformed(t *testing.T) {
	root, key, file, doc := bundleFixture(t)
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{"trailing": append(append([]byte{}, raw...), []byte("{}")...), "unknown": bytes.Replace(raw, []byte(`"mutates":false`), []byte(`"unknown":false,"mutates":false`), 1), "duplicate": bytes.Replace(raw, []byte(`"mutates":false`), []byte(`"mutates":false,"mutates":false`), 1), "oversize": make([]byte, localcompletion.MaxPlanBytes+1)} {
		t.Run(name, func(t *testing.T) {
			if code, _, _ := consumeBundle(t, root, key, writeHandoffFile(t, b)); code != 2 {
				t.Fatal("malformed accepted")
			}
		})
	}
	for _, field := range []string{"digest", "envelope"} {
		bad := doc
		if field == "digest" {
			bad.SHA256 = strings.Repeat("a", 64)
		} else {
			bad.Envelope += "tampered"
		}
		var encoded bytes.Buffer
		if err := emit(&encoded, bad); err != nil {
			t.Fatal(err)
		}
		if code, _, _ := consumeBundle(t, root, key, writeHandoffFile(t, encoded.Bytes())); code != 2 {
			t.Fatal("integrity mismatch accepted")
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err = os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := consumeBundle(t, root, key, link); code != 2 {
		t.Fatal("symlink accepted")
	}
	doc.Bundle.State.Scope = []string{"../outside"}
	if code, _, _ := consumeBundle(t, root, key, writeBundle(t, doc.Bundle)); code != 2 {
		t.Fatal("invalid scope accepted")
	}
	if code, _, _ := runCLI(t, "--root", root, "dogfood", "handoff", "--bundle", file); code != 2 {
		t.Fatal("implicit session accepted")
	}
}
func TestDogfoodHandoffBundleTaskStateAndDirtyRefusal(t *testing.T) {
	root, key, file, doc := bundleFixture(t)
	raw, _ := json.Marshal(doc.Bundle.State)
	for _, b := range [][]byte{[]byte(`{}`), bytes.Replace(raw, []byte(`"taskId":"TASK-1"`), []byte(`"taskId":"TASK-1","taskId":"TASK-2"`), 1), bytes.Replace(raw, []byte(`"taskId"`), []byte(`"TaskId"`), 1)} {
		if code, _, _ := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--task-state", writeHandoffFile(t, b)); code != 2 {
			t.Fatal("invalid state accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "dirty.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := consumeBundle(t, root, key, file); code != 1 || result["taskState"] != nil {
		t.Fatal("dirty import accepted")
	}
	if code, _, errout := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--task-state", writeHandoffFile(t, raw)); code != 2 || !strings.Contains(errout, "handoff-bundle-clean-candidate-required") {
		t.Fatalf("dirty export: %d %s", code, errout)
	}
}

func TestDogfoodHandoffBundleOutputBound(t *testing.T) {
	root, key, _, doc := bundleFixture(t)
	doc.Bundle.State.Decisions = make([]string, 20)
	for i := range doc.Bundle.State.Decisions {
		doc.Bundle.State.Decisions[i] = strings.Repeat("a", 2048)
	}
	// Valid bounded state can exceed the bundle limit after the envelope duplicates it.
	raw, _ := json.Marshal(doc.Bundle.State)
	code, out, errout := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--task-state", writeHandoffFile(t, raw))
	if code != 2 || out != "" || !strings.Contains(errout, "handoff-bundle-too-large") {
		t.Fatalf("oversize output %d %s %s", code, out, errout)
	}
}
