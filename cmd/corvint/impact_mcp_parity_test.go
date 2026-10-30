package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// NGI-V0-068: CLI `corvint impact` and MCP `corvint.impact` return the same
// receipt for the same Ruby path, revision and limit. The fixture's autoload
// roots and instruction references make the receipt larger than the historical
// harness default budget, so an implicit CLI budget would drop the advisory
// reference and test rows the MCP receipt keeps (beamfall/corvint#1786).
func TestImpactCLIMatchesMCPOnRuby(t *testing.T) {
	root := rubyParityFixture(t)
	registry, bridgeErr := bridge.New(root)
	if bridgeErr != nil {
		t.Fatal(bridgeErr)
	}
	const target = "app/interactors/x/refund.rb"
	cases := []struct {
		name string
		mcp  string
		cli  []string
	}{
		{"default", `{"paths":["` + target + `"],"limit":20}`, []string{"impact", "--limit", "20", target}},
		{"explicit-profile", `{"paths":["` + target + `"],"limit":20}`, []string{"impact", "--language-profile", "non-go-syntax-v0", "--limit", "20", target}},
		{"explicit-budget", `{"paths":["` + target + `"],"limit":20,"budgetBytes":5952}`, []string{"impact", "--limit", "20", "--budget-bytes", "5952", target}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			result, callErr := registry.Call(context.Background(), bridge.ToolImpact, []byte(item.mcp))
			if callErr != nil || result.State != "READY" {
				t.Fatalf("%s: %#v %#v", item.mcp, callErr, result.Abstention)
			}
			mcpRaw, err := json.Marshal(result.Receipt)
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := run(append([]string{"--root", root}, item.cli...), bytes.NewReader(nil), &stdout, &stderr); code != 0 {
				t.Fatalf("%v: exit %d: %s", item.cli, code, stderr.String())
			}
			var packet map[string]json.RawMessage
			if err := json.Unmarshal(stdout.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			cli, mcp := decodeNumbers(t, packet["context"]), decodeNumbers(t, mcpRaw)
			if !reflect.DeepEqual(cli, mcp) {
				t.Fatalf("CLI %v receipt differs from corvint.impact %s:\n%s\n%s", item.cli, item.mcp, packet["context"], mcpRaw)
			}
			coverage := cli.(map[string]any)["coverage"].(map[string]any)
			omitted := coverage["omitted_results"].(json.Number).String()
			if item.name == "explicit-budget" {
				if omitted == "0" {
					t.Fatalf("fixture no longer exceeds the historical budget: %v", coverage)
				}
				return
			}
			if coverage["advisory_results"].(json.Number).String() != "3" || omitted != "0" || coverage["budget_bytes"] != nil {
				t.Fatalf("syntax profile was budgeted or lost advisory rows: %v", coverage)
			}
		})
	}
}

// NGI-V0-069: --language-profile is accepted in any position, in the space and
// `=` forms, and every placement yields the same receipt bytes
// (beamfall/corvint#1786 defect 2). JavaScript and TypeScript paths keep the
// historical profile by default, so an ignored flag would change the receipt.
func TestImpactLanguageProfileAnyPosition(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"web/a.ts":    "import { b } from './b.js';\nexport const a = b;\n",
		"web/b.js":    "export const b = 1;\n",
		"web/main.go": "package web\n",
		"go.mod":      "module example.test/web\n\ngo 1.27\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cemGit(t, root, "init", "-q")
	cemGit(t, root, "config", "user.name", "fixture")
	cemGit(t, root, "config", "user.email", "fixture@example.test")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "profile position fixture")
	invoke := func(args []string) []byte {
		t.Helper()
		var stdout, stderr bytes.Buffer
		argv := append([]string{"--root", root, "impact"}, args...)
		if code := run(argv, bytes.NewReader(nil), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit %d: %s", argv, code, stderr.String())
		}
		return append([]byte(nil), stdout.Bytes()...)
	}
	const first, second = "web/a.ts", "web/b.js"
	historical := invoke([]string{"--limit", "20", first, second})
	if strings.Contains(string(historical), `"language_profile"`) {
		t.Fatalf("default TypeScript receipt selected the syntax profile: %s", historical)
	}
	var want []byte
	for _, form := range [][]string{{"--language-profile", "non-go-syntax-v0"}, {"--language-profile=non-go-syntax-v0"}} {
		with := func(parts ...[]string) []string {
			var args []string
			for _, part := range parts {
				args = append(args, part...)
			}
			return args
		}
		limit, paths := []string{"--limit", "20"}, []string{first, second}
		for _, args := range [][]string{
			with(form, limit, paths),
			with(limit, form, paths),
			with(limit, paths, form),
			with(limit, []string{first}, form, []string{second}),
			with([]string{first}, form, []string{second}, limit),
		} {
			got := invoke(args)
			if !strings.Contains(string(got), `"language_profile":"non-go-syntax-v0"`) {
				t.Fatalf("%v: profile not applied: %s", args, got)
			}
			if want == nil {
				want = got
			} else if !bytes.Equal(want, got) {
				t.Fatalf("%v: receipt bytes depend on flag position", args)
			}
		}
	}
	if bytes.Equal(want, historical) {
		t.Fatal("profile receipt equals the historical receipt; the test cannot detect an ignored flag")
	}
}

func rubyParityFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	stage := "app/interactors/x/processing/pipeline/stages/very_long_interactor_name_for_refund_processing_step_with_extra_words_to_lengthen_it_%02d.rb"
	agents := []string{"# App rules\n\nRefunds live in `app/interactors/x/refund.rb`.\n"}
	files := map[string]string{
		"app/interactors/x/refund.rb":           "module X\n  class Refund\n    def call(v)\n      public_send(v)\n      const_get(v)\n    end\n  end\nend\n",
		"app/controllers/refunds_controller.rb": "class RefundsController\n  def create\n    X::Refund.new.call\n  end\nend\n",
		"spec/refund_spec.rb":                   "RSpec.describe X::Refund do\nend\n",
		"spec/refunds_controller_spec.rb":       "RSpec.describe RefundsController do\n  it { X::Refund }\nend\n",
	}
	for index := 0; index < 12; index++ {
		name := fmt.Sprintf(stage, index)
		agents = append(agents, fmt.Sprintf("- Step %d: `%s`.\n", index, name))
		files[name] = fmt.Sprintf("module X\n  class Stage%02d\n  end\nend\n", index)
	}
	files["app/AGENTS.md"] = strings.Join(agents, "")
	for index := 0; index < 40; index++ {
		files[fmt.Sprintf("app/interactors/x/very_long_interactor_name_for_refund_processing_step_%02d.rb", index)] = fmt.Sprintf("module X\n  class VeryLongInteractorNameForRefundProcessingStep%02d\n  end\nend\n", index)
	}
	for _, directory := range []string{"adapters", "builders", "calculators", "channels", "clients", "commands", "decorators", "exporters", "forms", "handlers", "helpers", "importers", "jobs", "listeners", "mailers", "models", "notifiers", "operations", "parsers", "policies", "presenters", "queries", "serializers", "services", "subscribers", "uploaders", "validators", "workers"} {
		files["app/"+directory+"/"+directory+"_thing.rb"] = "class Thing\nend\n"
	}
	for index := 1; index <= 30; index++ {
		files[fmt.Sprintf("app/extra_root_directory_%d/thing.rb", index)] = "class Thing\nend\n"
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cemGit(t, root, "init", "-q")
	cemGit(t, root, "config", "user.name", "fixture")
	cemGit(t, root, "config", "user.email", "fixture@example.test")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "ruby parity fixture")
	return root
}
