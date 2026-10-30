package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// gh1640Config is the GitHub #1640 shape: two projects whose shared use block reads the browser
// binary from an environment variable.
func gh1640Config(setup, expression string) string {
	return "import { defineConfig, devices } from '@playwright/test';\n" + setup + "\nexport default defineConfig({\n  testDir: './tests',\n  use: { launchOptions: { executablePath: " + expression + " } },\n  projects: [\n    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },\n    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },\n  ],\n});\n"
}

func gh1640Digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func gh1640Resolve(t *testing.T, config string, inputs ...PlaywrightEnvironmentInput) []PlaywrightProject {
	t.Helper()
	projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config)
	if len(unknown) != 0 || len(projects) != 2 {
		t.Fatalf("projects=%+v unknown=%+v", projects, unknown)
	}
	resolvePlaywrightExecutables(t.TempDir(), projects, inputs, "playwright.config.ts", config)
	return projects
}

// AFP-V0-383: a recorded variable carries its name, source, state, value and the value's SHA-256;
// an UNSET variable has an empty value and digest.
func TestPlaywrightEnvironmentInputRecord_GH1640(t *testing.T) {
	set := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentArgument, "/opt/chrome/chrome", true)
	if set.State != PlaywrightEnvironmentSet || set.Value != "/opt/chrome/chrome" || set.Source != PlaywrightEnvironmentArgument || set.SHA256 != gh1640Digest("/opt/chrome/chrome") {
		t.Fatalf("set record: %+v", set)
	}
	if again := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentArgument, "/opt/chrome/chrome", true); again != set {
		t.Fatalf("record is not deterministic: %+v vs %+v", again, set)
	}
	if other := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentArgument, "/opt/other", true); other.SHA256 == set.SHA256 {
		t.Fatal("different values share a digest")
	}
	unset := NewPlaywrightEnvironmentInput("CHROME", PlaywrightEnvironmentInvocation, "ignored", false)
	if unset != (PlaywrightEnvironmentInput{Name: "CHROME", Source: PlaywrightEnvironmentInvocation, State: PlaywrightEnvironmentUnset}) {
		t.Fatalf("unset record: %+v", unset)
	}
	sorted := canonicalPlaywrightEnvironment([]PlaywrightEnvironmentInput{{Name: "B"}, {Name: "A"}})
	if sorted[0].Name != "A" || sorted[1].Name != "B" {
		t.Fatalf("not sorted: %+v", sorted)
	}
	for name, valid := range map[string]bool{"PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH": true, "_x1": true, "": false, "1X": false, "A-B": false, "A=B": false} {
		if ValidPlaywrightEnvironmentName(name) != valid {
			t.Fatalf("ValidPlaywrightEnvironmentName(%q) != %v", name, valid)
		}
	}
}

// AFP-V0-384, AFP-V0-385: the modelled process.env forms resolve against the recorded value; an
// unset bare read is undefined, so Playwright launches its bundled browser.
func TestPlaywrightExecutableResolvesFromRecordedEnvironment_GH1640(t *testing.T) {
	const name = "PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH"
	unset := NewPlaywrightEnvironmentInput(name, PlaywrightEnvironmentInvocation, "", false)
	set := NewPlaywrightEnvironmentInput(name, PlaywrightEnvironmentArgument, "/opt/chrome/chrome", true)
	empty := NewPlaywrightEnvironmentInput(name, PlaywrightEnvironmentArgument, "", true)
	for _, row := range []struct {
		label, expression string
		input             PlaywrightEnvironmentInput
		want              PlaywrightExecutable
	}{
		{"bare unset is bundled", "process.env." + name, unset, PlaywrightExecutable{Bundled: true, Variable: name}},
		{"bracket unset is bundled", "process.env['" + name + "']", unset, PlaywrightExecutable{Bundled: true, Variable: name}},
		{"bare set is the path", "process.env." + name, set, PlaywrightExecutable{Path: "/opt/chrome/chrome", Variable: name}},
		{"nullish unset takes the fallback", "process.env." + name + " ?? '/usr/bin/chromium'", unset, PlaywrightExecutable{Path: "/usr/bin/chromium", Variable: name}},
		{"nullish set keeps the value", "process.env." + name + " ?? '/usr/bin/chromium'", set, PlaywrightExecutable{Path: "/opt/chrome/chrome", Variable: name}},
		{"or empty takes the fallback", "process.env." + name + " || '/usr/bin/chromium'", empty, PlaywrightExecutable{Path: "/usr/bin/chromium", Variable: name}},
		{"or unset takes the fallback", "process.env." + name + " || '/usr/bin/chromium'", unset, PlaywrightExecutable{Path: "/usr/bin/chromium", Variable: name}},
	} {
		t.Run(row.label, func(t *testing.T) {
			for _, project := range gh1640Resolve(t, gh1640Config("", row.expression), row.input) {
				if len(project.execution) != 0 || project.Executable == nil || *project.Executable != row.want {
					t.Fatalf("%s: execution=%+v executable=%+v want %+v", project.Name, project.execution, project.Executable, row.want)
				}
			}
		})
	}
}

// AFP-V0-386, AFP-V0-387: a variable the plan did not observe, an empty evaluated value, a writer
// the config can run first, or an unmodelled expression keeps the EXECUTION unknown, and its
// detail names the variable and what is missing.
func TestPlaywrightExecutableUnknownKeepsReason_GH1640(t *testing.T) {
	const name = "CHROME_PATH"
	unset := NewPlaywrightEnvironmentInput(name, PlaywrightEnvironmentInvocation, "", false)
	empty := NewPlaywrightEnvironmentInput(name, PlaywrightEnvironmentArgument, "", true)
	other := NewPlaywrightEnvironmentInput("OTHER", PlaywrightEnvironmentArgument, "x", true)
	for _, row := range []struct {
		label, setup, expression string
		inputs                   []PlaywrightEnvironmentInput
		detail                   string
	}{
		{"no input", "", "process.env." + name, nil, "pass --playwright-env " + name + " to record it"},
		{"another variable", "", "process.env." + name, []PlaywrightEnvironmentInput{other}, "which this plan did not observe"},
		{"empty value", "", "process.env." + name, []PlaywrightEnvironmentInput{empty}, "evaluates to an empty string"},
		{"empty fallback", "", "process.env." + name + " ?? ''", []PlaywrightEnvironmentInput{unset}, "evaluates to an empty string"},
		{"dotenv module", "import 'dotenv/config';", "process.env." + name, []PlaywrightEnvironmentInput{unset}, `the config loads "dotenv/config"`},
		{"global setup", "const globalSetup = './setup';", "process.env." + name, []PlaywrightEnvironmentInput{unset}, "globalSetup"},
		{"unmodelled expression", "", "process.env." + name + " ? process.env." + name + " : undefined", []PlaywrightEnvironmentInput{unset}, ""},
	} {
		t.Run(row.label, func(t *testing.T) {
			for _, project := range gh1640Resolve(t, gh1640Config(row.setup, row.expression), row.inputs...) {
				if project.Executable != nil || len(project.execution) != 1 || project.execution[0].Reason != PlaywrightUnknownBrowserExecutable || project.execution[0].Axis != PlaywrightAxisExecution {
					t.Fatalf("%s: execution=%+v executable=%+v", project.Name, project.execution, project.Executable)
				}
				if !strings.Contains(project.execution[0].Detail, row.detail) {
					t.Fatalf("%s: detail %q lacks %q", project.Name, project.execution[0].Detail, row.detail)
				}
			}
		})
	}
}

// AFP-V0-383, AFP-V0-385: the static plan records the named variables and each project's
// resolution, and a resolved executable leaves no execution unknown in the plan.
func TestPlaywrightStaticPlanRecordsEnvironment_GH1640(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", gh1640Config("", "process.env.CHROME_PATH"))
	write(t, root, "tests/a.spec.ts", "import { test } from '@playwright/test';\ntest('a', async () => {});\n")
	inputs := []PlaywrightEnvironmentInput{NewPlaywrightEnvironmentInput("CHROME_PATH", PlaywrightEnvironmentInvocation, "", false)}
	plan, _, err := selectPlaywrightStaticDraft(root, "playwright.config.ts", []string{"tests/a.spec.ts"}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Environment, inputs) {
		t.Fatalf("environment=%+v", plan.Environment)
	}
	for _, project := range plan.Projects {
		if project.Executable == nil || !project.Executable.Bundled {
			t.Fatalf("%s: executable=%+v", project.Name, project.Executable)
		}
	}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == PlaywrightUnknownBrowserExecutable {
			t.Fatalf("resolved executable left an unknown: %+v", plan.Unknown)
		}
	}
	unnamed, _, err := selectPlaywrightStaticDraft(root, "playwright.config.ts", []string{"tests/a.spec.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if unnamed.Environment != nil || !slices.ContainsFunc(unnamed.Unknown, func(unknown PlaywrightUnknown) bool {
		return unknown.Reason == PlaywrightUnknownBrowserExecutable && strings.Contains(unknown.Detail, "--playwright-env CHROME_PATH")
	}) {
		t.Fatalf("unnamed: environment=%+v unknown=%+v", unnamed.Environment, unnamed.Unknown)
	}
}

// AFP-V0-388: without a discovery receipt the unknown names the missing input and how to write it.
func TestPlaywrightMissingDiscoveryNamesInput_GH1640(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", gh1640Config("", "undefined"))
	write(t, root, "tests/a.spec.ts", "import { test } from '@playwright/test';\ntest('a', async () => {});\n")
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/a.spec.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || len(plan.Selected) != 0 || !slices.ContainsFunc(plan.Unknown, func(unknown PlaywrightUnknown) bool {
		return unknown.Reason == "playwright:discovery-unproven" && strings.HasPrefix(unknown.Detail, "MISSING: no --playwright-discovery receipt") && strings.Contains(unknown.Detail, "corvint affected discovery")
	}) {
		t.Fatalf("scope=%s selected=%+v unknown=%+v", plan.Scope, plan.Selected, plan.Unknown)
	}
}
