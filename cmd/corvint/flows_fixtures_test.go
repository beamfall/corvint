package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

// Flows fixtures shared with root tests (affected, impact, MCP parity and the remaining flows
// dispatch/help tests). The handler-level copies live in internal/command/flows.

func runFlowsCLI(root string, args ...string) (int, string, string) {
	var out, diagnostic bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root, "flows"}, args...), bytes.NewReader(nil), &out, &diagnostic)
	return code, out.String(), diagnostic.String()
}

// appmapCLIRepo commits the internal/appmap fixture application into a fresh repository.
func appmapCLIRepo(t *testing.T) string {
	t.Helper()
	return appmapCLIRepoFrom(t, "fixture")
}

// appmapCLIRepoFrom commits one internal/appmap/testdata fixture into a fresh repository.
func appmapCLIRepoFrom(t *testing.T, fixture string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("..", "..", "internal", "appmap", "testdata", fixture)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

func decodeNumbers(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

// shopFixture is the small application behind the map, gaps and impact goldens. Commit A adds the
// sources and intents, B anchors reviews at A, and HEAD (C) changes cart/cart.go and line 4 of
// profile/profile.go.
type shopFixture struct {
	root, evidence string
	a, b, head     string
}

func shopGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false"}, args...)...)
	c.Dir = root
	date := "2026-01-01T00:00:00Z"
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func shopWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func shopCommit(t *testing.T, root, message string) string {
	t.Helper()
	shopGit(t, root, "add", ".")
	shopGit(t, root, "commit", "-qm", message)
	return shopGit(t, root, "rev-parse", "HEAD")
}

func shopLink(from, basis, reviewedAt string, target appflows.LinkTarget) appflows.FlowLink {
	return appflows.FlowLink{From: from, Basis: basis, Target: target, ReviewedAt: reviewedAt}
}

func shopIntents(t *testing.T, anchor string) map[string]string {
	t.Helper()
	d := doccorpus.Digest([]byte("shop"))
	evidence := doccorpus.Anchor{Repository: strings.Repeat("2", 40), Revision: strings.Repeat("b", 40), Path: "flows/shop.md", Blob: strings.Repeat("d", 40),
		SHA256: d, Start: 1, End: 1, SpanSHA256: d, Authority: "external-provider", Kind: "declared", Reason: "fixture"}
	adapter := func(control string) *appflows.FlowAdapter {
		return &appflows.FlowAdapter{Derivation: "declared", Evidence: evidence, RequiredPages: []string{}, NegativeControls: []string{control}, OrderedEvents: []doccorpus.BehaviorEvent{}}
	}
	src := func(path string, start, end int) appflows.LinkTarget {
		return appflows.LinkTarget{Type: "source", Path: path, StartLine: start, EndLine: end}
	}
	test := func(path, key string) appflows.LinkTarget {
		return appflows.LinkTarget{Type: "test", Path: path, TestKey: key}
	}
	assert := func(path, name string) appflows.LinkTarget {
		return appflows.LinkTarget{Type: "assertion", Path: path, Assertion: name}
	}
	intent := func(id string, proposed bool, steps, outcomes []string, variations []appflows.FlowVariation, links []appflows.FlowLink, a *appflows.FlowAdapter) appflows.FlowIntent {
		f := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: id, Revision: 1, Proposed: proposed, Kind: "ui", Actor: "shopper", Preconditions: []string{},
			Steps: []appflows.FlowStep{}, Outcomes: []appflows.FlowOutcome{}, Variations: variations, Links: links, Adapter: a}
		for _, s := range steps {
			f.Steps = append(f.Steps, appflows.FlowStep{StepID: s, Action: "do " + s})
		}
		for _, o := range outcomes {
			f.Outcomes = append(f.Outcomes, appflows.FlowOutcome{OutcomeID: o, Behavior: o + " visible", Matcher: "toBeVisible", Locator: "#" + o, Value: o})
		}
		return f
	}
	variation := func(id string, steps, outcomes, projects []string) appflows.FlowVariation {
		return appflows.FlowVariation{VariationID: id, Preconditions: []string{}, Steps: steps, ObservableFacts: []string{}, Outcomes: outcomes, Projects: projects}
	}
	flows := []appflows.FlowIntent{
		intent("checkout", false, []string{"pay"}, []string{"paid"},
			[]appflows.FlowVariation{variation("checkout.happy", []string{"pay"}, []string{"paid"}, []string{"chromium"})},
			[]appflows.FlowLink{
				shopLink("pay", "declared", anchor, src("checkout/checkout.go", 0, 0)),
				shopLink("checkout.happy", "declared", anchor, test("e2e/checkout.spec.ts", "checkout.spec.ts > pays")),
				shopLink("paid", "declared", anchor, assert("e2e/checkout.spec.ts", "status is paid")),
			}, adapter("checkout.spec.ts > declines")),
		intent("search", false, []string{"query"}, []string{"listed"},
			[]appflows.FlowVariation{variation("search.basic", []string{"query"}, []string{"listed"}, []string{"chromium", "firefox", "webkit"})},
			[]appflows.FlowLink{
				shopLink("query", "declared", anchor, src("search/search.go", 0, 0)),
				shopLink("search.basic", "declared", anchor, test("e2e/search.spec.ts", "search.spec.ts > finds")),
				shopLink("listed", "declared", anchor, assert("e2e/search.spec.ts", "results listed")),
			}, adapter("search.spec.ts > empty")),
		intent("profile", false, []string{"edit", "view"}, []string{"saved", "shown"},
			[]appflows.FlowVariation{variation("profile.edit", []string{"edit"}, []string{"saved"}, []string{}), variation("profile.view", []string{"view"}, []string{"shown"}, []string{})},
			[]appflows.FlowLink{
				shopLink("edit", "declared", anchor, src("profile/profile.go", 3, 4)),
				shopLink("view", "declared", anchor, src("profile/profile.go", 6, 7)),
				shopLink("profile.edit", "declared", anchor, test("e2e/profile.spec.ts", "profile.spec.ts > edits")),
				shopLink("profile.view", "declared", "", test("e2e/profile.spec.ts", "profile.spec.ts > views")),
				shopLink("shown", "declared", anchor, assert("e2e/profile.spec.ts", "profile shown")),
			}, nil),
		intent("wishlist", true, []string{"add"}, []string{"added"},
			[]appflows.FlowVariation{variation("wishlist.add", []string{"add"}, []string{"added"}, []string{})},
			[]appflows.FlowLink{
				shopLink("add", "inferred", "", src("cart/cart.go", 0, 0)),
				shopLink("wishlist.add", "inferred", "", test("", "wishlist.spec.ts > adds")),
			}, nil),
		intent("returns", false, []string{"request"}, []string{"refunded"},
			[]appflows.FlowVariation{variation("returns.basic", []string{"request"}, []string{"refunded"}, []string{})}, []appflows.FlowLink{}, nil),
	}
	files := map[string]string{}
	for _, f := range flows {
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		files["flows/"+f.FlowID+".json"] = string(raw)
	}
	return files
}

func shopRecord(key, project string, source appflows.RunSource, cleanup string, outcomes []string, controls []appflows.NegativeControl) appflows.TestRunEvidence {
	digest := strings.Repeat("a", 64)
	attempts := []appflows.RunAttempt{}
	for i, o := range outcomes {
		attempts = append(attempts, appflows.RunAttempt{Ordinal: i + 1, Outcome: o, AssertionAnchors: []appflows.RunAnchor{}, Attachments: []appflows.RunAttachment{}})
	}
	return appflows.TestRunEvidence{Schema: appflows.RunEvidenceSchema, Authority: appflows.AuthorityIngested, RunID: "run-1", Runner: appflows.RunRunner{Name: "playwright", Version: "1.50.0"},
		Source: source, BuildArtifactDigest: digest, Environment: appflows.RunDigestRef{ID: "ci", Digest: digest}, Fixture: appflows.RunDigestRef{ID: "seed", Digest: digest},
		TestKey: key, Project: project, Attempts: attempts, Cleanup: cleanup, NegativeControls: controls}
}

func newShopFixture(t *testing.T) shopFixture {
	t.Helper()
	root := t.TempDir()
	shopGit(t, root, "init", "-q")
	profile := "package profile\n\n// Edit saves a profile.\nfunc Edit(name string) string { return name }\n\n// View shows a profile.\nfunc View(name string) string { return name }\n"
	sources := map[string]string{
		"go.mod":               "module example.com/shop\n\ngo 1.22\n",
		"cart/cart.go":         "package cart\n\nfunc Total(n int) int { return n }\n",
		"checkout/checkout.go": "package checkout\n\nimport \"example.com/shop/cart\"\n\nfunc Pay(n int) int { return cart.Total(n) }\n",
		"search/search.go":     "package search\n\nfunc Find(q string) string { return q }\n",
		"profile/profile.go":   profile,
		"e2e/checkout.spec.ts": "test('pays', () => {});\ntest('declines', () => {});\n",
		"e2e/search.spec.ts":   "test('finds', () => {});\ntest('empty', () => {});\n",
		"e2e/profile.spec.ts":  "test('edits', () => {});\ntest('views', () => {});\n",
	}
	shopWrite(t, root, sources)
	shopWrite(t, root, shopIntents(t, ""))
	fx := shopFixture{root: root}
	fx.a = shopCommit(t, root, "A: shop and intents")
	shopWrite(t, root, shopIntents(t, fx.a))
	fx.b = shopCommit(t, root, "B: review at A")
	shopWrite(t, root, map[string]string{"cart/cart.go": "package cart\n\nfunc Total(n int) int { return n + 0 }\n",
		"profile/profile.go": strings.Replace(profile, "return name }\n\n// View", "return \"edited \" + name }\n\n// View", 1)})
	fx.head = shopCommit(t, root, "C: change cart and profile edit")
	at := appflows.RunSource{Commit: fx.head, Tree: shopGit(t, root, "rev-parse", "HEAD^{tree}"), Clean: true}
	atB := appflows.RunSource{Commit: fx.b, Tree: shopGit(t, root, "rev-parse", fx.b+"^{tree}"), Clean: true}
	control := func(key, observed string) []appflows.NegativeControl {
		return []appflows.NegativeControl{{TestKey: key, Expected: "failed", Observed: observed}}
	}
	records := []appflows.TestRunEvidence{
		shopRecord("checkout.spec.ts > pays", "chromium", at, "done", []string{"passed"}, control("checkout.spec.ts > declines", "failed")),
		shopRecord("search.spec.ts > finds", "chromium", at, "done", []string{"failed", "passed"}, control("search.spec.ts > empty", "failed")),
		shopRecord("search.spec.ts > finds", "firefox", at, "done", []string{"passed"}, []appflows.NegativeControl{}),
		shopRecord("search.spec.ts > finds", "webkit", at, "failed", []string{"passed"}, control("search.spec.ts > empty", "failed")),
		shopRecord("profile.spec.ts > edits", "", atB, "done", []string{"passed"}, []appflows.NegativeControl{}),
		{Schema: appflows.RunEvidenceSchema, Authority: appflows.AuthorityStatic, TestKey: "profile.spec.ts > views", Attempts: []appflows.RunAttempt{}, Cleanup: "not-declared", NegativeControls: []appflows.NegativeControl{}},
	}
	var lines bytes.Buffer
	for _, r := range records {
		line, err := appflows.EncodeRunEvidence(r)
		if err != nil {
			t.Fatal(err)
		}
		lines.Write(line)
	}
	fx.evidence = filepath.Join(t.TempDir(), "runs.jsonl")
	if err := os.WriteFile(fx.evidence, lines.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return fx
}

// inferCLIRepo commits a Rails resource router and one Playwright spec, and builds its map.
func inferCLIRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"appmap.json":              `{"schema":"application-map-manifest/0","app":"shop","routers":[{"path":"config/routes.rb","dialect":"rails-routes/0"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`,
		"e2e/pages/orders.page.ts": "export const orders = '/orders';\n",
		"config/routes.rb":         "Rails.application.routes.draw do\n  resources :orders\nend\n",
		"e2e/specs/ship.spec.ts": "import { test, expect } from '@playwright/test';\n" +
			"test('ships', async ({ page }) => {\n  await page.goto('/orders');\n  await page.getByRole('button', { name: 'Ship' }).click();\n" +
			"  await page.goto('/orders/1');\n  await expect(page.getByText('Shipped')).toBeVisible();\n});\n",
		"flows/retired.json": `{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":[]}` + "\n",
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
	}
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	filename := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(filename, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, filename
}

func maintenanceArgs() []string {
	return []string{"docs", "maintain", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/claims.json", "--docs-root", "docs"}
}
