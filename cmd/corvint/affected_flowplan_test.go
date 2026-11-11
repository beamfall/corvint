// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// flowPlanCLIRepo is the APK fixture (V1-5251, V1-5252) as a committed
// repository: a Go module with a unit, an integration and an undeclared
// package, a Playwright suite of three spec files, flows linking two of them,
// and the flows declaration when one is given. Commit A holds everything with
// unreviewed links, commit B the review anchors, commit C a change to
// src/checkout.ts and lib/lib.go; base is B, so the planned change is C's.
// The discovery receipt is written bound to C.
func flowPlanCLIRepo(t *testing.T, declaration, config string) (root, base string) {
	t.Helper()
	root = t.TempDir()
	shopGit(t, root, "init", "-q")
	files := map[string]string{
		".gitignore":                "discovery.json\n",
		"go.mod":                    "module example.test/m\n",
		"lib/lib.go":                "package lib\n\nfunc Help() string { return \"help\" }\n",
		"lib/lib_test.go":           "package lib\n\nimport \"testing\"\n\nfunc TestHelp(t *testing.T) { Help() }\n",
		"integration/integ_test.go": "package integration\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestIntegration(t *testing.T) { lib.Help() }\n",
		"package.json":              `{"name":"shop","private":true}`,
		"package-lock.json":         `{"name":"shop","lockfileVersion":3}`,
		"playwright.config.ts":      config,
		"e2e/pages/shop.ts":         `export const origin = "http://localhost:3000";`,
		"e2e/checkout.spec.ts":      "import { origin } from \"./pages/shop\";\n\ntest(\"pays\", () => origin);\n",
		"e2e/search.spec.ts":        "import { origin } from \"./pages/shop\";\n\ntest(\"finds\", () => origin);\n",
		"e2e/orphan.spec.ts":        "import { origin } from \"./pages/shop\";\n\ntest(\"wanders\", () => origin);\n",
		"src/cart.ts":               "export function total(n: number): number {\n  return n;\n}\n",
		"src/checkout.ts":           "import { total } from \"./cart\";\n\nexport function pay(n: number): number {\n  return total(n);\n}\n",
		"src/search.ts":             "export function find(q: string): string {\n  return q;\n}\n",
		"e2e-provider.json":         `{"schema":"application-flow-selection-provider/1","flows":"flows","runner_config":"playwright.config.ts","discovery":"discovery.json","global_paths":[],"inventory":[{"path":"e2e/checkout.spec.ts","project":"chromium","test_key":"checkout.spec.ts > pays"},{"path":"e2e/search.spec.ts","project":"chromium","test_key":"search.spec.ts > finds"}]}`,
	}
	if declaration != "" {
		files[appflows.FlowsDeclarationPath] = declaration
	}
	shopWrite(t, root, files)
	shopWrite(t, root, flowPlanIntents(t, ""))
	anchor := shopCommit(t, root, "A: fixture")
	shopWrite(t, root, flowPlanIntents(t, anchor))
	base = shopCommit(t, root, "B: review anchors")
	shopWrite(t, root, map[string]string{
		"src/checkout.ts": "import { total } from \"./cart\";\n\nexport function pay(n: number): number {\n  return total(n) + 1;\n}\n",
		"lib/lib.go":      "package lib\n\nfunc Help() string { return \"help!\" }\n",
	})
	head := shopCommit(t, root, "C: the change")
	writeFlowPlanDiscovery(t, root, head)
	return root, base
}

func flowPlanIntents(t *testing.T, anchor string) map[string]string {
	t.Helper()
	link := func(from, path, key string) appflows.FlowLink {
		target := appflows.LinkTarget{Type: "source", Path: path}
		if key != "" {
			target = appflows.LinkTarget{Type: "test", Path: path, TestKey: key}
		}
		return appflows.FlowLink{From: from, Basis: "declared", Target: target, ReviewedAt: anchor}
	}
	variation := func(id, step, outcome string) appflows.FlowVariation {
		return appflows.FlowVariation{VariationID: id, Preconditions: []string{}, Steps: []string{step}, ObservableFacts: []string{}, Outcomes: []string{outcome}, Projects: []string{"chromium"}}
	}
	intents := []appflows.FlowIntent{
		{Schema: appflows.FlowIntentSchema, FlowID: "checkout", Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
			Steps:      []appflows.FlowStep{{StepID: "pay", Action: "do pay"}},
			Outcomes:   []appflows.FlowOutcome{{OutcomeID: "paid", Behavior: "paid visible", Matcher: "toBeVisible", Locator: "#paid", Value: "paid"}},
			Variations: []appflows.FlowVariation{variation("checkout.happy", "pay", "paid"), variation("checkout.guest", "pay", "paid")},
			Links: []appflows.FlowLink{link("pay", "src/checkout.ts", ""), link("checkout.happy", "e2e/checkout.spec.ts", "checkout.spec.ts > pays"),
				link("checkout.guest", "e2e/checkout.spec.ts", "checkout.spec.ts > pays")}},
		{Schema: appflows.FlowIntentSchema, FlowID: "search", Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{},
			Steps:      []appflows.FlowStep{{StepID: "query", Action: "do query"}},
			Outcomes:   []appflows.FlowOutcome{{OutcomeID: "listed", Behavior: "listed visible", Matcher: "toBeVisible", Locator: "#listed", Value: "listed"}},
			Variations: []appflows.FlowVariation{variation("search.basic", "query", "listed")},
			Links:      []appflows.FlowLink{link("query", "src/search.ts", ""), link("search.basic", "e2e/search.spec.ts", "search.spec.ts > finds")}},
	}
	out := map[string]string{}
	for _, intent := range intents {
		raw, err := json.Marshal(intent)
		if err != nil {
			t.Fatal(err)
		}
		out["flows/"+intent.FlowID+".json"] = string(raw)
	}
	return out
}

func writeFlowPlanDiscovery(t *testing.T, root, revision string) {
	t.Helper()
	config, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typescript.ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	units := []typescript.PlaywrightDiscoveryUnit{}
	for _, spec := range []string{"checkout", "orphan", "search"} {
		units = append(units, typescript.PlaywrightDiscoveryUnit{Project: "chromium", Test: "e2e/" + spec + ".spec.ts"})
	}
	sum := sha256.Sum256(config)
	receipt := typescript.PlaywrightDiscovery{Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])},
		Profile: "playwright-discovery/0", Revision: revision, SourceDigest: digest, Units: units}
	raw, err := jsonv2.Marshal(receipt, jsonv2.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{"discovery.json": string(raw)})
}

const flowPlanCLIDeclaration = `{"profile":"corvint-flows/0","flows":"flows","kinds":[{"path":"lib","kind":"unit"},{"path":"integration","kind":"integration"}],"playwright":{"config":"playwright.config.ts","discovery":"discovery.json"}}`
const flowPlanCLIConfig = `export default { testDir: "e2e", projects: [{ name: "chromium" }] };`

func flowPlanE2E(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	plan, _ := receipt["plan"].(map[string]any)
	e2e, ok := plan["e2e"].(map[string]any)
	if !ok {
		t.Fatalf("plan has no e2e member: %v", plan)
	}
	return e2e
}

func flowPlanE2EPaths(t *testing.T, e2e map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, item := range e2e["selected"].([]any) {
		entry := item.(map[string]any)
		out[entry["path"].(string)] = entry
	}
	return out
}

func flowPlanKinds(t *testing.T, receipt map[string]any) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, item := range receipt["plan"].(map[string]any)["selected"].([]any) {
		entry := item.(map[string]any)
		kind, _ := entry["testKind"].(string)
		out[entry["unitId"].(string)] = kind
	}
	return out
}

// APK-V0-001, APK-V0-002, APK-V0-003, AFU-V1-141 (V1-5251, V1-5252): plain
// `corvint affected --base SHA`, no provider or profile option, returns the
// unit, integration and E2E selections in one receipt, each unit selection
// with its declared kind and each E2E entry with its flow, variation and
// changed path; the unlinked test is selected and reported.
func TestAPKV0001OnePlanForEveryKindViaCLI_V15251(t *testing.T) {
	t.Parallel()
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	kinds := flowPlanKinds(t, receipt)
	if kinds["go:example.test/m/lib"] != "unit" || kinds["go:example.test/m/integration"] != "integration" {
		t.Fatalf("kinds: %v", kinds)
	}
	e2e := flowPlanE2E(t, receipt)
	if e2e["scope"] != "BOUNDED" || e2e["flows"] != "flows" {
		t.Fatalf("e2e: %v", e2e)
	}
	inventory := e2e["inventory"].(map[string]any)
	if inventory["source"] != "discovery" || inventory["state"] != "MATCHED" || inventory["count"] != float64(3) || !reflect.DeepEqual(inventory["unlinked"], []any{"e2e/orphan.spec.ts"}) {
		t.Fatalf("inventory: %v", inventory)
	}
	entries := flowPlanE2EPaths(t, e2e)
	checkout, ok := entries["e2e/checkout.spec.ts"]
	if !ok || checkout["testKind"] != "e2e" || checkout["project"] != "chromium" || !reflect.DeepEqual(checkout["reasons"], []any{"flow-reach"}) || !reflect.DeepEqual(checkout["testKeys"], []any{"checkout.spec.ts > pays"}) {
		t.Fatalf("checkout.spec.ts: ok=%v %v", ok, checkout)
	}
	wantFlows := []any{
		map[string]any{"flow": "checkout", "variation": "checkout.guest", "changedPath": "src/checkout.ts", "target": "src/checkout.ts"},
		map[string]any{"flow": "checkout", "variation": "checkout.happy", "changedPath": "src/checkout.ts", "target": "src/checkout.ts"},
	}
	if !reflect.DeepEqual(checkout["flows"], wantFlows) {
		t.Fatalf("checkout flows: %v", checkout["flows"])
	}
	if orphan, ok := entries["e2e/orphan.spec.ts"]; !ok || !reflect.DeepEqual(orphan["reasons"], []any{"unlinked"}) {
		t.Fatalf("orphan.spec.ts: ok=%v %v", ok, orphan)
	}
	if _, ok := entries["e2e/search.spec.ts"]; ok || len(entries) != 2 {
		t.Fatalf("selected: %v", entries)
	}
}

// APK-V0-005 (V1-5251): the compact and full receipts carry the same e2e
// member and kinds; --format text prints them; and a twin repository without
// the declaration gets the same receipt minus the new members.
func TestAPKV0005MembersAreAdditive_V15251(t *testing.T) {
	t.Parallel()
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	compact, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	full, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--full")
	if code != 0 {
		t.Fatalf("--full exit %d: %s", code, stderr)
	}
	if !reflect.DeepEqual(flowPlanE2E(t, compact), flowPlanE2E(t, full)) || !reflect.DeepEqual(flowPlanKinds(t, compact), flowPlanKinds(t, full)) {
		t.Fatalf("compact and full disagree:\n%v\n%v", flowPlanE2E(t, compact), flowPlanE2E(t, full))
	}
	var text, textErr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base, "--format", "text"}, strings.NewReader(""), &text, &textErr); code != 0 {
		t.Fatalf("--format text exit %d: %s", code, textErr.String())
	}
	for _, want := range []string{"e2e-scope BOUNDED inventory=discovery:3", "e2e e2e/checkout.spec.ts project chromium reasons flow-reach flow checkout variation checkout.guest <- src/checkout.ts flow checkout variation checkout.happy <- src/checkout.ts", "e2e e2e/orphan.spec.ts project chromium reasons unlinked", " kind unit", " kind integration"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("text lacks %q:\n%s", want, text.String())
		}
	}
	// The twin without a declaration: strip the new members and the
	// commit-bound values, and the receipts are the same.
	twin, twinBase := flowPlanCLIRepo(t, "", flowPlanCLIConfig)
	bare, _, stderr, code := runAffectedCLI(t, twin, "--base", twinBase)
	if code != 0 {
		t.Fatalf("twin exit %d: %s", code, stderr)
	}
	if _, present := bare["plan"].(map[string]any)["e2e"]; present {
		t.Fatal("the twin has an e2e member without a declaration")
	}
	for _, item := range bare["plan"].(map[string]any)["selected"].([]any) {
		if _, present := item.(map[string]any)["testKind"]; present {
			t.Fatalf("the twin carries testKind: %v", item)
		}
	}
	strip := func(receipt map[string]any, revision, base string) map[string]any {
		plan := receipt["plan"].(map[string]any)
		delete(plan, "e2e")
		for _, item := range plan["selected"].([]any) {
			delete(item.(map[string]any), "testKind")
		}
		receipt["revision"] = "REVISION"
		receipt["range"].(map[string]any)["base"] = "BASE"
		return receipt
	}
	left, right := strip(compact, compact["revision"].(string), base), strip(bare, bare["revision"].(string), twinBase)
	if !reflect.DeepEqual(left, right) {
		l, _ := json.MarshalIndent(left, "", " ")
		r, _ := json.MarshalIndent(right, "", " ")
		t.Fatalf("declared receipt minus the new members differs from the undeclared one:\n%s\n%s", l, r)
	}
}

// AFU-V1-144 (V1-5252): the plan read derives the inventory without running
// the Playwright config, so a config with side effects leaves no trace.
func TestAFUV1144PlanReadRunsNoProjectCode_V15252(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "config-ran")
	config := "import { writeFileSync } from \"node:fs\";\nwriteFileSync(" + strconvQuote(marker) + ", \"ran\");\nexport default { testDir: \"e2e\", projects: [{ name: \"chromium\" }] };\n"
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, config)
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the plan read ran the Playwright config: %v", err)
	}
	e2e := flowPlanE2E(t, receipt)
	if inventory := e2e["inventory"].(map[string]any); inventory["source"] != "discovery" || inventory["state"] != "MATCHED" {
		t.Fatalf("the config was not read as data: %v", inventory)
	}
}

func strconvQuote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

// AFU-V1-142 (V1-5252): without the receipt the CLI answer is FULL with the
// typed unknown and the known tests, never an empty bounded one.
func TestAFUV1142MissingReceiptViaCLI_V15252(t *testing.T) {
	t.Parallel()
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	if err := os.Remove(filepath.Join(root, "discovery.json")); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	e2e := flowPlanE2E(t, receipt)
	unknown := e2e["unknown"].([]any)
	if e2e["scope"] != "FULL" || len(unknown) != 1 || unknown[0].(map[string]any)["reason"] != "E2E_INVENTORY_UNDERIVABLE" || unknown[0].(map[string]any)["detail"] != "discovery:MISSING" {
		t.Fatalf("e2e: %v", e2e)
	}
	if entries := flowPlanE2EPaths(t, e2e); len(entries) != 2 || entries["e2e/checkout.spec.ts"] == nil || entries["e2e/search.spec.ts"] == nil {
		t.Fatalf("selected: %v", entries)
	}
	// The E2E unknown stays in the e2e member; plan.unknown is untouched.
	for _, item := range receipt["plan"].(map[string]any)["unknown"].([]any) {
		if reason := item.(map[string]any)["reason"].(string); strings.HasPrefix(reason, "E2E_") || reason == "FLOWS_DECLARATION_INVALID" {
			t.Fatalf("an E2E unknown reached plan.unknown: %v", item)
		}
	}
}

// AFU-V1-145 (V1-5252): with --provider the e2e-safe test_selection is still
// written and the e2e member records the provider's precedence and the
// discovered file the provider omits.
func TestAFUV1145ProviderWinsViaCLI_V15252(t *testing.T) {
	t.Parallel()
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--provider", "e2e-provider.json", "--selection-profile", "e2e-safe")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	selection, ok := receipt["advice"].(map[string]any)["test_selection"].(map[string]any)
	if !ok || selection["schema"] != appflows.E2ESafeSchema {
		t.Fatalf("test_selection: %v", receipt["advice"])
	}
	inventory := flowPlanE2E(t, receipt)["inventory"].(map[string]any)
	if inventory["source"] != "provider" || !reflect.DeepEqual(inventory["providerOmits"], []any{"e2e/orphan.spec.ts"}) {
		t.Fatalf("inventory: %v", inventory)
	}
}

// FSV-V1-004 (V1-5410): `corvint affected --base SHA` on a flows declaration
// of a schema version this build does not read selects every E2E test with
// FLOWS_SCHEMA_UNSUPPORTED, naming the found and supported versions in the
// JSON receipt and the text summary, and its unit and integration
// selections are those of a twin without the declaration.
func TestFSVV1004UnsupportedVersionViaCLI_V15410(t *testing.T) {
	t.Parallel()
	declaration := strings.Replace(flowPlanCLIDeclaration, `"corvint-flows/0"`, `"corvint-flows/7"`, 1)
	root, base := flowPlanCLIRepo(t, declaration, flowPlanCLIConfig)
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	e2e := flowPlanE2E(t, receipt)
	unknown, _ := e2e["unknown"].([]any)
	if e2e["scope"] != "FULL" || len(e2e["selected"].([]any)) != 0 || len(unknown) != 1 {
		t.Fatalf("e2e: %v", e2e)
	}
	entry := unknown[0].(map[string]any)
	if entry["reason"] != "FLOWS_SCHEMA_UNSUPPORTED" || entry["detail"] != ".corvint/flows.json version 7; supported 0" || !strings.Contains(entry["hint"].(string), "every E2E test is selected") {
		t.Fatalf("unknown: %v", entry)
	}
	for _, item := range receipt["plan"].(map[string]any)["unknown"].([]any) {
		if reason := item.(map[string]any)["reason"].(string); strings.HasPrefix(reason, "FLOWS_") {
			t.Fatalf("a flows unknown reached plan.unknown: %v", item)
		}
	}
	twin, twinBase := flowPlanCLIRepo(t, "", flowPlanCLIConfig)
	bare, _, stderr, code := runAffectedCLI(t, twin, "--base", twinBase)
	if code != 0 {
		t.Fatalf("twin exit %d: %s", code, stderr)
	}
	if got, want := flowPlanKinds(t, receipt), flowPlanKinds(t, bare); !reflect.DeepEqual(got, want) {
		t.Fatalf("unit and integration selection changed: %v, without a declaration %v", got, want)
	}
	var text, textErr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base, "--format", "text"}, strings.NewReader(""), &text, &textErr); code != 0 {
		t.Fatalf("--format text exit %d: %s", code, textErr.String())
	}
	for _, want := range []string{"e2e-scope FULL inventory=flows:0", "e2e-unknown FLOWS_SCHEMA_UNSUPPORTED .corvint/flows.json version 7; supported 0"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("text lacks %q:\n%s", want, text.String())
		}
	}
}
