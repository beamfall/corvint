package affected

import (
	"context"
	"reflect"
	"testing"
)

// AFP-V0-192 (V1-2075): an instruction-file-only change selects nothing when
// no unit's reads are unbounded, and otherwise names, per UNBOUNDED_READER
// selection, the unit whose own file makes its reads unbounded and why: the
// unit itself, or the root-locating unit it depends on or whose dependent its
// tests import. A declared read scope still excludes a unit (AFP-V0-023).
func TestInstructionFileChangeNamesEachUnboundedReadersCause_V1_2075(t *testing.T) {
	bounded := fake{name: "go", units: []Unit{
		{ID: "go:core", Sources: []string{"core/core.go"}, Tests: []string{"core/core_test.go"}},
		{ID: "go:mid", Sources: []string{"mid/mid.go"}, Tests: []string{"mid/mid_test.go"}, Imports: []string{"go:core"}},
	}}
	graph, err := Build(context.Background(), t.TempDir(), bounded)
	if err != nil {
		t.Fatal(err)
	}
	if plan := Select(graph, []string{"AGENTS.md"}); len(plan.Selected) != 0 {
		t.Fatalf("bounded repository: selected=%v", witnesses(plan))
	}

	language := fake{name: "go", units: []Unit{
		{ID: "go:own", Sources: []string{"own/own.go"}, Tests: []string{"own/own_test.go"}, UnboundedReads: "own/own_test.go calls os.Getwd"},
		{ID: "go:locator", Sources: []string{"locator/locator.go"}, LocatesRoot: true, UnboundedReads: "locator/locator.go calls runtime.Caller"},
		{ID: "go:user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:locator"}},
		{ID: "go:testuser", Sources: []string{"testuser/testuser.go"}, Tests: []string{"testuser/testuser_test.go"}, TestImports: []string{"go:user"}},
		{ID: "go:scoped", Sources: []string{"scoped/scoped.go"}, Tests: []string{"scoped/scoped_test.go"}, Imports: []string{"go:locator"}, ReadScoped: true},
		{ID: "go:plain", Sources: []string{"plain/plain.go"}, Tests: []string{"plain/plain_test.go"}},
	}}
	graph, err = Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"AGENTS.md"})
	locator := &UnboundedCause{Unit: "go:locator", Reads: "locator/locator.go calls runtime.Caller"}
	want := map[string]Witness{
		"go:own":      {Kind: WitnessUnboundedReader, DirtyPath: "AGENTS.md", Via: []string{"go:own"}, Cause: &UnboundedCause{Unit: "go:own", Reads: "own/own_test.go calls os.Getwd"}},
		"go:user":     {Kind: WitnessUnboundedReader, DirtyPath: "AGENTS.md", Via: []string{"go:user"}, Cause: locator},
		"go:testuser": {Kind: WitnessUnboundedReader, DirtyPath: "AGENTS.md", Via: []string{"go:testuser"}, Cause: locator},
	}
	if got := witnesses(plan); !reflect.DeepEqual(got, want) {
		t.Fatalf("selected=%v\nwant     %v", got, want)
	}
}
