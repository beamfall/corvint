// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/selectionswitch"
)

// selectionGroup is an affected group over a bare root whose planner counts
// its runs and whose environment holds value for CORVINT_SELECTION.
func selectionGroup(t *testing.T, value string) (Affected, *int) {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding, ok := rootbind.Bind(root)
	if !ok {
		t.Fatal("bind")
	}
	runs := 0
	runner := func(_ context.Context, _, base string, stdout, _ io.Writer) int {
		runs++
		_, _ = fmt.Fprintf(stdout, "{\"base\":%q,\"profile\":\"affected-summary/1\"}\n", base)
		return 0
	}
	lookup := func(name string) (string, bool) {
		if name != selectionswitch.Variable {
			t.Fatalf("looked up %s", name)
		}
		return value, value != ""
	}
	return Affected{Root: root, Binding: binding, Run: runner, Lookup: lookup}, &runs
}

// MTT-V1-008: an unknown CORVINT_SELECTION value selects every test and
// carries the unknown-value reason naming the value; full and full:go select
// every test (all Go tests among them) with the kill-switch reason; none of
// them runs the planner, and an unset switch plans as before.
func TestMTTV1008AffectedToolReadsTheKillSwitch(t *testing.T) {
	for _, tc := range []struct {
		value, reason string
		runners       []any
	}{
		{"fastest", selectionswitch.ReasonUnknownValue, []any{}},
		{"full", selectionswitch.ReasonKillSwitch, []any{}},
		{"full:go", selectionswitch.ReasonKillSwitch, []any{"go"}},
	} {
		group, runs := selectionGroup(t, tc.value)
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{"base": "HEAD~1"})
		if failure != nil || result["isError"] != false {
			t.Fatalf("%s: %#v %#v", tc.value, result, failure)
		}
		structured, _ := result["structuredContent"].(map[string]any)
		selection, _ := structured["selection"].(map[string]any)
		if structured["profile"] != AffectedSelectAllProfile || structured["scope"] != "FULL" || structured["selectAll"] != true ||
			structured["reason"] != tc.reason || structured["base"] != "HEAD~1" || selection["value"] != tc.value || !reflect.DeepEqual(selection["runners"], tc.runners) {
			t.Fatalf("%s: %#v", tc.value, structured)
		}
		if hint, _ := structured["hint"].(string); hint == "" || strings.Contains(hint, "\n") {
			t.Fatalf("%s: hint %q is not one line", tc.value, hint)
		}
		if tc.value == "fastest" && !strings.Contains(structured["detail"].(string), `"fastest"`) {
			t.Fatalf("unknown value not named: %q", structured["detail"])
		}
		if *runs != 0 {
			t.Fatalf("%s ran the planner %d times", tc.value, *runs)
		}
	}
	group, runs := selectionGroup(t, "")
	result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
	if failure != nil || result["structuredContent"].(map[string]any)["profile"] != "affected-summary/1" || *runs != 1 {
		t.Fatalf("unset switch: %#v %#v runs=%d", result, failure, *runs)
	}
}

// MTT-V1-011: when the planner refuses a stale binary, the tool answers a
// successful select-all with reason self-binary-stale, the planner's message
// as its detail and a one-line rebuild hint; any other refusal stays the tool
// error.
func TestMTTV1011AffectedToolOnAStaleBinarySelectsAll(t *testing.T) {
	group, _ := selectionGroup(t, "")
	group.Run = func(_ context.Context, _, base string, _, stderr io.Writer) int {
		if base == "other" {
			_, _ = fmt.Fprintln(stderr, `{"code": "invalid-arguments", "error": "argument --base: unknown", "ok": false}`)
			return 2
		}
		_, _ = fmt.Fprintln(stderr, `{"code": "self-binary-stale", "error": "this binary was built from commit abc1234; rebuild it", "ok": false}`)
		return 2
	}
	result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
	structured, _ := result["structuredContent"].(map[string]any)
	hint, _ := structured["hint"].(string)
	if failure != nil || result["isError"] != false || structured["profile"] != AffectedSelectAllProfile || structured["selectAll"] != true ||
		structured["reason"] != AffectedStaleCode || structured["detail"] != "this binary was built from commit abc1234; rebuild it" ||
		hint == "" || strings.Contains(hint, "\n") || structured["selection"] != nil {
		t.Fatalf("stale: %#v %#v", result, failure)
	}
	result, _ = group.Call(context.Background(), AffectedTool, map[string]any{"base": "other"})
	if structured, _ := result["structuredContent"].(map[string]any); result["isError"] != true || structured["code"] != "invalid-arguments" {
		t.Fatalf("other refusal: %#v", result)
	}
}

// MTT-V1-008, delivered state: no command line parses CORVINT_SELECTION, so
// selectionswitch.Parse is the grammar's one parser; the tool's answer for
// each review value is exactly the one Parse classifies, and only the unset
// switch runs the planner.
func TestMTTV1008AffectedToolAgreesWithTheOneParser(t *testing.T) {
	for _, value := range []string{"full", "full:go", "full:a,a", "fastest", " full", "full:Go", ""} {
		group, runs := selectionGroup(t, value)
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
		if failure != nil {
			t.Fatalf("%q: %#v", value, failure)
		}
		structured, _ := result["structuredContent"].(map[string]any)
		parsed := selectionswitch.Parse(value)
		if !parsed.On() {
			if structured["profile"] != "affected-summary/1" || *runs != 1 {
				t.Fatalf("%q: unset switch did not plan: %#v", value, structured)
			}
			continue
		}
		if structured["reason"] != parsed.Reason() || structured["selectAll"] != true || *runs != 0 {
			t.Fatalf("%q: tool reason %v, Parse reason %q, runs %d", value, structured["reason"], parsed.Reason(), *runs)
		}
	}
}

// MTT-V1-008 known limit: full:<runner>[,<runner>] widens to every test, and
// the answer echoes the named runners in order and says it was widened.
func TestMTTV1008RunnerSwitchWidensAndEchoesItsRunners(t *testing.T) {
	group, runs := selectionGroup(t, "full:jest,go")
	result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
	structured, _ := result["structuredContent"].(map[string]any)
	selection, _ := structured["selection"].(map[string]any)
	if failure != nil || structured["selectAll"] != true || structured["scope"] != "FULL" || *runs != 0 ||
		!reflect.DeepEqual(selection["runners"], []any{"jest", "go"}) || selection["value"] != "full:jest,go" {
		t.Fatalf("%#v %#v", structured, failure)
	}
	if detail, _ := structured["detail"].(string); !strings.Contains(detail, `"full:jest,go"`) || !strings.Contains(detail, "widens that to every test") {
		t.Fatalf("detail %q", detail)
	}
}

// MTT-V1-008: a 10 KB value is echoed in at most 256 bytes, cut with the
// truncation marker, in selection.value and in detail. A long runner list
// names runners outside the seven, so it is an unknown value (the narrowing-measurement kill switch)
// and echoes no runners.
func TestMTTV1008EchoedValueIsBounded(t *testing.T) {
	long := "fastest" + strings.Repeat("é", 5*1024)
	var names []string
	for index := range 2000 {
		names = append(names, fmt.Sprintf("runner%d", index))
	}
	runnerList := "full:" + strings.Join(names, ",")
	for _, value := range []string{long, runnerList} {
		if len(value) < 10*1024 {
			t.Fatalf("fixture is %d bytes", len(value))
		}
		group, _ := selectionGroup(t, value)
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
		structured, _ := result["structuredContent"].(map[string]any)
		selection, _ := structured["selection"].(map[string]any)
		echoed, _ := selection["value"].(string)
		marker := fmt.Sprintf("...(truncated, %d bytes)", len(value))
		if failure != nil || len(echoed) > 256 || !strings.HasSuffix(echoed, marker) || !strings.HasPrefix(value, strings.TrimSuffix(echoed, marker)) || !utf8.ValidString(echoed) {
			t.Fatalf("value of %d bytes echoed as %d bytes %q (%#v)", len(value), len(echoed), echoed, failure)
		}
		detail, _ := structured["detail"].(string)
		if !strings.Contains(detail, marker) || strings.Contains(detail, value[:1024]) || len(detail) > 4*256+256 {
			t.Fatalf("detail of %d bytes is not bounded: %.300q", len(detail), detail)
		}
		runners, _ := selection["runners"].([]any)
		if len(runners) != 0 || structured["reason"] != selectionswitch.ReasonUnknownValue {
			t.Fatalf("unknown value echoed runners %q with reason %v", runners, structured["reason"])
		}
		if text := result["content"].([]any)[0].(map[string]any)["text"].(string); strings.Contains(text, value[:1024]) {
			t.Fatal("the text block echoes the raw value")
		}
	}
}

// MTT-V1-008 with the narrowing-measurement kill switch: a runner list naming a runner outside the
// seven is an unknown value: every test is selected with the unknown-value
// reason naming the value and the known runners, no runner is echoed, and the
// planner never runs.
func TestMTTV1008UnknownRunnerNameIsAnUnknownValue(t *testing.T) {
	for _, value := range []string{"full:this-runner-does-not-exist", "full:go,cargo", "full:node-test"} {
		group, runs := selectionGroup(t, value)
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
		structured, _ := result["structuredContent"].(map[string]any)
		selection, _ := structured["selection"].(map[string]any)
		detail, _ := structured["detail"].(string)
		if failure != nil || result["isError"] != false || structured["selectAll"] != true || structured["scope"] != "FULL" ||
			structured["reason"] != selectionswitch.ReasonUnknownValue || *runs != 0 || selection["value"] != value ||
			!reflect.DeepEqual(selection["runners"], []any{}) || !strings.Contains(detail, fmt.Sprintf("%q", value)) || !strings.Contains(detail, "minitest") {
			t.Fatalf("%q: %#v %#v", value, structured, failure)
		}
	}
}

// MTT-V1-008: the 256-byte echo bound holds for the bytes a client decodes.
// Unix allows invalid UTF-8 in an environment value, and JSON turns each
// invalid byte into U+FFFD (three bytes), so 256 0xff bytes, a short invalid
// value and a 10 KB mixed one are each bounded after serialisation. Any value
// that grows past the bound carries the truncation marker.
func TestMTTV1008EchoBoundsInvalidUTF8(t *testing.T) {
	for name, value := range map[string]string{
		"256 0xff":         strings.Repeat("\xff", 256),
		"100 0xff":         strings.Repeat("\xff", 100),
		"10 KB mixed":      strings.Repeat("ok\xfe\xff é", 1200),
		"short invalid":    "fast\xffest",
		"cut rune at 260+": strings.Repeat("a", 258) + "é" + strings.Repeat("\xff", 300),
	} {
		group, runs := selectionGroup(t, value)
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
		if failure != nil || *runs != 0 {
			t.Fatalf("%s: failure %#v runs %d", name, failure, *runs)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			StructuredContent struct {
				Detail    string `json:"detail"`
				Reason    string `json:"reason"`
				Selection struct {
					Value string `json:"value"`
				} `json:"selection"`
			} `json:"structuredContent"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		echoed := decoded.StructuredContent.Selection.Value
		marker := fmt.Sprintf("...(truncated, %d bytes)", len(value))
		// The value as a client decodes it if it were echoed whole.
		whole, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var decodedWhole string
		if err := json.Unmarshal(whole, &decodedWhole); err != nil {
			t.Fatal(err)
		}
		grows := len(decodedWhole) > 256
		if len(echoed) > 256 || !utf8.ValidString(echoed) || decoded.StructuredContent.Reason != selectionswitch.ReasonUnknownValue {
			t.Fatalf("%s: echoed %d bytes %q", name, len(echoed), echoed)
		}
		if grows != strings.HasSuffix(echoed, marker) {
			t.Fatalf("%s: grows=%v but echoed %q", name, grows, echoed)
		}
		if !grows && echoed != decodedWhole {
			t.Fatalf("%s: a value within the bound changed: %q", name, echoed)
		}
		if len(decoded.StructuredContent.Detail) > 4*256+256 || strings.Contains(decoded.StructuredContent.Detail, strings.Repeat("\uFFFD", 100)) {
			t.Fatalf("%s: detail of %d bytes is not bounded", name, len(decoded.StructuredContent.Detail))
		}
	}
	if echoed, _ := echoValue("fast\xffest"); echoed != "fast\uFFFDest" {
		t.Fatalf("short invalid value echoed as %q", echoed)
	}
}
