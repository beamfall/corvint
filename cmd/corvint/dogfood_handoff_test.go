package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// handoffRepository is an enrolled fixture whose requirement and governance
// anchors resolve to task evidence.
func handoffRepository(t *testing.T) (string, string) {
	t.Helper()
	root := queryCLIRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "specs", "event.md"), []byte("# Event specification\n\n- `LCP-TEST-001`: Preserve an explicit requirement anchor.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "docs/specs/event.md")
	gitOutput(t, root, "commit", "-qm", "add requirement fixture")
	key := localcompletion.HashSession("handoff-session")
	base := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	plan, err := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"docs/specs/event.md"}, Checks: []localcompletion.Check{{ID: "check", Argv: []string{"false"}, TimeoutSeconds: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localcompletion.Begin(context.Background(), root, key, plan); err != nil {
		t.Fatal(err)
	}
	return root, key
}

// emitHandoff runs the emitting subverb and stores its stdout outside the root.
func emitHandoff(t *testing.T, root, key string) (string, handoffDocument) {
	t.Helper()
	code, stdout, stderr := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--anchors", "LCP-TEST-001  AGENTS.md LCP-TEST-001")
	if code != 0 {
		t.Fatalf("emit exit %d: %s", code, stderr)
	}
	var document handoffDocument
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(file, []byte(stdout), 0600); err != nil {
		t.Fatal(err)
	}
	return file, document
}

func consumeHandoff(t *testing.T, root, key, file string) (int, map[string]any, string) {
	t.Helper()
	code, stdout, stderr := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--receipt", file)
	var result map[string]any
	if code != 2 {
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("consume stdout: %v %s", err, stdout)
		}
	}
	return code, result, stderr
}

// TestDogfoodHandoffReceiptReresolvesSamePacket pins SESSION-V0-017 and
// SESSION-V0-018: the receipt carries identity, anchors, packet digest and
// degradations as untrusted data, and a second read on the same revision
// re-resolves the byte-identical packet without writing any state.
func TestDogfoodHandoffReceiptReresolvesSamePacket(t *testing.T) {
	t.Parallel()
	root, key := handoffRepository(t)
	before := dogfoodPrivateFiles(t, root)
	file, document := emitHandoff(t, root, key)
	receipt := document.Receipt
	head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if document.Tool != "dogfood-handoff" || document.Mode != "emit" || document.Mutates || !document.OK || document.Claim != "caller-owned-selected-workflow-only" {
		t.Fatalf("envelope: %+v", document)
	}
	if receipt.Profile != handoffProfile || receipt.Authority != "none" || receipt.SessionKey != key || receipt.Root != resolved || receipt.Revision.Commit != head || receipt.Enrollment.Lifecycle != "active" {
		t.Fatalf("receipt identity: %+v", receipt)
	}
	if len(receipt.Anchors) != 2 || receipt.Anchors[0].Anchor != "AGENTS.md" || receipt.Anchors[1].Anchor != "LCP-TEST-001" {
		t.Fatalf("anchors not sorted and unique: %+v", receipt.Anchors)
	}
	if receipt.Packet.Bytes <= 0 || receipt.Packet.Bytes > handoffBudget || !reflect.DeepEqual(receipt.Degradations, []string{"frontier-authority-unavailable"}) {
		t.Fatalf("packet or degradations: %+v", receipt)
	}
	encoded, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if framed, _ := repoenvelope.Frame(string(encoded)); document.Envelope != framed || !strings.HasPrefix(document.Envelope, repoenvelope.Prefix) {
		t.Fatalf("receipt is not framed as untrusted data: %q", document.Envelope)
	}
	code, result, stderr := consumeHandoff(t, root, key, file)
	if code != 0 {
		t.Fatalf("consume exit %d: %s %v", code, stderr, result)
	}
	handoff := result["handoff"].(map[string]any)
	if result["mutates"] != false || handoff["state"] != "reresolved" || len(handoff["drift"].([]any)) != 0 || handoff["packetSha256"] != receipt.Packet.SHA256 {
		t.Fatalf("consume: %v", result)
	}
	packet, err := gokernel.CanonicalJSON(result["packet"])
	if err != nil {
		t.Fatal(err)
	}
	packet = append(packet, '\n')
	if dogfoodSHA(packet) != receipt.Packet.SHA256 || len(packet) != receipt.Packet.Bytes {
		t.Fatalf("re-resolved packet differs from the receipt digest")
	}
	if len(result["packet"].(map[string]any)["task_evidence"].([]any)) == 0 {
		t.Fatalf("explicit anchors lost: %v", result["packet"])
	}
	if !reflect.DeepEqual(before, dogfoodPrivateFiles(t, root)) {
		t.Fatal("handoff emit or consume mutated private or repository state")
	}
}

// TestDogfoodHandoffReportsRevisionAndAnchorDrift pins SESSION-V0-018 and
// SESSION-V0-019: on a later revision the receiver names the revision and the
// one anchor whose evidence moved, withholds the recompiled packet and exits 1;
// a foreign key or malformed receipt is refused.
func TestDogfoodHandoffReportsRevisionAndAnchorDrift(t *testing.T) {
	t.Parallel()
	root, key := handoffRepository(t)
	file, document := emitHandoff(t, root, key)
	if err := os.WriteFile(filepath.Join(root, "docs", "specs", "event.md"), []byte("# Event specification\n\nMoved.\n\n- `LCP-TEST-001`: Preserve an explicit requirement anchor.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "commit", "-qam", "move requirement")
	code, result, stderr := consumeHandoff(t, root, key, file)
	if code != 1 {
		t.Fatalf("drift exit %d, want 1: %s", code, stderr)
	}
	handoff := result["handoff"].(map[string]any)
	if handoff["state"] != "drifted" || result["packet"] != nil || result["mutates"] != false {
		t.Fatalf("drift must withhold the recompiled packet: %v", result)
	}
	fields := map[string]map[string]any{}
	for _, row := range handoff["drift"].([]any) {
		entry := row.(map[string]any)
		name := entry["field"].(string)
		if anchor, ok := entry["anchor"].(string); ok {
			name += ":" + anchor
		}
		fields[name] = entry
	}
	head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	revision := fields["revision"]
	if revision == nil || revision["receipt"].(map[string]any)["commit"] != document.Receipt.Revision.Commit || revision["current"].(map[string]any)["commit"] != head {
		t.Fatalf("revision drift not exact: %v", handoff["drift"])
	}
	if fields["anchor:LCP-TEST-001"] == nil || fields["anchor:AGENTS.md"] != nil || fields["packet"] == nil || fields["root"] != nil {
		t.Fatalf("anchor drift not exact: %v", handoff["drift"])
	}
	t.Run("foreign key", func(t *testing.T) {
		code, _, stderr := consumeHandoff(t, root, localcompletion.HashSession("other-session"), file)
		if code != 2 || !strings.Contains(stderr, "handoff-session-key-mismatch") {
			t.Fatalf("foreign key: %d %s", code, stderr)
		}
	})
	t.Run("malformed receipt", func(t *testing.T) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for name, tampered := range map[string]string{
			"unknown member": strings.Replace(string(raw), `"authority":"none"`, `"authority":"none","extra":1`, 1),
			"authority":      strings.Replace(string(raw), `"authority":"none"`, `"authority":"granted"`, 1),
			"trailing data":  string(raw) + "{}",
		} {
			bad := filepath.Join(t.TempDir(), "receipt.json")
			if err := os.WriteFile(bad, []byte(tampered), 0600); err != nil {
				t.Fatal(err)
			}
			code, _, stderr := consumeHandoff(t, root, key, bad)
			if code != 2 || !strings.Contains(stderr, "invalid-handoff-receipt") {
				t.Fatalf("%s: %d %s", name, code, stderr)
			}
		}
	})
	t.Run("anchors and receipt are exclusive", func(t *testing.T) {
		code, _, stderr := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--anchors", "AGENTS.md", "--receipt", file)
		if code != 2 || !strings.Contains(stderr, "invalid-local-completion-option") {
			t.Fatalf("exclusive options: %d %s", code, stderr)
		}
	})
}
