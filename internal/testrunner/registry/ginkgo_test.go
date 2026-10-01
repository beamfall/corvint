package registry_test

import (
	"path/filepath"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// ginkgoRegistryReport is a minimal report shaped from the pinned Ginkgo
// v2.33.0 JSON projection; it is not runner-generated output.
const ginkgoRegistryReport = `[{"SuitePath":"/src/calc","SuiteDescription":"Calc","SuiteLabels":null,"SuiteSemVerConstraints":null,"SuiteComponentSemVerConstraints":null,"SuiteSucceeded":true,"SuiteHasProgrammaticFocus":false,"SpecialSuiteFailureReasons":null,"PreRunStats":{"TotalSpecs":1,"SpecsThatWillRun":1},"StartTime":"2026-10-01T00:00:00Z","EndTime":"2026-10-01T00:00:01Z","RunTime":1,"SuiteConfig":{"RandomSeed":1,"RandomizeAllSpecs":false,"FocusStrings":null,"SkipStrings":null,"FocusFiles":null,"SkipFiles":null,"LabelFilter":"","SemVerFilter":"","FailOnPending":false,"FailOnEmpty":true,"FailFast":false,"FlakeAttempts":1,"MustPassRepeatedly":0,"DryRun":false,"PollProgressAfter":0,"PollProgressInterval":0,"Timeout":3600000000000,"EmitSpecProgress":false,"OutputInterceptorMode":"none","SourceRoots":null,"GracePeriod":1000000000,"SleepOnFailure":0,"ParallelProcess":1,"ParallelTotal":1,"ParallelHost":""},"SpecReports":[{"ContainerHierarchyTexts":["Math"],"ContainerHierarchyLocations":null,"ContainerHierarchyLabels":null,"ContainerHierarchySemVerConstraints":null,"ContainerHierarchyComponentSemVerConstraints":null,"LeafNodeType":"It","LeafNodeLocation":{"FileName":"/src/calc/calc_test.go","LineNumber":9},"LeafNodeLabels":null,"LeafNodeSemVerConstraints":null,"LeafNodeText":"adds","State":"passed","StartTime":"2026-10-01T00:00:00Z","EndTime":"2026-10-01T00:00:00Z","RunTime":1,"ParallelProcess":1,"NumAttempts":1,"MaxFlakeAttempts":1,"MaxMustPassRepeatedly":0}]}]`

func TestGinkgoRegistryDispatchAndTargetBoundary(t *testing.T) {
	count := 0
	for _, r := range registry.Runners() {
		if r == "ginkgo-v2" {
			count++
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	req := tr.Request{Runner: "ginkgo-v2", Executable: "/calc.test", ReportDir: "/fresh", Root: root, Project: "calc_suite_test.go", InputFiles: map[string]string{"calc_suite_test.go": strings.Repeat("a", 64)}, Target: "TestCalc::Calc", ExpectedTests: []string{"TestCalc::Calc::Math adds"}}
	inv, e := registry.Build(req)
	if e != nil || len(inv.Argv) != 17 || inv.Argv[0] != "-test.run=^TestCalc$" || len(inv.Phases) != 0 {
		t.Fatal(inv, e)
	}
	in := tr.Input{Runner: req.Runner, Target: req.Target, SourceRoot: "/src/calc", Expected: req.ExpectedTests, Reports: map[string][]byte{"ginkgo.json": []byte(ginkgoRegistryReport)}, SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes}
	o, e := registry.Parse(in)
	if e != nil || !o.Complete || len(o.Tests) != 1 || o.Tests[0].ID != "TestCalc::Calc::Math adds" || o.Tests[0].State != tr.Passed {
		t.Fatal(o, e)
	}
	in.Target = "TestCalc::Other"
	in.Expected = []string{"TestCalc::Other::Math adds"}
	o, e = registry.Parse(in)
	if e == nil || !strings.Contains(e.Error(), "SuiteDescription differs from Target") || o.Complete {
		t.Fatal("wrong suite admitted", o, e)
	}
	in.Target, in.Expected, in.SourceRoot = req.Target, req.ExpectedTests, "/elsewhere"
	o, e = registry.Parse(in)
	if e != nil || o.Complete || o.Tests[0].State != tr.Unknown {
		t.Fatal("foreign suite path admitted", o, e)
	}
}
