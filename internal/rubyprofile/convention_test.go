package rubyprofile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
)

// conventionFixture is the #1475 repository: autoload-convention sources and
// specs with no Rails application, a namespaced constant and an acronym.
var conventionFixture = map[string]string{
	"Gemfile":                            "source \"https://rubygems.org\"\n",
	"app/models/alpha_account.rb":        "class AlphaAccount\n  def balance; Alpha::Ledger.new.total; end\nend\n",
	"app/models/alpha/ledger.rb":         "module Alpha\n  class Ledger\n    def total; 1; end\n  end\nend\n",
	"app/services/alpha_report.rb":       "class AlphaReport\n  def call; AlphaAccount.new.balance; end\nend\n",
	"app/clients/api_client.rb":          "class APIClient\n  def get; 1; end\nend\n",
	"config/initializers/inflections.rb": "ActiveSupport::Inflector.inflections(:en) do |inflect|\n  inflect.acronym \"API\"\nend\n",
	"spec/models/alpha_account_spec.rb":  "RSpec.describe AlphaAccount do\n  it \"x\" do expect(AlphaAccount.new.balance).to eq(1) end\nend\n",
	"spec/services/alpha_report_spec.rb": "RSpec.describe AlphaReport do\n  it \"y\" do expect(AlphaReport.new.call).to eq(1) end\nend\n",
	"spec/ledger_spec.rb":                "RSpec.describe Alpha::Ledger do\n  it \"l\" do expect(Alpha::Ledger.new.total).to eq(1) end\nend\n",
	"spec/remote/client_spec.rb":         "RSpec.describe APIClient do\n  it \"c\" do expect(APIClient.new.get).to eq(1) end\nend\n",
}

// TestAffectedSelectsEveryImpactAutoloadTest_RUBYAFFECTEDV0045 is the #1475
// parity check: for each changed path, every test that context impact names
// through its autoload-convention references (namespaced constant and acronym
// inflection included) is selected by affected, which stays BOUNDED.
func TestAffectedSelectsEveryImpactAutoloadTest_RUBYAFFECTEDV0045(t *testing.T) {
	requireGit(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for relative, body := range conventionFixture {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.email", "conformance@example.invalid")
	git(t, root, "config", "user.name", "conformance")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "issue 1475 fixture")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(context.Background(), root, ruby.New())
	if err != nil {
		t.Fatal(err)
	}
	for changed, minimum := range map[string]int{"app/models/alpha/ledger.rb": 2, "app/models/alpha_account.rb": 2, "app/clients/api_client.rb": 1} {
		receipt, err := contextindex.Impact(index, []string{changed}, 50)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Results []struct {
				ID   string `json:"id"`
				Kind string `json:"kind"`
			} `json:"results"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{changed})
		selected := map[string]bool{}
		for _, test := range plan.SelectedTests() {
			selected[test] = true
		}
		named := 0
		for _, result := range decoded.Results {
			if result.Kind != "test" {
				continue
			}
			named++
			if !selected[result.ID] {
				t.Fatalf("%s: impact names %s, affected selected %v (scope %s, unknown %v)", changed, result.ID, plan.SelectedTests(), plan.Scope, plan.Unknown)
			}
		}
		if named < minimum || plan.Scope != affected.ScopeBounded {
			t.Fatalf("%s: impact named %d tests (want >= %d), scope %s", changed, named, minimum, plan.Scope)
		}
	}
}
