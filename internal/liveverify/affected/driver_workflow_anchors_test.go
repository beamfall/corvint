// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"strings"
	"testing"
)

// AZU-V0-214 (V1-4999): a workflow that reuses steps through YAML anchors and
// aliases is inside the driver's subset. An alias step carries the anchored
// step's text, so its run commands are the job's edges; an alias to a name
// the file never anchors, or a redefined anchor, keeps the file unparsable.
func TestDriverWorkflowAnchorsAndAliases_AZU_V0_214(t *testing.T) {
	body := strings.Join([]string{
		"name: anchors",
		"on: push",
		"jobs:",
		"  first:",
		"    runs-on: ubuntu-latest",
		"    steps:",
		"      - &checkout",
		"        uses: actions/checkout@v4",
		"      - &check",
		"        name: Check",
		"        run: script/check.sh",
		"      - &inline name: Inline",
		"        run: script/inline.sh",
		"      - name: Own",
		"        run: &own go test ./pkg/...",
		"  second:",
		"    runs-on: ubuntu-latest",
		"    steps:",
		"      - *checkout",
		"      - *check",
		"      - *inline",
		"      - name: Repeat",
		"        run: *own",
		"",
	}, "\n")
	workflow := parseDriverWorkflow(".github/workflows/anchors.yml", []byte(body))
	if workflow.unparsable {
		t.Fatal("anchored workflow is unparsable")
	}
	second := workflow.root.get("jobs").get("second").get("steps")
	if second == nil || len(second.items) != 4 {
		t.Fatalf("second job steps: %+v", second)
	}
	for index, want := range []string{"", "script/check.sh", "script/inline.sh", "go test ./pkg/..."} {
		if got := second.items[index].get("run").text(); got != want {
			t.Errorf("step %d run %q, want %q", index, got, want)
		}
	}
	if got := second.items[0].get("uses").text(); got != "actions/checkout@v4" {
		t.Errorf("alias step uses %q", got)
	}
	// The anchored steps are parsed once; the first job keeps them too.
	first := workflow.root.get("jobs").get("first").get("steps")
	if first == nil || len(first.items) != 4 || first.items[1].get("run").text() != "script/check.sh" {
		t.Errorf("first job steps: %+v", first)
	}
	for name, broken := range map[string]string{
		"undefined alias":  strings.Replace(body, "      - *check\n", "      - *missing\n", 1),
		"redefined anchor": strings.Replace(body, "      - &inline name: Inline", "      - &check name: Inline", 1),
		"alias with text":  strings.Replace(body, "      - *check\n", "      - *check extra\n", 1),
		"anchored alias":   strings.Replace(body, "        run: *own\n", "        run: &again *own\n", 1),
	} {
		if !parseDriverWorkflow(".github/workflows/anchors.yml", []byte(broken)).unparsable {
			t.Errorf("%s: parsed although outside the subset", name)
		}
	}
}
