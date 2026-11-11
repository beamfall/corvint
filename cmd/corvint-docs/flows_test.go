// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	flowscmd "github.com/Beamfall/corvint/internal/command/flows"
)

// Handler-level flows tests live in internal/command/flows. These are here because they
// exercise this binary's dispatch, its `how` alias, its help and its command tree
// (decision 0595, CMS-V0-912).

// howFixture commits a Rails resource router and one Playwright spec, and builds its map.
func howFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	cemWrite(t, root, "appmap.json", `{"schema":"application-map-manifest/0","app":"shop","routers":[{"path":"config/routes.rb","dialect":"rails-routes/0"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`)
	cemWrite(t, root, "e2e/pages/orders.page.ts", "export const orders = '/orders';\n")
	cemWrite(t, root, "config/routes.rb", "Rails.application.routes.draw do\n  resources :orders\nend\n")
	cemWrite(t, root, "e2e/specs/ship.spec.ts", "import { test, expect } from '@playwright/test';\n"+
		"test('ships', async ({ page }) => {\n  await page.goto('/orders');\n  await page.getByRole('button', { name: 'Ship' }).click();\n"+
		"  await page.goto('/orders/1');\n  await expect(page.getByText('Shipped')).toBeVisible();\n});\n")
	cemWrite(t, root, "flows/retired.json", `{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":[]}`+"\n")
	cemGit(t, root, "init", "-q")
	cemGit(t, root, "add", "-A")
	cemGit(t, root, "commit", "-qm", "fixture")
	code, out, diag := runCLI(t, "--root", root, "flows", "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	filename := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(filename, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, filename
}

// CMS-V0-912: the flows group and how are dispatched and documented by corvint-docs, and help
// needs no repository.
func TestCMSV0912FlowsHelpDoesNotRequireRepository(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"help", "flows"}, "application-flow-report/0"},
		{[]string{"flows", "--help"}, "application-flow-report/0"},
		{[]string{"--root", "/definitely/not/a/repository", "flows", "record", "--help"}, "application-flow-report/0"},
		{[]string{"help", "how"}, "site-knowledge-how/0"},
		{[]string{"how", "--help"}, "site-knowledge-how/0"},
		{[]string{"--help"}, "corvint-docs [--root PATH] flows [SUBCOMMAND] [OPTIONS]"},
		{[]string{"--help"}, "corvint-docs [--root PATH] how [OPTIONS] GOAL"},
	} {
		code, out, diag := runCLI(t, test.args...)
		if code != 0 || diag != "" || !strings.Contains(out, test.want) {
			t.Fatalf("%q: exit %d stderr %q, stdout lacks %q", test.args, code, diag, test.want)
		}
		if strings.Contains(out, "\n  corvint [--root PATH] ") {
			t.Fatalf("%q: a synopsis spells the corvint binary, not corvint-docs", test.args)
		}
	}
	if code, _, diag := runCLI(t, "help", "nope"); code != 2 || !strings.Contains(diag, "'flows', 'how'") {
		t.Fatalf("unknown help topic does not list flows and how: %d %s", code, diag)
	}
}

// CAP-V0-007, CMS-V0-912: every flows subcommand the handlers dispatch is a `flows NAME` node of
// this binary's command tree, so none is reachable without a synopsis in its help.
func TestCMSV0912EveryFlowsSubcommandIsInTheCommandTree(t *testing.T) {
	nodes := map[string]bool{}
	for _, node := range commandTree().Nodes {
		nodes[strings.Join(node.Path, " ")] = true
	}
	names := flowscmd.Subcommands()
	if len(names) == 0 {
		t.Fatal("no flows subcommands")
	}
	for _, name := range names {
		if !nodes["flows "+name] {
			t.Errorf("flows %s is dispatched but is not in corvint-docs help --json", name)
		}
	}
	if !slices.Contains(names, "test-evidence") {
		t.Error("test-evidence is not a registered subcommand; its refusal would fall through to the manifest form")
	}
}

// SKH-V0-810: `corvint-docs how` is a top-level alias of `flows appmap how`: the same flags
// give byte-identical stdout and the same refusal codes, and root help lists it.
func TestSKHV0810HowAlias(t *testing.T) {
	root, filename := howFixture(t)
	flags := []string{"--map", filename, "--source", "e2e", "--condition", "an order exists", "--full", "ship an order"}
	wantCode, want, _ := runCLI(t, append([]string{"--root", root, "flows", "appmap", "how"}, flags...)...)
	code, got, diag := runCLI(t, append([]string{"--root", root, "how"}, flags...)...)
	if wantCode != 0 || code != 0 || got != want || got == "" {
		t.Fatalf("alias = %d %q (%s), nested = %d %q", code, got, diag, wantCode, want)
	}
	for _, args := range [][]string{{"--map", filename, "--source", "e2e"}, {"--source", "e2e", "ship"}} {
		nested, _, nestedDiag := runCLI(t, append([]string{"--root", root, "flows", "appmap", "how"}, args...)...)
		alias, _, aliasDiag := runCLI(t, append([]string{"--root", root, "how"}, args...)...)
		if alias != 2 || nested != 2 || aliasDiag != nestedDiag {
			t.Fatalf("%v exit alias=%d nested=%d, want 2 and one refusal: %s / %s", args, alias, nested, aliasDiag, nestedDiag)
		}
	}
	if code, out, _ := runCLI(t, "help"); code != 0 || !strings.Contains(out, "corvint-docs [--root PATH] how ") {
		t.Fatalf("root help does not list how: %d %s", code, out)
	}
}

// TestFlowsImpactUnknownBaseUsesRootRefusal pins this binary's wiring of flowsEnv.BaseRefusal:
// the flows package tests use a stub Env, so only this end-to-end dispatch catches a miswired or
// nil refusal (AFU-V1 impact keeps the affected-revision refusal envelope).
func TestFlowsImpactUnknownBaseUsesRootRefusal(t *testing.T) {
	root := taskContextRepository(t)
	cemWrite(t, root, "flows/retired.json", `{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":[]}`+"\n")
	cemGit(t, root, "add", "-A")
	cemGit(t, root, "commit", "-qm", "flows")
	base := strings.Repeat("0", 40)
	code, out, diagnostic := runCLI(t, "--root", root, "flows", "impact", "--flows", "flows", "--base", base)
	if code != 2 || out != "" {
		t.Fatalf("code=%d out=%q diagnostic=%s", code, out, diagnostic)
	}
	for _, want := range []string{"unsupported-affected-revision", "base " + base + " is not a commit in this repository", "affected.use-commit-base"} {
		if !strings.Contains(diagnostic, want) {
			t.Fatalf("diagnostic lacks %q: %s", want, diagnostic)
		}
	}
}

// CMS-V0-914: `corvint-docs flows test-evidence` refuses with a pointer to the corvint verb.
func TestCMSV0914TestEvidenceRefusalIsReachable(t *testing.T) {
	code, out, diag := runCLI(t, "--root", t.TempDir(), "flows", "test-evidence", "--flows", "flows", "a.go")
	if code != 2 || out != "" || !strings.Contains(diag, `"code": "invalid-arguments"`) || !strings.Contains(diag, flowscmd.TestEvidenceRefusal) {
		t.Fatalf("%d %q %s", code, out, diag)
	}
}

// AFU-V1-130
func TestAFUV1130FlowsHelpSaysTestLinksOnly(t *testing.T) {
	code, out, diagnostic := runCLI(t, "help", "flows")
	if code != 0 {
		t.Fatal(code, diagnostic)
	}
	help := strings.Join(strings.Fields(out), " ")
	for _, want := range []string{"A playwright-list import (Playwright's JSON test list) supplies test links only",
		`keyed "path > describe > title"; steps, actors and outcomes are authored by hand`, "reported and skipped, the rest are written, and the command exits 1"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help lacks %q", want)
		}
	}
}

// AFU-V1-904: help shows the run header unbracketed and states each value format.
func TestAFUV1904Help(t *testing.T) {
	code, out, diagnostic := runCLI(t, "help", "flows")
	if code != 0 {
		t.Fatal(code, diagnostic)
	}
	help := strings.Join(strings.Fields(out), " ")
	for _, want := range []string{
		"--from FILE --run-id ID --runner-version VERSION --source-commit SHA --source-tree SHA [--source-clean] --build-artifact-digest DIGEST --environment-id ID --environment-digest DIGEST --fixture-id ID --fixture-digest DIGEST [--cleanup STATE]",
		"Every --format, playwright-json included, requires the run header",
		"bare 64-hex sha256 digests (no sha256: prefix)", "--cleanup is done, failed or not-declared",
		"One refusal (exit 2, invalid-arguments) names every missing or invalid header flag with its format",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("flows help lacks %q", want)
		}
	}
	if strings.Contains(help, "[--run-id ID]") {
		t.Fatal("flows help still brackets --run-id as optional")
	}
}
