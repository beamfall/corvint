package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
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
	for arguments, want := range map[string]string{
		`{}`:                  string(capcatalog.Snapshot()),
		`{"topic":"catalog"}`: string(capcatalog.Snapshot()),
		`{"topic":"guide"}`:   capcatalog.Guide(),
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
	for _, arguments := range []string{`{"topic":"COMMAND"}`, `{"topic":null}`, `{"topic":1}`, `{"extra":true}`, `null`, `[]`} {
		if _, callErr := registry.Call(context.Background(), ToolHelp, []byte(arguments)); callErr == nil || callErr.Code != "invalid-arguments" {
			t.Fatalf("%s %s error = %#v", profile, arguments, callErr)
		}
	}
	ready, _ := registry.Call(context.Background(), ToolHelp, []byte(`{}`))
	borrowed := ready
	borrowed.Tool = ToolStatus
	tampered := ready
	tampered.Receipt = map[string]any{}
	for key, value := range ready.Receipt {
		tampered.Receipt[key] = value
	}
	tampered.Receipt["text"] = "x"
	if !validResult(ready) || validResult(borrowed) || validResult(tampered) {
		t.Fatalf("%s: validResult accepts a CORVINT_BINARY result outside a READY corvint.help or with a mismatched digest", profile)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, callErr := registry.Call(ctx, ToolHelp, []byte(`{}`)); callErr == nil || callErr.Code != "cancelled" {
		t.Fatalf("%s cancelled call error = %#v", profile, callErr)
	}
}
