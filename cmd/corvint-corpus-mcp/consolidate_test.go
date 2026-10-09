package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/testplan"
)

func consolidationInput(t *testing.T, specs ...string) string {
	t.Helper()
	rows := []any{}
	for i, spec := range specs {
		rows = append(rows, map[string]any{
			"variation_id": "V" + strconv.Itoa(i+1), "spec": spec, "app": "admin", "setup": "scenarios/club.ts",
			"screen": "screen:admin:club.teesheet", "user": "club-admin", "org": "club-a",
			"action": []any{"element:act" + strconv.Itoa(i)}, "assertion": []any{"element:see"}, "requires": []any{}, "changes": []any{},
			"destructive": i == 2, "witnesses": []any{map[string]any{"test_id": "absent", "basis": "declared"}},
		})
	}
	data, err := json.Marshal(map[string]any{"schema": testplan.InputSchema, "variations": rows})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TCN-V0-012: corvint-corpus-mcp lists corvint.consolidate_tests only with --consolidation,
// decodes its arguments strictly, confines tests and maps to the root, returns the CLI's exact
// bytes framed as untrusted data with the plan as structured content, checks a pasted plan, and
// refuses rather than exceed one MCP message.
func TestTCNV0012CorpusMCPConsolidateTests(t *testing.T) {
	root, _ := mapPlanRoot(t)
	for _, c := range []struct {
		args          []string
		ok            bool
		consolidation bool
	}{
		{[]string{"--root", root, "--artifact", "corpus.json"}, true, false},
		{[]string{"--root", root, "--artifact", "corpus.json", "--consolidation"}, true, true},
		{[]string{"--root", root, "--artifact", "corpus.json", "--map", "a.json", "--consolidation", "--map", "b.json"}, true, true},
		{[]string{"--root", root, "--artifact", "corpus.json", "--consolidation", "--consolidation"}, false, false},
		{[]string{"--root", root, "--artifact", "corpus.json", "--consolidation", "x"}, false, false},
		{[]string{"--root", root, "--artifact", "corpus.json", "--consolidation=true"}, false, false},
		{[]string{"--consolidation", "--root", root, "--artifact", "corpus.json"}, false, false},
	} {
		_, _, _, consolidation, _, ok := parseArguments(c.args)
		if ok != c.ok || consolidation != c.consolidation {
			t.Errorf("parseArguments(%q) = %v %v", c.args, consolidation, ok)
		}
	}

	registry, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	listed := func(h *toolHandler) map[string]any {
		result, rpc := h.Handle(context.Background(), protocol.Request{Method: "tools/list", Params: map[string]any{}}, nil)
		if rpc != nil {
			t.Fatal(rpc)
		}
		raw, _ := json.Marshal(result["tools"])
		var tools []map[string]any
		_ = json.Unmarshal(raw, &tools)
		for _, tool := range tools {
			if tool["name"] == consolidateTool {
				return tool
			}
		}
		return nil
	}
	if listed(&toolHandler{registry: registry}) != nil {
		t.Fatal("consolidate_tests listed without --consolidation")
	}
	h := &toolHandler{registry: registry, consolidator: &consolidator{root: root}}
	tool := listed(h)
	if tool == nil {
		t.Fatal("consolidate_tests not listed")
	}
	if hints := tool["annotations"].(map[string]any); hints["readOnlyHint"] != true || hints["idempotentHint"] != true {
		t.Fatalf("annotations %v", hints)
	}
	if hidden, rpc := (&toolHandler{registry: registry}).call(context.Background(), map[string]any{"name": consolidateTool, "arguments": map[string]any{"input": "{}"}}); rpc == nil && hidden["isError"] != true {
		t.Fatal("an unlisted consolidate_tests answered")
	}

	// A provider document and a map inside the root.
	sum := sha256.Sum256([]byte("absent spec\n"))
	receipt, _ := json.Marshal(map[string]any{"receipt": map[string]any{"kind": "e2e",
		"identity": map[string]any{"testFileDigests": map[string]string{filepath.Join(root, "missing.spec.ts"): hex.EncodeToString(sum[:])}},
		"tests":    []any{map[string]any{"name": "absent", "state": "passed"}}}})
	if err := os.WriteFile(filepath.Join(root, "receipt.json"), receipt, 0o644); err != nil {
		t.Fatal(err)
	}
	input := consolidationInput(t, "tests/e2e/a.spec.ts", "tests/e2e/a.spec.ts", "tests/e2e/a.spec.ts")
	call := func(arguments map[string]any) (map[string]any, *protocol.RPCError) {
		return h.call(context.Background(), map[string]any{"name": consolidateTool, "arguments": arguments})
	}
	text := func(result map[string]any) string {
		return result["content"].([]any)[0].(map[string]any)["text"].(string)
	}
	library, err := testplan.Run(context.Background(), testplan.Request{Root: root, Input: []byte(input), MaxSteps: 4,
		Tests: []string{filepath.Join(root, "receipt.json")}, Maps: []string{filepath.Join(root, "map-appmap.json")}})
	if err != nil {
		t.Fatal(err)
	}
	var wantPlan map[string]any
	_ = json.Unmarshal(library.JSON(), &wantPlan)
	for format, want := range map[string][]byte{"table": library.Table(), "json": library.JSON()} {
		result, rpc := call(map[string]any{"input": input, "tests": []any{"receipt.json"}, "maps": []any{"map-appmap.json"}, "max_steps": 4, "format": format})
		if rpc != nil || result["isError"] != false {
			t.Fatalf("%s: %#v %v", format, result, rpc)
		}
		framed, _ := repoenvelope.Frame(string(want))
		if text(result) != framed || !reflect.DeepEqual(result["structuredContent"], wantPlan) {
			t.Fatalf("%s: the result is not the CLI bytes and plan:\n%s", format, text(result))
		}
	}
	if wantPlan["authority"] != "candidate" || !strings.Contains(string(library.Table()), "anchors=VALIDATED") || wantPlan["status"] != "INCOMPLETE" ||
		!strings.Contains(string(library.JSON()), `"reason":"unresolved-screen"`) {
		t.Fatalf("plan: %s", library.JSON())
	}

	// An absolute link to a file inside the root is read like the file itself.
	if err := os.Symlink(filepath.Join(root, "receipt.json"), filepath.Join(root, "receipt-link.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "map-appmap.json"), filepath.Join(root, "map-link.json")); err != nil {
		t.Fatal(err)
	}
	linked, rpc := call(map[string]any{"input": input, "tests": []any{"receipt-link.json"}, "maps": []any{"map-link.json"}, "max_steps": 4, "format": "json"})
	if framed, _ := repoenvelope.Frame(string(library.JSON())); rpc != nil || linked["isError"] != false || text(linked) != framed {
		t.Fatalf("absolute links inside the root: %#v %v", linked, rpc)
	}

	// Check mode returns the recomputed table, or the check's code as a tool error.
	plain, err := testplan.Run(context.Background(), testplan.Request{Root: root, Input: []byte(input)})
	if err != nil {
		t.Fatal(err)
	}
	table := string(plain.Table())
	if result, rpc := call(map[string]any{"input": input, "plan": "Reviewed:\n\n" + table}); rpc != nil || result["isError"] != false || !strings.Contains(text(result), table) {
		t.Fatalf("check: %#v %v", result, rpc)
	}
	for plan, code := range map[string]string{
		strings.Replace(table, "V1, V2", "V2, V1", 1): "test-plan-mismatch",
		"no table": "test-plan-header-missing",
	} {
		result, rpc := call(map[string]any{"input": input, "plan": plan})
		if rpc != nil || result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != code {
			t.Errorf("check %q: %#v %v", code, result, rpc)
		}
	}

	// Refusals inside the schema are coded tool errors.
	outside := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(outside, receipt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leak.json")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		arguments map[string]any
		code      string
	}{
		{map[string]any{"input": input, "tests": []any{"../receipt.json"}}, "test-plan-invalid-arguments"},
		{map[string]any{"input": input, "tests": []any{outside}}, "test-plan-invalid-arguments"},
		{map[string]any{"input": input, "tests": []any{"leak.json"}}, "test-plan-invalid-arguments"},
		{map[string]any{"input": input, "maps": []any{"absent.json"}}, "test-plan-invalid-arguments"},
		{map[string]any{"input": input, "maps": []any{"corpus.json"}}, "appmap-invalid-map"},
		{map[string]any{"input": input, "tests": []any{"corpus.json"}}, "invalid-test-validity-receipt"},
		{map[string]any{"input": `{"schema":"test-consolidation-input/0","schema":"test-consolidation-input/0","variations":[]}`}, "test-plan-invalid-input"},
		{map[string]any{"input": "not json"}, "test-plan-invalid-input"},
	} {
		result, rpc := call(c.arguments)
		if rpc != nil || result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != c.code {
			t.Errorf("%v: %#v %v", c.arguments, result, rpc)
		}
	}

	// Arguments outside the advertised schema are invalid params.
	nine := []any{}
	for range 9 {
		nine = append(nine, "receipt.json")
	}
	for _, arguments := range []map[string]any{
		{},
		{"input": nil},
		{"input": ""},
		{"input": 1},
		{"input": strings.Repeat(" ", maxInlineBytes+1)},
		{"input": input, "extra": true},
		{"input": input, "revision": "HEAD"},
		{"input": input, "tests": nil},
		{"input": input, "tests": "receipt.json"},
		{"input": input, "tests": nine},
		{"input": input, "maps": []any{""}},
		{"input": input, "maps": []any{1}},
		{"input": input, "max_steps": 1},
		{"input": input, "max_steps": 33},
		{"input": input, "max_steps": 4.5},
		{"input": input, "max_steps": "8"},
		{"input": input, "max_steps": nil},
		{"input": input, "format": "xml"},
		{"input": input, "format": nil},
		{"input": input, "plan": ""},
		{"input": input, "plan": nil},
		{"input": input, "plan": table, "format": "table"},
	} {
		if _, rpc := call(arguments); rpc == nil {
			t.Errorf("arguments %v accepted", arguments)
		}
	}

	// A plan whose framed text and structured copy would exceed one MCP message is refused.
	specs := []string{}
	for i := range 240 {
		specs = append(specs, "tests/"+strconv.Itoa(i)+"/"+strings.Repeat("s", 3800)+".spec.ts")
	}
	large := consolidationInput(t, specs...)
	if len(large) > maxInlineBytes {
		t.Fatalf("the bound fixture is %d bytes", len(large))
	}
	result, rpc := call(map[string]any{"input": large})
	if rpc != nil || result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != "test-plan-bound-exceeded" {
		t.Fatalf("bound: %v %v", result["structuredContent"], rpc)
	}
	if _, err := testplan.Run(context.Background(), testplan.Request{Root: root, Input: []byte(large)}); err != nil {
		t.Fatalf("the CLI bound refused the same input: %v", err)
	}
	fits, rpc := call(map[string]any{"input": consolidationInput(t, specs[:80]...)})
	if encoded, _ := json.Marshal(fits); rpc != nil || fits["isError"] != false || len(encoded) > consolidateResponseBytes {
		t.Fatalf("a smaller plan: %v %d", rpc, len(encoded))
	}
}
