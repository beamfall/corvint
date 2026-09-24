package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// handoffSpec has a non-ASCII path, so the packet carries bytes that emit
// escapes on stdout.
var handoffSpec = filepath.Join("docs", "specs", "événement.md")

// handoffRepository is an enrolled fixture whose requirement and governance
// anchors resolve to task evidence.
func handoffRepository(t *testing.T) (string, string) {
	t.Helper()
	root := queryCLIRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "specs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, handoffSpec), []byte("# Event specification\n\n- `LCP-TEST-001`: Preserve an explicit requirement anchor.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", handoffSpec)
	gitOutput(t, root, "commit", "-qm", "add requirement fixture")
	key := localcompletion.HashSession("handoff-session")
	base := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	plan, err := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{filepath.ToSlash(handoffSpec)}, Checks: []localcompletion.Check{{ID: "check", Argv: []string{"false"}, TimeoutSeconds: 1}}})
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
	return writeHandoffFile(t, []byte(stdout)), document
}

func writeHandoffFile(t *testing.T, raw []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

// writeHandoffDocument re-emits an edited document exactly as emit would.
func writeHandoffDocument(t *testing.T, document handoffDocument) string {
	t.Helper()
	var encoded bytes.Buffer
	if err := emit(&encoded, document); err != nil {
		t.Fatal(err)
	}
	return writeHandoffFile(t, encoded.Bytes())
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

// handoffDriftFields indexes drift rows by field, with the anchor name appended.
func handoffDriftFields(result map[string]any) map[string]map[string]any {
	fields := map[string]map[string]any{}
	for _, row := range result["handoff"].(map[string]any)["drift"].([]any) {
		entry := row.(map[string]any)
		name := entry["field"].(string)
		if anchor, ok := entry["anchor"].(string); ok {
			name += ":" + anchor
		}
		fields[name] = entry
	}
	return fields
}

// TestDogfoodHandoffReceiptReresolvesSamePacket pins SESSION-V0-017 and
// SESSION-V0-018: the receipt carries identity, anchors, packet digest and
// degradations as untrusted data, and a second read on the same revision
// re-resolves the packet whose printed bytes hash to the receipt digest,
// without writing any state.
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
	if !reflect.DeepEqual(handoff["degradations"], []any{"frontier-authority-unavailable"}) || result["packet"] != nil {
		t.Fatalf("consume degradations or packet member: %v", result)
	}
	printed, _ := result["packetBase64"].(string)
	packet, err := base64.StdEncoding.DecodeString(printed)
	if err != nil {
		t.Fatal(err)
	}
	if dogfoodSHA(packet) != receipt.Packet.SHA256 || len(packet) != receipt.Packet.Bytes {
		t.Fatalf("printed packet bytes do not hash to the receipt digest")
	}
	if !bytes.ContainsFunc(packet, func(r rune) bool { return r > 0x7e }) || !bytes.Contains(packet, []byte(`"relation":"requirement-definition"`)) {
		t.Fatalf("fixture lost its non-ASCII evidence or explicit anchor: %s", packet)
	}
	for _, alias := range []string{root, filepath.Join(root, "docs")} {
		aliased := document
		aliased.Receipt.Root = alias
		if code, result, stderr := consumeHandoff(t, root, key, writeHandoffDocument(t, aliased)); code != 0 {
			t.Fatalf("root alias %s reported drift: %d %s %v", alias, code, stderr, result)
		}
	}
	if !reflect.DeepEqual(before, dogfoodPrivateFiles(t, root)) {
		t.Fatal("handoff emit or consume mutated private or repository state")
	}
}

// TestDogfoodHandoffReportsRevisionAndAnchorDrift pins SESSION-V0-018 and
// SESSION-V0-019: on a later revision the receiver names the revision and the
// one anchor whose evidence moved, withholds the recompiled packet, exits 1
// and writes nothing; a foreign root is reported as root drift.
func TestDogfoodHandoffReportsRevisionAndAnchorDrift(t *testing.T) {
	t.Parallel()
	root, key := handoffRepository(t)
	file, document := emitHandoff(t, root, key)
	if err := os.WriteFile(filepath.Join(root, handoffSpec), []byte("# Event specification\n\nMoved.\n\n- `LCP-TEST-001`: Preserve an explicit requirement anchor.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "commit", "-qam", "move requirement")
	before := dogfoodPrivateFiles(t, root)
	code, result, stderr := consumeHandoff(t, root, key, file)
	if code != 1 {
		t.Fatalf("drift exit %d, want 1: %s", code, stderr)
	}
	handoff := result["handoff"].(map[string]any)
	if handoff["state"] != "drifted" || result["packetBase64"] != nil || result["mutates"] != false {
		t.Fatalf("drift must withhold the recompiled packet: %v", result)
	}
	fields := handoffDriftFields(result)
	head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	revision := fields["revision"]
	if revision == nil || revision["receipt"].(map[string]any)["commit"] != document.Receipt.Revision.Commit || revision["current"].(map[string]any)["commit"] != head {
		t.Fatalf("revision drift not exact: %v", handoff["drift"])
	}
	if fields["anchor:LCP-TEST-001"] == nil || fields["anchor:AGENTS.md"] != nil || fields["packet"] == nil || fields["root"] != nil || fields["enrollment"] != nil {
		t.Fatalf("anchor drift not exact: %v", handoff["drift"])
	}
	if !reflect.DeepEqual(before, dogfoodPrivateFiles(t, root)) {
		t.Fatal("drifted consume mutated private or repository state")
	}
	t.Run("foreign root", func(t *testing.T) {
		foreign := document
		foreign.Receipt.Root = filepath.Join(t.TempDir(), "elsewhere")
		code, result, stderr := consumeHandoff(t, root, key, writeHandoffDocument(t, foreign))
		row := handoffDriftFields(result)["root"]
		if code != 1 || row == nil || row["receipt"] != foreign.Receipt.Root || row["current"] != document.Receipt.Root {
			t.Fatalf("root drift: %d %s %v", code, stderr, result)
		}
	})
}

// TestDogfoodHandoffReportsEnrollmentDriftAndDegradations pins SESSION-V0-017
// and SESSION-V0-019: a cancelled enrollment is enrollment drift and a
// degradation, and uncommitted work is a degradation of the emitted receipt.
func TestDogfoodHandoffReportsEnrollmentDriftAndDegradations(t *testing.T) {
	t.Parallel()
	root, key := handoffRepository(t)
	file, _ := emitHandoff(t, root, key)
	if _, err := localcompletion.Cancel(context.Background(), root, key); err != nil {
		t.Fatal(err)
	}
	before := dogfoodPrivateFiles(t, root)
	code, result, stderr := consumeHandoff(t, root, key, file)
	fields := handoffDriftFields(result)
	enrollment := fields["enrollment"]
	if code != 1 || enrollment == nil || enrollment["receipt"].(map[string]any)["lifecycle"] != "active" || enrollment["current"].(map[string]any)["lifecycle"] != "cancelled" {
		t.Fatalf("enrollment drift: %d %s %v", code, stderr, result)
	}
	if fields["revision"] != nil || fields["root"] != nil {
		t.Fatalf("cancel is only enrollment drift: %v", fields)
	}
	if !slices.Contains(result["handoff"].(map[string]any)["degradations"].([]any), any("enrollment-cancelled")) {
		t.Fatalf("consume omits current degradations: %v", result)
	}
	if !reflect.DeepEqual(before, dogfoodPrivateFiles(t, root)) {
		t.Fatal("drifted consume mutated private or repository state")
	}
	if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("draft\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, document := emitHandoff(t, root, key)
	want := []string{"enrollment-cancelled", "frontier-authority-unavailable", "uncommitted-work"}
	if !reflect.DeepEqual(document.Receipt.Degradations, want) || document.Receipt.Revision.WorktreeState != "mixed" {
		t.Fatalf("degradations: %+v", document.Receipt)
	}
}

// TestDogfoodHandoffRefusesMalformedReceiptsAndAnchors pins SESSION-V0-018:
// every receipt field must have its emitted shape and bytes, the key must
// match, and anchor text is bounded tokens only.
func TestDogfoodHandoffRefusesMalformedReceiptsAndAnchors(t *testing.T) {
	t.Parallel()
	root, key := handoffRepository(t)
	file, document := emitHandoff(t, root, key)
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	hex40, hex64 := strings.Repeat("a", 40), strings.Repeat("b", 64)
	edits := map[string]func(*handoffDocument){
		"tool":              func(d *handoffDocument) { d.Tool = "dogfood-status" },
		"mode":              func(d *handoffDocument) { d.Mode = "consume" },
		"not ok":            func(d *handoffDocument) { d.OK = false },
		"mutates":           func(d *handoffDocument) { d.Mutates = true },
		"profile":           func(d *handoffDocument) { d.Receipt.Profile = "corvint-dogfood-handoff/1" },
		"authority":         func(d *handoffDocument) { d.Receipt.Authority = "granted" },
		"session key":       func(d *handoffDocument) { d.Receipt.SessionKey = strings.ToUpper(key) },
		"relative root":     func(d *handoffDocument) { d.Receipt.Root = "repository" },
		"unclean root":      func(d *handoffDocument) { d.Receipt.Root += "/../repository" },
		"control root":      func(d *handoffDocument) { d.Receipt.Root += "/\x1b[2J" },
		"long root":         func(d *handoffDocument) { d.Receipt.Root = "/" + strings.Repeat("r", handoffRootLen) },
		"commit":            func(d *handoffDocument) { d.Receipt.Revision.Commit = strings.ToUpper(hex40) },
		"tree":              func(d *handoffDocument) { d.Receipt.Revision.Tree = hex40[:39] },
		"dirty digest":      func(d *handoffDocument) { d.Receipt.Revision.DirtyPathsSHA256 = "" },
		"worktree state":    func(d *handoffDocument) { d.Receipt.Revision.WorktreeState = "dirty" },
		"base":              func(d *handoffDocument) { d.Receipt.Enrollment.Base = "main" },
		"plan digest":       func(d *handoffDocument) { d.Receipt.Enrollment.PlanDigest = hex40 },
		"lifecycle":         func(d *handoffDocument) { d.Receipt.Enrollment.Lifecycle = "granted" },
		"packet profile":    func(d *handoffDocument) { d.Receipt.Packet.Profile = "corvint-dogfood-prompt/1" },
		"packet digest":     func(d *handoffDocument) { d.Receipt.Packet.SHA256 = hex64[:63] },
		"zero bytes":        func(d *handoffDocument) { d.Receipt.Packet.Bytes = 0 },
		"bytes over budget": func(d *handoffDocument) { d.Receipt.Packet.Bytes = handoffBudget + 1 },
		"budget":            func(d *handoffDocument) { d.Receipt.Packet.BudgetBytes = handoffBudget / 2 },
		"limit":             func(d *handoffDocument) { d.Receipt.Packet.Limit = handoffLimit + 1 },
		"anchor digest":     func(d *handoffDocument) { d.Receipt.Anchors[0].SHA256 = "sha256:" + hex64 },
		"unsorted anchors": func(d *handoffDocument) {
			d.Receipt.Anchors[0], d.Receipt.Anchors[1] = d.Receipt.Anchors[1], d.Receipt.Anchors[0]
		},
		"duplicate anchors": func(d *handoffDocument) { d.Receipt.Anchors[1] = d.Receipt.Anchors[0] },
	}
	for name, edit := range edits {
		tampered := document
		tampered.Receipt.Anchors = slices.Clone(document.Receipt.Anchors)
		edit(&tampered)
		if code, _, stderr := consumeHandoff(t, root, key, writeHandoffDocument(t, tampered)); code != 2 || !strings.Contains(stderr, "invalid-handoff-receipt") {
			t.Errorf("%s: %d %s", name, code, stderr)
		}
	}
	for name, tampered := range map[string]string{
		"duplicate member":   strings.Replace(string(raw), `"authority":"none"`, `"authority":"none","authority":"none"`, 1),
		"case-folded member": strings.Replace(string(raw), `"authority":"none"`, `"Authority":"none"`, 1),
		"unknown member":     strings.Replace(string(raw), `"authority":"none"`, `"authority":"none","extra":1`, 1),
		"trailing data":      string(raw) + "{}",
		"reformatted":        strings.Replace(string(raw), `{"claim"`, `{ "claim"`, 1),
	} {
		if code, _, stderr := consumeHandoff(t, root, key, writeHandoffFile(t, []byte(tampered))); code != 2 || !strings.Contains(stderr, "invalid-handoff-receipt") {
			t.Errorf("%s: %d %s", name, code, stderr)
		}
	}
	if code, _, stderr := consumeHandoff(t, root, localcompletion.HashSession("other-session"), file); code != 2 || !strings.Contains(stderr, "handoff-session-key-mismatch") {
		t.Errorf("foreign key: %d %s", code, stderr)
	}
	if code, _, stderr := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--anchors", "AGENTS.md", "--receipt", file); code != 2 || !strings.Contains(stderr, "invalid-local-completion-option") {
		t.Errorf("exclusive options: %d %s", code, stderr)
	}
	many := make([]string, 0, handoffMaxAnchors+1)
	for index := range handoffMaxAnchors + 1 {
		many = append(many, fmt.Sprintf("A%d", index))
	}
	for name, anchors := range map[string]string{
		"too many":      strings.Join(many, " "),
		"too long":      strings.Repeat("a", handoffAnchorLen+1),
		"control":       "LCP-TEST-001\x1b[2J",
		"invalid UTF-8": "LCP-\xff",
	} {
		if code, _, stderr := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key, "--anchors", anchors); code != 2 || !strings.Contains(stderr, "invalid-handoff-anchors") {
			t.Errorf("anchors %s: %d %s", name, code, stderr)
		}
	}
}

// TestDogfoodHandoffRefusesUnstableRepository pins SESSION-V0-017: a
// repository that moves during the read is refused, never receipted. It
// replaces the package probe, so it cannot run in parallel.
func TestDogfoodHandoffRefusesUnstableRepository(t *testing.T) {
	root, key := handoffRepository(t)
	probe := handoffProbe
	t.Cleanup(func() { handoffProbe = probe })
	for name, moved := range map[string]int{"dogfood-handoff-context-drift": 1, "dogfood-handoff-repository-drift": 2} {
		calls := 0
		handoffProbe = func(ctx context.Context, root string) (gokernel.Repository, error) {
			repository, err := probe(ctx, root)
			calls++
			if calls == moved {
				repository.CommitRevision = strings.Repeat("0", len(repository.CommitRevision))
			}
			return repository, err
		}
		if code, stdout, stderr := runCLI(t, "--root", root, "dogfood", "handoff", "--session-key", key); code != 2 || stdout != "" || !strings.Contains(stderr, name) {
			t.Errorf("%s: %d %q %s", name, code, stdout, stderr)
		}
	}
}
