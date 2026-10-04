package cishards

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAFPV0022OrderRunsSelectedUnitsFirstWithoutChangingTheSet(t *testing.T) {
	packages := []string{"m/a", "m/b", "m/c", "m/d", "m/e", "m/f"}
	plan := `{"profile":"affected-plan/0","ok":true,"plan":{"selected":[
		{"unitId":"go:m/e","witness":{"kind":"UNBOUNDED_READER"}},
		{"unitId":"go:m/d","witness":{"kind":"PATH_LITERAL_READER"}},
		{"unitId":"go:m/c","witness":{"kind":"DEPENDENCY_PATH"}},
		{"unitId":"go:m/f","witness":{"kind":"UNBOUNDED_READER"}},
		{"unitId":"go:m/f","witness":{"kind":"DIRECT_SOURCE_CHANGE"}},
		{"unitId":"go:m/absent","witness":{"kind":"DIRECT_SOURCE_CHANGE"}},
		{"unitId":"dotnet:m/a","witness":{"kind":"DIRECT_SOURCE_CHANGE"}}]}}`
	got, ok := Order(packages, []byte(plan))
	if want := []string{"m/c", "m/f", "m/d", "m/e", "m/a", "m/b"}; !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("ok=%v got=%v want=%v", ok, got, want)
	}
	if !reflect.DeepEqual(packages, []string{"m/a", "m/b", "m/c", "m/d", "m/e", "m/f"}) {
		t.Fatalf("input mutated: %v", packages)
	}
	sorted := append([]string(nil), got...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(sorted, packages) {
		t.Fatalf("set changed: %v", got)
	}
}

func TestAFPV0022OrderFallsBackToTheCurrentOrder(t *testing.T) {
	packages := []string{"m/a", "m/b"}
	selected := `"plan":{"selected":[{"unitId":"go:m/b","witness":{"kind":"DIRECT_SOURCE_CHANGE"}}]}`
	for name, plan := range map[string]string{
		"empty":     "",
		"not json":  "go: no plan",
		"truncated": `{"profile":"affected-plan/0","ok":true,` + selected,
		"profile":   `{"profile":"affected-plan/1","ok":true,` + selected + `}`,
		"not ok":    `{"profile":"affected-plan/0","ok":false,` + selected + `}`,
		"oversized": `{"profile":"affected-plan/0","ok":true,` + selected + `}` + strings.Repeat(" ", MaxPlanBytes),
	} {
		if got, ok := Order(packages, []byte(plan)); ok || !reflect.DeepEqual(got, packages) {
			t.Errorf("%s: ok=%v got=%v", name, ok, got)
		}
	}
	if got, ok := Order(packages, []byte(`{"profile":"affected-plan/0","ok":true,"plan":{"selected":[]}}`)); !ok || !reflect.DeepEqual(got, packages) {
		t.Errorf("empty selection: ok=%v got=%v", ok, got)
	}
}
