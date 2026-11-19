// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

func TestLEVV1014GoSliceArgumentLimit(t *testing.T) {
	m := Manifest{RunnerProfile: ProfileLocal, Units: []Unit{{Adapter: AdapterGo, Unit: "go:example/pkg", Slice: SliceSelected, Flag: "-run", Pattern: strings.Repeat("X", contract.MaxArgBytes+1), Shard: 0}}}
	got := Invocations(m, Declared{}, 0, "", "")
	if len(got) != 1 || slices.Contains(got[0].Argv, "-run") || !slices.Contains(got[0].Argv, "example/pkg") || got[0].Units[0].Reason != contract.ReasonArgvLimit {
		t.Fatalf("%+v", got)
	}
}

func TestLEVV1014PytestArgumentFile(t *testing.T) {
	request := contract.Request{Instance: contract.Instance{Runner: "pytest", Root: "."}, Full: []string{"python", "-m", "pytest"}}
	for i := 0; i < 5000; i++ {
		request.Tests = append(request.Tests, fmt.Sprintf("pytest:tests/test_%04d_%s.py", i, strings.Repeat("x", 60)))
	}
	runner, _ := contract.For("pytest")
	fallback, err := runner.Command(request)
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Reason != contract.ReasonArgvLimit {
		t.Fatal(fallback)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	old := pytestArgumentsForVersion(request, fallback, "8.1.9", report)
	if old.Tier != contract.TierFull {
		t.Fatal(old)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(report), "selected.args")); !os.IsNotExist(err) {
		t.Fatal("old runner wrote args")
	}
	got := pytestArgumentsForVersion(request, fallback, "8.2.0", report)
	if got.Tier != contract.TierFile || !contract.ArgvFits(got.Argv) || !strings.HasPrefix(got.Argv[len(got.Argv)-1], "@") {
		t.Fatal(got)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(report), "selected.args"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 5000 {
		t.Fatal("argument file lost a selection")
	}
}
