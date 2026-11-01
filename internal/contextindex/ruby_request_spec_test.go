package contextindex

import "testing"

// TestRubyControllerReachesItsRequestSpecByConvention_NGIV0761 (V1-1851): a
// controller change lists spec/requests/<name>_spec.rb as an advisory test row
// with a named convention witness, a namespaced controller maps to its
// namespaced spec, and a controller with no conventional request spec (or the
// application controller) abstains rather than guess.
func TestRubyControllerReachesItsRequestSpecByConvention_NGIV0761(t *testing.T) {
	t.Parallel()
	files := rubyAutoloadIssueFiles()
	files["spec/requests/things_spec.rb"] = "RSpec.describe \"things\", type: :request do\n  it { get \"/things/1\" }\nend\n"
	files["app/controllers/admin/users_controller.rb"] = "module Admin\n  class UsersController < ApplicationController\n  end\nend\n"
	files["spec/requests/admin/users_spec.rb"] = "RSpec.describe \"admin users\", type: :request do\n  it { get \"/admin/users\" }\nend\n"
	files["spec/requests/application_spec.rb"] = "RSpec.describe \"app\", type: :request do\n  it { get \"/\" }\nend\n"
	for changed, want := range map[string]string{
		"app/controllers/things_controller.rb":      "spec/requests/things_spec.rb",
		"app/controllers/admin/users_controller.rb": "spec/requests/admin/users_spec.rb",
	} {
		row := rubyResultRows(registryImpact(t, files, changed))[want]
		if row == nil || row["kind"] != "test" || row["advisory"] != true || row["score"] != rubyRequestSpecConventionScore {
			t.Fatalf("%s: request spec row = %#v", changed, row)
		}
		evidenceRows := mapsFromAny(row["evidence"])
		if len(evidenceRows) != 1 || evidenceRows[0]["confidence"] != "low" || evidenceRows[0]["authority"] != "syntax" ||
			stringValue(evidenceRows[0]["reason"]) == "" {
			t.Fatalf("%s: request spec evidence = %#v", changed, evidenceRows)
		}
	}
	for changed, absent := range map[string]string{
		"app/controllers/other_controller.rb":       "spec/requests/other_spec.rb",
		"app/controllers/application_controller.rb": "spec/requests/application_spec.rb",
	} {
		if row := rubyResultRows(registryImpact(t, files, changed))[absent]; row != nil {
			t.Fatalf("%s guessed %s: %#v", changed, absent, row)
		}
	}
}
