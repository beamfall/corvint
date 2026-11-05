// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// CIN-V0-7504: the workflow must use flags this tree's real parser admits.
func TestCINV07504WorkflowPlanFlags(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^          "\$RUNNER_TEMP/corvint-web-plan" --root "\$GITHUB_WORKSPACE/source" test (.*) > "\$RUNNER_TEMP/web-plan.json" \|\| status=\$\?$`).FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("workflow has no plan invocation")
	}
	args := strings.Fields(m[1])
	for i, arg := range args {
		arg = strings.Trim(arg, `"`)
		arg = strings.ReplaceAll(arg, "$BASE", strings.Repeat("a", 40))
		arg = strings.ReplaceAll(arg, "$RUNNER_TEMP", t.TempDir())
		args[i] = arg
	}
	options, err := parseTestOptions(args)
	if err != nil || !options.dryRun || options.profile != "ci" || !filepath.IsAbs(options.confine) {
		t.Fatalf("workflow flags %q: %+v %v", args, options, err)
	}
}
