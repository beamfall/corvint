package main

import (
	"bytes"
	json "encoding/json/v2"
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
		{[]string{"--snapshot", "s.json", "--playwright-config", "p.ts", "--playwright-env", "A=1"}, "--playwright-env cannot be combined with --snapshot"},
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

// AFP-V0-383, AFP-V0-385: end to end, `affected --playwright-config --playwright-env NAME=VALUE`
// records the variable in plan.environment and resolves each project's computed executablePath,
// so the command must reach SelectPlaywrightWithEnvironment with the parsed inputs.
func TestAffectedPlaywrightEnvResolvesExecutableEndToEnd_GH1640(t *testing.T) {
	root := t.TempDir()
	writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	writePlaywrightCLIFile(t, root, "playwright.config.ts", "import { defineConfig, devices } from '@playwright/test';\nexport default defineConfig({\n  testDir: './tests',\n  use: { launchOptions: { executablePath: process.env.GH1640_CLI_CHROME } },\n  projects: [\n    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },\n    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },\n  ],\n});\n")
	writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "page"`)
	writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; import { page } from "../src/page"; test("page", () => page)`)
	affectedGit(t, root, "init", "-q")
	affectedGit(t, root, "config", "user.email", "corvint@example.test")
	affectedGit(t, root, "config", "user.name", "Corvint Test")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "fixture")
	writePlaywrightCLIFile(t, root, "src/page.ts", `export const page = "changed"`)

	run := func(extra ...string) map[string]any {
		t.Helper()
		var stdout, stderr bytes.Buffer
		arguments := append([]string{"--root", root, "affected", "--playwright-config", "playwright.config.ts"}, extra...)
		if code := runContext(t.Context(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit=%d stderr=%s", extra, code, stderr.String())
		}
		var receipt map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		return receipt["plan"].(map[string]any)
	}
	plan := run("--playwright-env", "GH1640_CLI_CHROME=/opt/cli/chrome")
	environment, _ := plan["environment"].([]any)
	want := typescript.NewPlaywrightEnvironmentInput("GH1640_CLI_CHROME", typescript.PlaywrightEnvironmentArgument, "/opt/cli/chrome", true)
	if len(environment) != 1 {
		t.Fatalf("plan.environment=%v", plan["environment"])
	}
	if record := environment[0].(map[string]any); record["name"] != want.Name || record["source"] != want.Source || record["state"] != want.State || record["value"] != want.Value || record["sha256"] != want.SHA256 {
		t.Fatalf("plan.environment[0]=%v want %+v", record, want)
	}
	projects, _ := plan["projects"].([]any)
	if len(projects) != 2 {
		t.Fatalf("plan.projects=%v", plan["projects"])
	}
	for _, raw := range projects {
		project := raw.(map[string]any)
		executable, _ := project["executable"].(map[string]any)
		if executable["path"] != "/opt/cli/chrome" || executable["variable"] != "GH1640_CLI_CHROME" || executable["bundled"] != false {
			t.Fatalf("project %v executable=%v", project["name"], project["executable"])
		}
	}
	for _, raw := range plan["unknown"].([]any) {
		if unknown := raw.(map[string]any); unknown["reason"] == typescript.PlaywrightUnknownBrowserExecutable {
			t.Fatalf("executable unknown kept: %v", unknown)
		}
	}

	without := run()
	if _, present := without["environment"]; present {
		t.Fatalf("plan.environment present without --playwright-env: %v", without["environment"])
	}
	if !strings.Contains(stringifyPlaywrightUnknowns(without), "pass --playwright-env GH1640_CLI_CHROME to record it") {
		t.Fatalf("missing-input detail absent: %v", without["unknown"])
	}
}

func stringifyPlaywrightUnknowns(plan map[string]any) string {
	var details []string
	for _, raw := range plan["unknown"].([]any) {
		if detail, ok := raw.(map[string]any)["detail"].(string); ok {
			details = append(details, detail)
		}
	}
	return strings.Join(details, "\n")
}
