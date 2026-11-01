package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// MCPV0-100: every profile advertises and answers corvint.help from the
// embedded catalog and guide, without reading the repository, within the
// bridge budget, and refuses any other argument as invalid.
func TestMCPV0100HelpToolReturnsTheEmbeddedCatalogAndGuide(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, constructor := range map[string]func(string) (*Registry, *Error){"default": New, "task-review": NewTaskReview, "task-review-lsp": NewTaskReviewLSP, "flows": NewFlows} {
		registry, err := constructor(root)
		if err != nil {
			t.Fatal(err)
		}
		assertHelpTool(t, name, registry)
	}
}

func assertHelpTool(t *testing.T, profile string, registry *Registry) {
	t.Helper()
	advertised := false
	for _, tool := range registry.Tools() {
		advertised = advertised || tool.Name == ToolHelp
	}
	if !advertised {
		t.Fatalf("%s profile does not advertise %s", profile, ToolHelp)
	}
	// MCPV0-953: the guide by default and one command's slice when named.
	querySlice, _, _ := capcatalog.Slice(capcatalog.Snapshot(), "query")
	for arguments, want := range map[string]string{
		`{}`:                                    capcatalog.Guide(),
		`{"topic":"catalog"}`:                   string(capcatalog.Snapshot()),
		`{"topic":"guide"}`:                     capcatalog.Guide(),
		`{"command":"query"}`:                   string(querySlice),
		`{"topic":"command","command":"query"}`: string(querySlice),
	} {
		// The repository is an empty .git directory: any repository read would fail.
		result, callErr := registry.Call(context.Background(), ToolHelp, []byte(arguments))
		if callErr != nil {
			t.Fatalf("%s %s: %v", profile, arguments, callErr)
		}
		digest := sha256.Sum256([]byte(want))
		if result.State != "READY" || result.AuthorityClass != "CORVINT_BINARY" || result.Repository != nil ||
			result.Receipt["text"] != want || result.Receipt["sha256"] != hex.EncodeToString(digest[:]) || result.Receipt["byteCount"] != len(want) {
			t.Fatalf("%s %s result = %+v", profile, arguments, result)
		}
		encoded, encodeErr := result.CanonicalJSON()
		if encodeErr != nil || len(encoded) > maxResultBytes/2 || !fitsMCPFrame(encoded, result) {
			t.Fatalf("%s %s: %d canonical bytes leave less than half the %d-byte budget (%v)", profile, arguments, len(encoded), maxResultBytes, encodeErr)
		}
		again, _ := registry.Call(context.Background(), ToolHelp, []byte(arguments))
		if second, _ := again.CanonicalJSON(); string(second) != string(encoded) {
			t.Fatalf("%s %s is not deterministic", profile, arguments)
		}
	}
	for _, arguments := range []string{`{"topic":"COMMAND"}`, `{"topic":null}`, `{"topic":1}`, `{"extra":true}`, `null`, `[]`,
		`{"topic":"command"}`, `{"command":"nope"}`, `{"command":null}`, `{"topic":"guide","command":"query"}`, `{"topic":"catalog","command":"query"}`} {
		if _, callErr := registry.Call(context.Background(), ToolHelp, []byte(arguments)); callErr == nil || callErr.Code != "invalid-arguments" {
			t.Fatalf("%s %s error = %#v", profile, arguments, callErr)
		}
	}
	ready, _ := registry.Call(context.Background(), ToolHelp, []byte(`{"topic":"catalog"}`))
	borrowed := ready
	borrowed.Tool = ToolStatus
	if !validResult(ready) || validResult(borrowed) {
		t.Fatalf("%s: validResult refuses a READY corvint.help or accepts CORVINT_BINARY on another tool", profile)
	}
	catalogText := string(capcatalog.Snapshot())
	replaced := catalogText + " "
	replacedDigest := sha256.Sum256([]byte(replaced))
	for name, tamper := range map[string]func(map[string]any){
		"text replaced": func(receipt map[string]any) { receipt["text"] = "x" },
		"text replaced with recomputed hash": func(receipt map[string]any) {
			receipt["text"], receipt["sha256"], receipt["byteCount"] = replaced, hex.EncodeToString(replacedDigest[:]), len(replaced)
		},
		"command missing":               func(receipt map[string]any) { delete(receipt, "command") },
		"command of the other topic":    func(receipt map[string]any) { receipt["command"] = "corvint help guide" },
		"guide topic with catalog text": func(receipt map[string]any) { receipt["topic"], receipt["command"] = "guide", "corvint help guide" },
		"negative byteCount":            func(receipt map[string]any) { receipt["byteCount"] = -1 },
		"byteCount not an integer":      func(receipt map[string]any) { receipt["byteCount"] = float64(len(catalogText)) },
		"extra member":                  func(receipt map[string]any) { receipt["note"] = "x" },
		"schema missing":                func(receipt map[string]any) { delete(receipt, "schema") },
		"command topic with catalog text": func(receipt map[string]any) {
			receipt["topic"], receipt["command"] = "command", "corvint help --json query"
		},
	} {
		tampered := ready
		tampered.Receipt = map[string]any{}
		for key, value := range ready.Receipt {
			tampered.Receipt[key] = value
		}
		tamper(tampered.Receipt)
		if validResult(tampered) {
			t.Fatalf("%s: validResult keeps CORVINT_BINARY authority for a receipt with its %s", profile, name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, callErr := registry.Call(ctx, ToolHelp, []byte(`{}`)); callErr == nil || callErr.Code != "cancelled" {
		t.Fatalf("%s cancelled call error = %#v", profile, callErr)
	}
}

// MCPV0-100: cancellation is checked before the budget, so an oversized text
// under a cancelled context fails cancelled, and only a live call abstains.
func TestMCPV0100HelpCancellationPrecedesTheBudget(t *testing.T) {
	t.Parallel()
	oversized := strings.Repeat("x", maxResultBytes+1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, callErr := helpResult(ctx, helpTopicCatalog, "corvint help --json", oversized); callErr == nil || callErr.Code != "cancelled" {
		t.Fatalf("cancelled oversized call error = %#v", callErr)
	}
	result, callErr := helpResult(context.Background(), helpTopicCatalog, "corvint help --json", oversized)
	if callErr != nil || result.State != "ABSTAINED" || result.Abstention.Reason != "OUTPUT_BUDGET_EXCEEDED" || result.Receipt != nil {
		t.Fatalf("live oversized call = %+v, %#v", result, callErr)
	}
}

// MCPV0-104: the companion servers' entry answers exactly what Registry.Call
// answers, wrapped as cmd/corvint-mcp wraps it, and refuses the same inputs.
func TestMCPV0104HelpToolResultIsTheRegistryAnswer(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, arguments := range []string{`{}`, `{"topic":"catalog"}`, `{"topic":"guide"}`} {
		answer, callErr := HelpToolResult(context.Background(), []byte(arguments))
		if callErr != nil {
			t.Fatalf("%s: %v", arguments, callErr)
		}
		result, _ := registry.Call(context.Background(), ToolHelp, []byte(arguments))
		object, _ := result.Object()
		text, _ := result.TextJSON()
		framed, _ := repoenvelope.Frame(string(text))
		content, _ := answer["content"].([]any)
		if len(answer) != 3 || answer["isError"] != false || !reflect.DeepEqual(answer["structuredContent"], object) || len(content) != 1 ||
			!reflect.DeepEqual(content[0], map[string]any{"type": "text", "text": framed}) {
			t.Fatalf("%s: companion answer differs from the registry answer", arguments)
		}
	}
	if !reflect.DeepEqual(HelpDescriptor(), helpDescriptor()) {
		t.Fatal("HelpDescriptor differs from the corvint-mcp descriptor")
	}
	for _, arguments := range []string{``, `{"topic":"COMMAND"}`, `{"topic":null}`, `{"extra":true}`, `null`, `[]`, "{\"topic\":\"\xff\"}", `{"topic":"` + strings.Repeat("x", maxArgumentSize) + `"}`} {
		if _, callErr := HelpToolResult(context.Background(), []byte(arguments)); callErr == nil || callErr.Code != "invalid-arguments" {
			t.Fatalf("%.40q error = %#v", arguments, callErr)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, callErr := HelpToolResult(ctx, []byte(`{}`)); callErr == nil || callErr.Code != "cancelled" {
		t.Fatalf("cancelled call error = %#v", callErr)
	}
}
