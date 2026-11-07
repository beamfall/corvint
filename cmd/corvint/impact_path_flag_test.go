package main

import (
	"reflect"
	"strings"
	"testing"
)

// TestImpactPathFlagIsPositionalAlias_AFPV07921 covers V1-5045: impact takes
// --path PATH, repeatable and attached or separate, as affected does, with the
// same parsed paths as the positional form, and a --path without a value names
// the positional form.
func TestImpactPathFlagIsPositionalAlias_AFPV07921(t *testing.T) {
	t.Parallel()
	positional, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"pkg/a.go", "pkg/b.go", "pkg/c.go"}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	flagged, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"--path", "pkg/a.go", "--path=pkg/b.go", "pkg/c.go"}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(flagged.impactPaths, positional.impactPaths) || !reflect.DeepEqual(flagged.impactRawPaths, positional.impactRawPaths) {
		t.Fatalf("--path paths = %q raw %q, want positional %q raw %q", flagged.impactPaths, flagged.impactRawPaths, positional.impactPaths, positional.impactRawPaths)
	}
	for name, arguments := range map[string][]string{
		"missing value":       {"pkg/a.go", "--path"},
		"option as the value": {"--path", "--limit", "3"},
	} {
		_, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, arguments, "darwin")
		if err == nil || !strings.Contains(err.Error(), "argument --path: expected one argument (or pass PATH positionally: corvint impact PATH...)") {
			t.Errorf("%s: err = %v, want the --path refusal naming the positional form", name, err)
		}
	}
	if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"--paths", "pkg/a.go"}, "darwin"); err == nil || !strings.Contains(err.Error(), "unrecognized arguments: --paths") {
		t.Errorf("--paths: err = %v, want unrecognized arguments", err)
	}
}
