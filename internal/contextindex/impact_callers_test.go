package contextindex

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestImpactDisclosesOmittedDirectGoCallers is the V1-0263 regression, a
// synthetic reduction of a Beamfall path impact. A feature record reached only
// through a reverse-importing test's marker (800) and a same-package reference
// (775) outrank the direct cross-package caller's reverse-import row (700), so
// a small limit drops the caller. The packet must then name the caller, its
// call line, its rank and the limit, and must not name an importer that calls
// nothing or a caller that fits within the limit.
func TestImpactDisclosesOmittedDirectGoCallers(t *testing.T) {
	t.Run("GPK-V0-075", func(t *testing.T) {
		root := impactRepositoryWithFiles(t, map[string]string{
			"go.mod": "module example.test/fixture\n\ngo 1.27.0\n",
			"testing/features.yaml": "features:\n" +
				"  - id: unrelated-dashboard\n    area: console\n    summary: Unrelated dashboard.\n    adr: []\n    applies: [server]\n    status: shipped\n",
			"testing/scenarios.yaml": "scenarios: []\n",
			"pkg/core/core.go":       "package core\n\nfunc ComputeTotal() int { return 1 }\n",
			"pkg/core/core_test.go":  "package core\n\nfunc TestComputeTotal() { _ = ComputeTotal() }\n",
			"pkg/core/other.go":      "package core\n\nvar total = ComputeTotal()\n",
			"pkg/app/app.go":         "package app\n\nimport (\n\t\"example.test/fixture/pkg/core\"\n)\n\nfunc Run() int {\n\treturn core.ComputeTotal()\n}\n",
			"pkg/alias/alias.go":     "package alias\n\nimport c \"example.test/fixture/pkg/core\"\n\nvar _ = c.ComputeTotal\n",
			"pkg/blank/blank.go":     "package blank\n\nimport _ \"example.test/fixture/pkg/core\"\n",
			"pkg/dash/dash_test.go":  "package dash\n\nimport \"example.test/fixture/pkg/core\"\n\n// feature:unrelated-dashboard\nfunc TestDash() { _ = core.ComputeTotal() }\n",
		})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		narrow, err := Impact(index, []string{"pkg/core/core.go"}, 3)
		if err != nil {
			t.Fatal(err)
		}
		uncertainty := strings.Join(stringsField(narrow["coverage"].(map[string]any)["uncertainty"]), "\n")
		want := "direct Go caller pkg/alias/alias.go (line 5 names c.ComputeTotal declared by pkg/core/core.go) ranked 5 as a 700-score reverse-import row and was omitted by result limit 3\n" +
			"direct Go caller pkg/app/app.go (line 8 names core.ComputeTotal declared by pkg/core/core.go) ranked 6 as a 700-score reverse-import row and was omitted by result limit 3"
		if !strings.Contains(uncertainty, want) {
			t.Fatalf("uncertainty lacks %q:\n%s", want, uncertainty)
		}
		for _, absent := range []string{"pkg/blank/blank.go", "pkg/dash/dash_test.go"} {
			if strings.Contains(uncertainty, absent) {
				t.Fatalf("uncertainty names %s, which is not a non-test caller:\n%s", absent, uncertainty)
			}
		}
		budget := MaxPacketBytes
		budgeted, err := EvalImpact(index, []string{"pkg/core/core.go"}, 3, &budget)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(stringsField(budgeted["coverage"].(map[string]any)["uncertainty"]), "\n"); !strings.Contains(got, want) {
			t.Fatalf("budgeted impact uncertainty lacks %q:\n%s", want, got)
		}
		wide, err := Impact(index, []string{"pkg/core/core.go"}, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		if got := stringsField(wide["coverage"].(map[string]any)["uncertainty"]); len(got) != 0 {
			t.Fatalf("a limit that keeps every caller disclosed %q", got)
		}
	})
}

// TestImpactNamesTheImportingTestThatCarriesARelatedMarker is the V1-0263
// reason-text regression. A feature record that only an importing test's
// marker admits keeps its 800 score (GPK-V0-067 stays rejected) but names
// that test instead of claiming the changed path carries the marker; a key
// the changed path itself carries keeps the changed-path reason even when an
// importing test carries it too.
func TestImpactNamesTheImportingTestThatCarriesARelatedMarker(t *testing.T) {
	t.Run("GPK-V0-075", func(t *testing.T) {
		root := impactRepositoryWithFiles(t, map[string]string{
			"go.mod": "module example.test/fixture\n\ngo 1.27.0\n",
			"testing/features.yaml": "features:\n" +
				"  - id: core-total\n    area: core\n    summary: Core total.\n    adr: []\n    applies: [server]\n    status: shipped\n" +
				"  - id: unrelated-dashboard\n    area: console\n    summary: Unrelated dashboard.\n    adr: []\n    applies: [server]\n    status: shipped\n",
			"testing/scenarios.yaml": "scenarios: []\n",
			"pkg/core/core.go":       "package core\n\n// feature:core-total\nfunc ComputeTotal() int { return 1 }\n",
			"pkg/dash/dash_test.go":  "package dash\n\nimport \"example.test/fixture/pkg/core\"\n\n// feature:unrelated-dashboard\nfunc TestDash() { _ = core.ComputeTotal() }\n",
			"pkg/sum/sum_test.go":    "package sum\n\nimport \"example.test/fixture/pkg/core\"\n\n// feature:core-total\nfunc TestSum() { _ = core.ComputeTotal() }\n",
		})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Impact(index, []string{"pkg/core/core.go"}, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result["results"])
		if err != nil {
			t.Fatal(err)
		}
		packet := string(encoded)
		for _, want := range []string{
			`"reason":"test pkg/dash/dash_test.go importing changed pkg/core/core.go carries feature:unrelated-dashboard"`,
			`"reason":"changed path carries feature:core-total"`,
		} {
			if !strings.Contains(packet, want) {
				t.Fatalf("results lack %s:\n%s", want, packet)
			}
		}
		for _, absent := range []string{"changed path carries feature:unrelated-dashboard", "sum_test.go importing changed"} {
			if strings.Contains(packet, absent) {
				t.Fatalf("results carry %q:\n%s", absent, packet)
			}
		}
		for _, item := range result["results"].([]any) {
			row := item.(map[string]any)
			if row["id"] == "unrelated-dashboard" && row["score"] != 800 {
				t.Fatalf("importing-test feature record scored %v, not 800", row["score"])
			}
		}
	})
}
