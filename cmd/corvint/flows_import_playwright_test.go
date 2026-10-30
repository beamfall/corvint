package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// AFU-V1-130
func TestAFUV1130FlowsHelpSaysTestLinksOnly(t *testing.T) {
	var out, diagnostic bytes.Buffer
	if code := runContext(context.Background(), []string{"help", "flows"}, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	help := strings.Join(strings.Fields(out.String()), " ")
	for _, want := range []string{"A playwright-list import (Playwright's JSON test list) supplies test links only",
		`keyed "path > describe > title"; steps, actors and outcomes are authored by hand`, "reported and skipped, the rest are written, and the command exits 1"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help lacks %q", want)
		}
	}
}

// AFU-V1-128 AFU-V1-129: a playwright list from a config with testDir './tests' imports one flow per
// listed test, keyed by repository path and describe path, so flows map and flows gaps resolve the
// imported links; a re-import reports each test and exits 1.
func TestAFUV1128FlowsCLIPlaywrightListLinksResolve(t *testing.T) {
	root := flowIntentRepo(t)
	tests := filepath.Join(root, "tests")
	if err := os.Mkdir(tests, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tests, "cart.spec.ts"), []byte("test.describe('guest', () => { test('pays', () => {}) })\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir, err := json.Marshal(tests)
	if err != nil {
		t.Fatal(err)
	}
	list := `{"config":{"rootDir":` + string(dir) + `,"projects":[{"name":"chromium","testDir":` + string(dir) + `}]},"suites":[
 {"title":"cart.spec.ts","file":"cart.spec.ts","specs":[],"suites":[
  {"title":"guest","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium"}]}]},
  {"title":"member","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium"}]}]}]}]}`
	source := filepath.Join(t.TempDir(), "list.json")
	if err = os.WriteFile(source, []byte(list), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic := runFlowsCLI(root, "import", "--flows", "flows", "--from", source, "--format", "playwright-list")
	if code != 0 || out != "flows/cart.spec-guest-pays.json\nflows/cart.spec-member-pays.json\n" {
		t.Fatalf("import %d %q %s", code, out, diagnostic)
	}
	// Accept the guest flow and declare its link so flows gaps counts it as test coverage (AFU-V1-009).
	guest := filepath.Join(root, "flows", "cart.spec-guest-pays.json")
	raw, err := os.ReadFile(guest)
	if err != nil {
		t.Fatal(err)
	}
	var intent appflows.FlowIntent
	if err = json.Unmarshal(raw, &intent); err != nil || len(intent.Links) != 1 {
		t.Fatalf("imported intent %v %s", err, raw)
	}
	intent.Proposed, intent.Links[0].Basis = false, "declared"
	if raw, err = json.Marshal(intent); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(guest, raw, 0600); err != nil {
		t.Fatal(err)
	}
	commitFlows(t, root)
	code, out, diagnostic = runFlowsCLI(root, "map", "--flows", "flows", "--path", "tests/cart.spec.ts")
	if code != 0 || !strings.Contains(out, `"flow":"cart.spec-guest-pays"`) || !strings.Contains(out, `"flow":"cart.spec-member-pays"`) {
		t.Fatalf("path lookup %d %s %s", code, out, diagnostic)
	}
	code, out, diagnostic = runFlowsCLI(root, "map", "--flows", "flows", "--test-key", "tests/cart.spec.ts > member > pays")
	if code != 0 || !strings.Contains(out, `"flows":[{"flow":"cart.spec-member-pays","from":"cart.spec-member-pays.spec","basis":"inferred"`) {
		t.Fatalf("test-key lookup %d %s %s", code, out, diagnostic)
	}
	code, out, diagnostic = runFlowsCLI(root, "gaps", "--flows", "flows")
	if code != 0 || !strings.Contains(out, `"code":"no-test","member":"cart.spec-member-pays.spec"`) || strings.Contains(out, `"code":"no-test","member":"cart.spec-guest-pays.spec"`) {
		t.Fatalf("gaps %d %s %s", code, out, diagnostic)
	}
	code, out, diagnostic = runFlowsCLI(root, "import", "--flows", "flows", "--from", source, "--format", "playwright-list")
	if code != 1 || out != "" || !strings.Contains(diagnostic, "imported 0 listed tests; skipped 2:") ||
		!strings.Contains(diagnostic, `skipped "tests/cart.spec.ts > guest > pays": flow cart.spec-guest-pays already exists`) {
		t.Fatalf("re-import %d %q %s", code, out, diagnostic)
	}
}
