package witnesspaths

import "testing"

// TestCAXV0062DeclareNeverClassifiesCodeOrThePolicy checks CAX-V0-062: a
// path an extractor reads and the policy file itself are never declared,
// whatever glob matches them, while Rule still names the matching rule; and
// rules sharing one line keep their own position.
func TestCAXV0062DeclareNeverClassifiesCodeOrThePolicy(t *testing.T) {
	policy, rules := FromBlob([]byte(`{"profile":"corvint-witness-paths/0","rules":[{"glob":"docs/**","class":"inert"},{"glob":"**","class":"data","readers":["a.go"]}]}`))
	if policy.Status != "BOUND" || policy.Rules != 2 || rules.Len() != 2 {
		t.Fatalf("policy %+v", policy)
	}
	for _, name := range []string{"docs/tool.go", "docs/run.sh", "src/app.ts", PolicyPath} {
		if declaration := rules.Declare(name); declaration != nil {
			t.Fatalf("%s declared %+v", name, declaration)
		}
	}
	if declaration := rules.Rule("docs/tool.go"); declaration == nil || declaration.Glob != "docs/**" {
		t.Fatalf("Rule(docs/tool.go) = %+v", declaration)
	}
	inert, data := rules.Declare("docs/guide.md"), rules.Declare("config/app.yml")
	if inert == nil || inert.Class != ClassInert || inert.Line != 1 || inert.rule != 1 {
		t.Fatalf("inert %+v", inert)
	}
	if data == nil || data.Class != ClassData || data.Line != 1 || data.rule != 2 {
		t.Fatalf("data %+v", data)
	}
	if tier, reason := LanguageCoverage("config/app.yml"); tier != "NONE" || reason != "LANGUAGE_COVERAGE_NONE" {
		t.Fatalf("yaml coverage %s %s", tier, reason)
	}
	if invalid, none := FromBlob([]byte(`{"profile":"other","rules":[]}`)); invalid.Status != "INVALID" || none.Len() != 0 {
		t.Fatalf("invalid policy %+v", invalid)
	}
}
