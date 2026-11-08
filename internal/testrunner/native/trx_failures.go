package native

import (
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TRXFailures returns the failed rows of a VSTest TRX report that Parse
// rejected (RSP-V0-003). A selection plugin keeps them so a passing FULL retry
// cannot erase a failure the report already recorded, for example when the
// summary total disagrees with the result rows. A row is kept only when it is
// Failed, its definition is unique, carries the runner's adapter and a class
// and method name, and its identity appears once. Passes and skips of a
// rejected report stay unobserved. Parse's own behavior is unchanged.
func TRXFailures(runner string, b []byte) []tr.Test {
	adapter := map[string]string{"dotnet-vstest-nunit": "executor://nunit3testexecutor/", "dotnet-vstest-mstest": "executor://mstestadapter/v2", "dotnet-vstest-xunit": "executor://xunit/VsTestRunner2/netcoreapp"}[runner]
	if adapter == "" || len(b) > tr.MaxReportBytes || checkNativeTRXXML(b) != nil {
		return nil
	}
	var doc trxDoc
	if decodeXML(b, &doc) != nil || doc.XMLName.Local != "TestRun" || doc.XMLName.Space != "http://microsoft.com/schemas/VisualStudio/TeamTest/2010" {
		return nil
	}
	defs, dup := map[string]string{}, map[string]bool{}
	for _, d := range doc.Definitions {
		if _, ok := defs[d.ID]; ok {
			dup[d.ID] = true
		}
		if d.Method.Adapter != adapter || d.Method.Class == "" || d.Method.Name == "" {
			dup[d.ID] = true
		}
		defs[d.ID] = d.Method.Class + "." + d.Method.Name
	}
	count := map[string]int{}
	for _, r := range doc.Results {
		if id, ok := defs[r.ID]; ok && !dup[r.ID] {
			count[id]++
		}
	}
	var out []tr.Test
	for _, r := range doc.Results {
		id, ok := defs[r.ID]
		if r.Outcome != "Failed" || !ok || dup[r.ID] || strings.TrimSpace(id) == "" || count[id] != 1 || len(out) == tr.MaxTests {
			continue
		}
		row := tr.Test{ID: id, Name: r.Name, State: tr.Failed, FailureKind: tr.Unknown}
		if r.Error != nil {
			row.Message = r.Error.Message
		}
		out = append(out, row)
	}
	return out
}
