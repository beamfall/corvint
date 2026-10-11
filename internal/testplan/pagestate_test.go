package testplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func pageState(t *testing.T, variations ...map[string]any) PageStateReport {
	t.Helper()
	data, err := GroupByPageState(decode(t, inputJSON(t, variations...)))
	if err != nil {
		t.Fatal(err)
	}
	var r PageStateReport
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// ANL-V0-012: same page state and read-only variations from different specs share one group.
func TestANLV0012GroupsSharedPageState(t *testing.T) {
	other := func(v map[string]any) { v["spec"] = "tests/e2e/other.spec.ts" }
	r := pageState(t, variation("v1"), variation("v2", other), variation("v3", set("org", "club-b")))
	if len(r.Groups) != 1 || len(r.Blocked) != 0 {
		t.Fatalf("%+v", r)
	}
	g := r.Groups[0]
	if strings.Join(g.VariationIDs, ",") != "v1,v2" || g.VisitsSaved != 1 || len(g.Specs) != 2 || len(g.Assertions) != 2 {
		t.Fatalf("%+v", g)
	}
	if again := pageState(t, variation("v3", set("org", "club-b")), variation("v2", other), variation("v1")); again.Digest != r.Digest {
		t.Fatal("digest depends on input order")
	}
}

// ANL-V0-012: a fact value containing the old key delimiter cannot alias a different fact set.
func TestANLV0012RequiredSetsStayDistinct(t *testing.T) {
	r := pageState(t, variation("a", set("requires", []any{"a=1", "b=2"})), variation("b", set("requires", []any{"a=1\u0000b=2"})))
	if len(r.Groups) != 0 {
		t.Fatalf("%+v", r.Groups)
	}
}

// ANL-V0-013: destructive, state-changing, requirement-differing and anchor-missing variations are
// never merged into a group.
func TestANLV0013BlocksAndSeparates(t *testing.T) {
	r := pageState(t, variation("a"), variation("b"),
		variation("c", set("destructive", true)),
		variation("d", set("changes", []any{"slot=1"})),
		variation("e", drop("user")),
		variation("f", set("requires", []any{"slot=2"})))
	reasons := map[string]string{}
	for _, b := range r.Blocked {
		reasons[b.VariationID] = b.Reason
	}
	if reasons["c"] != "destructive" || reasons["d"] != "changes-state" || !strings.HasPrefix(reasons["e"], "anchor-missing") || len(reasons) != 3 {
		t.Fatalf("%v", reasons)
	}
	if len(r.Groups) != 1 || strings.Join(r.Groups[0].VariationIDs, ",") != "a,b" {
		t.Fatalf("%+v", r.Groups)
	}
}
