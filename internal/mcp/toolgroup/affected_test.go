// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// MMR-V0-1118 (V1-5254): the group's closed input admits one non-empty,
// bounded, never option-like base and nothing else; a group with no runner is
// the not-served tool error, a root whose binding moved is invalid-root, and a
// successful run answers the planner's stdout bytes framed, the refusal of a
// failed run carrying the planner's error and its code when the table lists
// it, affected-failed led by that code otherwise (MTT-V1-001).
func TestMMRV01118AffectedCallContract(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding, ok := rootbind.Bind(root)
	if !ok {
		t.Fatal("bind")
	}
	runs := 0
	runner := func(_ context.Context, gotRoot, base string, stdout, stderr io.Writer) int {
		runs++
		if gotRoot != root {
			t.Fatalf("root=%q want %q", gotRoot, root)
		}
		switch base {
		case "unknown":
			_, _ = fmt.Fprintln(stderr, `{"code": "unknown-base", "error": "argument --base: unknown", "ok": false}`)
			return 2
		case "silent":
			return 2
		}
		_, _ = fmt.Fprintf(stdout, "{\"base\":%q,\"profile\":\"affected-summary/1\"}\n", base)
		return 0
	}
	group := Affected{Root: root, Binding: binding, Run: runner}
	ctx := context.Background()
	for name, arguments := range map[string]map[string]any{
		"empty base":       {"base": ""},
		"option-like base": {"base": "--full"},
		"dash base":        {"base": "-"},
		"non-string base":  {"base": 7},
		"unknown member":   {"path": "x"},
		"extra member":     {"base": "HEAD~1", "full": true},
		"too long":         {"base": strings.Repeat("a", affectedBaseLimit+1)},
	} {
		if _, failure := group.Call(ctx, AffectedTool, arguments); failure == nil || failure.Code != protocol.CodeInvalidParams {
			t.Fatalf("%s: %#v", name, failure)
		}
	}
	if _, failure := group.Call(ctx, "corvint.other", map[string]any{}); failure == nil || failure.Code != protocol.CodeInvalidParams {
		t.Fatalf("unknown tool: %#v", failure)
	}
	if runs != 0 {
		t.Fatalf("a refused argument reached the planner %d time(s)", runs)
	}
	for _, test := range []struct {
		name          string
		arguments     map[string]any
		code, message string
	}{
		{"unlisted planner code", map[string]any{"base": "unknown"}, "affected-failed", "unknown-base: argument --base: unknown"},
		{"silent failure", map[string]any{"base": "silent"}, "affected-failed", "corvint affected --summary failed"},
	} {
		result, failure := group.Call(ctx, AffectedTool, test.arguments)
		structured, _ := result["structuredContent"].(map[string]any)
		if failure != nil || result["isError"] != true || structured["code"] != test.code || structured["error"] != test.message || structured["profile"] != AffectedErrorProfile || structured["tool"] != AffectedTool {
			t.Fatalf("%s: %#v %#v", test.name, result, failure)
		}
	}
	for _, base := range []string{"", "HEAD~1", strings.Repeat("f", 40)} {
		arguments := map[string]any{}
		if base != "" {
			arguments["base"] = base
		}
		result, failure := group.Call(ctx, AffectedTool, arguments)
		if failure != nil || result["isError"] != false {
			t.Fatalf("base %q: %#v %#v", base, result, failure)
		}
		want := fmt.Sprintf("{\"base\":%q,\"profile\":\"affected-summary/1\"}\n", base)
		framed, err := repoenvelope.Frame(want)
		if err != nil {
			t.Fatal(err)
		}
		structured, _ := result["structuredContent"].(map[string]any)
		if text := result["content"].([]any)[0].(map[string]any)["text"]; text != framed || structured["base"] != base {
			t.Fatalf("base %q: text %q structured %#v", base, text, structured)
		}
	}
	if result, failure := (Affected{Root: root, Binding: binding}).Call(ctx, AffectedTool, map[string]any{}); failure != nil || result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != "not-served" {
		t.Fatalf("no runner: %#v %#v", result, failure)
	}
	moved := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(moved, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if result, failure := (Affected{Root: moved, Binding: binding, Run: runner}).Call(ctx, AffectedTool, map[string]any{}); failure != nil || result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != "invalid-root" {
		t.Fatalf("moved root: %#v %#v", result, failure)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	// DMF-V1-010: a cancelled call is the cancelled tool error, not -32603.
	if result, failure := group.Call(cancelled, AffectedTool, map[string]any{}); failure != nil || result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != CancelledCode {
		t.Fatalf("cancelled: %#v %#v", result, failure)
	}
}
