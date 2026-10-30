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
		"profile":   `{"profile":"affected-plan/2","ok":true,` + selected + `}`,
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

// TestAFPV0035OrderReadsTheCompactDefaultPlan pins that the compact
// affected-plan/1 default (testCount, no tests array) orders like the full /0.
func TestAFPV0035OrderReadsTheCompactDefaultPlan(t *testing.T) {
	packages := []string{"m/a", "m/b"}
	plan := `{"profile":"affected-plan/1","ok":true,"plan":{"excluded":{"count":1,"digest":"d","groups":[]},"selected":[{"testCount":3,"unitId":"go:m/b","witness":{"kind":"DIRECT_SOURCE_CHANGE"}}]}}`
	if got, ok := Order(packages, []byte(plan)); !ok || !reflect.DeepEqual(got, []string{"m/b", "m/a"}) {
		t.Fatalf("ok=%v got=%v", ok, got)
	}
}

func TestAFPV0025ShareReportsSelectedEstimatedTime(t *testing.T) {
	costs := []byte(`{"profile":"corvint-ci-package-costs/0","source":{"goVersion":"go1.27.1","revision":"0000000000000000000000000000000000000000","runUrl":"https://github.com/beamfall/corvint/actions/runs/1"},"milliseconds":{"m/a":1000,"m/b":3000,"m/c":2000,"m/d":4000}}`)
	plan := []byte(`{"profile":"affected-plan/0","ok":true,"plan":{"selected":[
		{"unitId":"go:m/a","witness":{"kind":"DIRECT_SOURCE_CHANGE"}},
		{"unitId":"go:m/d","witness":{"kind":"UNBOUNDED_READER"}},
		{"unitId":"go:m/e","witness":{"kind":"UNBOUNDED_READER"}},
		{"unitId":"go:m/absent","witness":{"kind":"DIRECT_SOURCE_CHANGE"}},
		{"unitId":"dotnet:m/b","witness":{"kind":"DIRECT_SOURCE_CHANGE"}}]}}`)
	got, ok := shareOf([]string{"m/a", "m/b", "m/c", "m/d", "m/e"}, plan, costs)
	// m/e is unpriced and takes the median, 3000.
	want := Share{Profile: "corvint-ci-selected-share/0", SelectedPackages: 3, UniversePackages: 5, UnboundedPackages: 2,
		SelectedMilliseconds: 8000, UniverseMilliseconds: 13000, UnboundedMilliseconds: 7000, UnpricedPackages: 1, SelectedPermille: 615}
	if !ok || got != want {
		t.Fatalf("ok=%v got=%+v want=%+v", ok, got, want)
	}
	for name, bad := range map[string][2][]byte{
		"unreadable plan":  {[]byte(`{"profile":"affected-plan/0","ok":false}`), costs},
		"unreadable costs": {plan, []byte(`{}`)},
	} {
		if _, ok := shareOf([]string{"m/a"}, bad[0], bad[1]); ok {
			t.Fatalf("%s: reported a share", name)
		}
	}
	if _, ok := ShareOf(nil, plan); ok {
		t.Fatal("empty universe reported a share")
	}
}
