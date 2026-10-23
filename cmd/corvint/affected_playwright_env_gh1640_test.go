package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// AFP-V0-382: --playwright-env takes NAME=VALUE or NAME, names each variable once, requires
// --playwright-config, and reads only the variables it names from the invoking environment.
func TestAffectedPlaywrightEnvFlag_GH1640(t *testing.T) {
	invocation, err := parseAffectedOptions([]string{"--playwright-config", "playwright.config.ts", "--playwright-env", "B_PATH=/opt/b", "--playwright-env", "A_PATH"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(invocation.PlaywrightEnv, []string{"B_PATH=/opt/b", "A_PATH"}) {
		t.Fatalf("PlaywrightEnv=%v", invocation.PlaywrightEnv)
	}
	for _, row := range []struct {
		arguments []string
		message   string
	}{
		{[]string{"--playwright-env", "A_PATH"}, "--playwright-env requires --playwright-config"},
		{[]string{"--playwright-config", "p.ts", "--playwright-env", "A=1", "--playwright-env", "A"}, "names A more than once"},
		{[]string{"--playwright-config", "p.ts", "--playwright-env", "1A"}, "NAME=VALUE or NAME"},
		{[]string{"--playwright-config", "p.ts", "--playwright-env", "=x"}, "NAME=VALUE or NAME"},
	} {
		if _, err := parseAffectedOptions(row.arguments); err == nil || !strings.Contains(err.Error(), row.message) {
			t.Fatalf("%v: err=%v want %q", row.arguments, err, row.message)
		}
	}

	t.Setenv("GH1640_SET", "/usr/bin/chromium")
	t.Setenv("GH1640_SHADOWED", "from-environment")
	inputs := affectedPlaywrightEnvironment([]string{"GH1640_SET", "GH1640_UNSET_NEVER_EXPORTED", "GH1640_SHADOWED=from-argument", "GH1640_EMPTY="})
	want := []typescript.PlaywrightEnvironmentInput{
		typescript.NewPlaywrightEnvironmentInput("GH1640_SET", typescript.PlaywrightEnvironmentInvocation, "/usr/bin/chromium", true),
		typescript.NewPlaywrightEnvironmentInput("GH1640_UNSET_NEVER_EXPORTED", typescript.PlaywrightEnvironmentInvocation, "", false),
		typescript.NewPlaywrightEnvironmentInput("GH1640_SHADOWED", typescript.PlaywrightEnvironmentArgument, "from-argument", true),
		typescript.NewPlaywrightEnvironmentInput("GH1640_EMPTY", typescript.PlaywrightEnvironmentArgument, "", true),
	}
	if !slices.Equal(inputs, want) {
		t.Fatalf("inputs=%+v want %+v", inputs, want)
	}
}
