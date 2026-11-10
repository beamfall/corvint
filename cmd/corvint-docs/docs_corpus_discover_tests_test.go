package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const discoverTestsCLISpec = `RSpec.describe Cart do
  before { visit '/cart' }

  it 'adds an item', :slow do
    post '/cart/items'
    expect(response).to have_http_status(:created)
  end
end
`

// TestDCXV1370DocsCorpusDiscoverTestsCLI pins DCX-V1-370 at the command: `docs corpus
// discover-tests` prints the generated, revision-pinned record byte-identically across runs,
// refuses an invalid runner or revision with a typed corpus code, and is named in docs help.
func TestDCXV1370DocsCorpusDiscoverTestsCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	cemWrite(t, root, "spec/cart_spec.rb", discoverTestsCLISpec)
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "specs")
	revision := cemGit(t, root, "rev-parse", "HEAD")
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "discover-tests", "--runner", "rspec", "--revision", revision, "--path", "spec")
	if code != 0 {
		t.Fatalf("discover-tests = %d %s", code, stderr)
	}
	var doc struct {
		Schema, Trust, Revision string
		Examples                []struct {
			Identity     struct{ Example string }
			Expectations []struct{ Matcher string }
			Navigations  []struct{ Kind, Source string }
		}
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Schema != "corvint-test-discovery/1" || doc.Trust != "generated" || doc.Revision != revision || len(doc.Examples) != 1 ||
		doc.Examples[0].Identity.Example != "spec/cart_spec.rb[1:1]" || doc.Examples[0].Expectations[0].Matcher != "have_http_status" || len(doc.Examples[0].Navigations) != 2 {
		t.Fatalf("record = %s", out)
	}
	if _, again, _ := corpusCLI(t, root, "docs", "corpus", "discover-tests", "--runner", "rspec", "--revision", revision, "--path", "spec"); again != out {
		t.Fatal("second run differs")
	}
	for want, args := range map[string][]string{
		"corpus-refused":          {"--runner", "jest", "--revision", revision, "--path", "spec"},
		"corpus-invalid-revision": {"--runner", "rspec", "--revision", strings.Repeat("0", 40), "--path", "spec"},
	} {
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "discover-tests"}, args...)...)
		if code == 0 || !strings.Contains(out+stderr, `"`+want+`"`) {
			t.Fatalf("%v = %d %q %q, want %s", args, code, out, stderr, want)
		}
	}
	if !strings.Contains(docsHelp, "docs corpus discover-tests --runner playwright|rspec --revision FULL_COMMIT --path PATH") || !strings.Contains(docsHelp, "DCX-V1-370..376") {
		t.Fatal("docs help does not name discover-tests")
	}
}
