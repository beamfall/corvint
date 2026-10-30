package main

import (
	"strings"
	"testing"
)

// TestSTRV0062ArgumentRefusalNamesTheFault: STR-V0-062 (V1-2808). An
// unknown flag or a stray positional argument to `corvint test` is refused
// with invalid-arguments, and the message names the flag or argument at
// fault before the usage line, instead of the usage line alone. A flag whose
// required companion is missing or misplaced names the companion flag
// (--confine, --shard-plan), not the usage line alone.
func TestSTRV0062ArgumentRefusalNamesTheFault(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		arguments []string
		want      string
	}{
		"unknown flag":             {[]string{"test", "--out", "x"}, "flag provided but not defined: -out; usage: corvint test --base FULL_COMMIT_ID"},
		"missing value":            {[]string{"test", "--base"}, "flag needs an argument: -base; usage: corvint test"},
		"stray positional":         {[]string{"test", "--base", strings.Repeat("a", 40), "extra"}, "unexpected argument \\\"extra\\\"; usage: corvint test"},
		"ci without confine":       {[]string{"test", "--base", strings.Repeat("a", 40), "--profile", "ci"}, "--profile ci needs --confine ABSOLUTE_PATH"},
		"ci with relative confine": {[]string{"test", "--base", strings.Repeat("a", 40), "--profile", "ci", "--confine", "wrapper"}, "--profile ci needs --confine ABSOLUTE_PATH"},
		"local with confine":       {[]string{"test", "--base", strings.Repeat("a", 40), "--confine", "/abs/wrapper"}, "--confine applies only to --profile ci"},
		"measured without plan":    {[]string{"test", "--base", strings.Repeat("a", 40), "--placement", "measured"}, "--placement measured needs --shard-plan PATH"},
		"plan without measured":    {[]string{"test", "--base", strings.Repeat("a", 40), "--shard-plan", "plan.json"}, "--shard-plan applies only to it"},
	} {
		code, stdout, stderr := runTestPlanCLI(t, c.arguments...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "invalid-arguments") || !strings.Contains(stderr, c.want) {
			t.Errorf("%s: exit=%d stdout=%q stderr=%q lacks %q", name, code, stdout, stderr, c.want)
		}
	}
}
