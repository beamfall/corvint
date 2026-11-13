// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// affectedRow is the table's corvint.affected row.
func affectedRow(t *testing.T) capcatalog.MCPTool {
	t.Helper()
	for _, row := range capcatalog.MCPTools() {
		if row.Name == AffectedTool {
			return row
		}
	}
	t.Fatal("no corvint.affected table row")
	return capcatalog.MCPTool{}
}

// MTT-V1-001: the table advertises exactly the codes corvint.affected
// answers, and every failure path the tool has, driven one by one, answers a
// code the table lists: the group's own refusals, each planner code it passes
// through, an unlisted or missing planner code as affected-failed, an
// oversized verdict and an envelope collision.
func TestMTTV1001AffectedErrorsAreAdvertised(t *testing.T) {
	row := affectedRow(t)
	want := append(AffectedErrors(), capcatalog.ErrorServerBusy)
	slices.Sort(want)
	advertised := slices.Sorted(slices.Values(row.Errors))
	if !slices.Equal(advertised, slices.Compact(want)) {
		t.Fatalf("table advertises %q, the tool answers %q", advertised, want)
	}
	root, binding := boundRoot(t)
	stderrRun := func(text string) AffectedRunner {
		return func(_ context.Context, _, _ string, _, stderr io.Writer) int {
			_, _ = io.WriteString(stderr, text)
			return 2
		}
	}
	stdoutRun := func(text string) AffectedRunner {
		return func(_ context.Context, _, _ string, stdout, _ io.Writer) int {
			_, _ = io.WriteString(stdout, text)
			return 0
		}
	}
	moved := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(moved, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	type path struct {
		group Affected
		ctx   context.Context
		code  string
	}
	paths := map[string]path{
		"no runner":     {Affected{Root: root, Binding: binding}, context.Background(), "not-served"},
		"moved root":    {NewAffected(moved, binding, stdoutRun("{}")), context.Background(), "invalid-root"},
		"cancelled":     {NewAffected(root, binding, stdoutRun("{}")), cancelled, CancelledCode},
		"silent":        {NewAffected(root, binding, stderrRun("")), context.Background(), AffectedFailedCode},
		"raw stderr":    {NewAffected(root, binding, stderrRun("panic: planner\n")), context.Background(), AffectedFailedCode},
		"unlisted code": {NewAffected(root, binding, stderrRun(`{"code":"output-failed","error":"cannot write"}`)), context.Background(), AffectedFailedCode},
		"collision":     {NewAffected(root, binding, stdoutRun(`{"profile":"affected-summary/1","x":"`+repoenvelope.Terminator+`"}`)), context.Background(), repoenvelope.CollisionCode},
		"oversized":     {NewAffected(root, binding, stdoutRun(`{"profile":"affected-summary/1","x":"`+strings.Repeat("u", 2<<20)+`"}`)), context.Background(), AffectedOversizedCode},
	}
	for _, code := range affectedPlannerCodes {
		paths["planner "+code] = path{NewAffected(root, binding, stderrRun(fmt.Sprintf(`{"code":%q,"error":"refused","ok":false}`, code))), context.Background(), code}
	}
	reached := map[string]bool{}
	for name, test := range paths {
		result, failure := test.group.Call(test.ctx, AffectedTool, map[string]any{"base": "HEAD~1"})
		structured, _ := result["structuredContent"].(map[string]any)
		code, _ := structured["code"].(string)
		if failure != nil || result["isError"] != true || code != test.code {
			t.Fatalf("%s: code %q want %q; failure %#v", name, code, test.code, failure)
		}
		if !slices.Contains(row.Errors, code) {
			t.Fatalf("%s: answered %q, which the table does not advertise %q", name, code, row.Errors)
		}
		reached[code] = true
	}
	for _, code := range AffectedErrors() {
		if !reached[code] {
			t.Fatalf("no path drove %q", code)
		}
	}
	// The planner's base refusal reaches the tool unchanged (the review's
	// {"base":"a"} case), now a listed code.
	result, _ := NewAffected(root, binding, stderrRun(`{"code":"invalid-arguments","error":"--base must be a full commit id, got a","ok":false}`)).Call(context.Background(), AffectedTool, map[string]any{"base": "a"})
	if structured, _ := result["structuredContent"].(map[string]any); structured["code"] != "invalid-arguments" || !slices.Contains(row.Errors, "invalid-arguments") {
		t.Fatalf("base a: %#v", result)
	}
}
