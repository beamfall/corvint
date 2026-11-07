// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"reflect"
	"slices"
	"testing"
)

// AZU-V0-007 (selbatch fix round 2): declared units on one shard whose argv
// is identical, such as two consumers of the browser gate, share one
// invocation that carries both units; a different argv, another shard and a
// unit the declaration names no command for each keep their own.
func TestAZUV0007IdenticalDeclaredCommandsShareInvocation(t *testing.T) {
	gate := []string{"bash", "script/web-flows-gate"} //corvint:fixture-path
	other := append(slices.Clone(gate), "other")
	m := Manifest{RunnerProfile: ProfileLocal, Units: []Unit{
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:declared-consumer:web/e2e.mjs"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:declared-consumer:web/nav.mjs"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:declared-consumer:web/other.mjs"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:kotlin"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:swift"},
		{Adapter: AdapterDeclared, Shard: 1, Slice: SliceWhole, Unit: "declared:declared-consumer:web/late.mjs"},
	}}
	declared := parsedDeclared()
	declared.Languages = []LanguageCommand{
		{Language: "declared-consumer:web/e2e.mjs", Argv: gate},
		{Language: "declared-consumer:web/late.mjs", Argv: gate},
		{Language: "declared-consumer:web/nav.mjs", Argv: gate},
		{Language: "declared-consumer:web/other.mjs", Argv: other},
	}
	want := []Planned{
		{Adapter: AdapterDeclared, Argv: gate, Env: []string{}, Units: m.Units[0:2]},
		{Adapter: AdapterDeclared, Argv: other, Env: []string{}, Units: m.Units[2:3]},
		{Adapter: AdapterDeclared, Argv: []string{}, Env: []string{}, Units: m.Units[3:4]},
		{Adapter: AdapterDeclared, Argv: []string{}, Env: []string{}, Units: m.Units[4:5]},
	}
	if got := Invocations(m, declared, 0, "/repo", ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("shard 0:\ngot  %#v\nwant %#v", got, want)
	}
	if got := Invocations(m, declared, 1, "/repo", ""); !reflect.DeepEqual(got, []Planned{{Adapter: AdapterDeclared, Argv: gate, Env: []string{}, Units: m.Units[5:6]}}) {
		t.Fatalf("shard 1: %#v", got)
	}
}
