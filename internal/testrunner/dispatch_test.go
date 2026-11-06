package testrunner

import (
	"errors"
	"testing"
)

// TestBuildWithAppliesTheSharedAdmissionFirst traces TRE-V0-081 and
// LGP-V0-011 on the helper registry.Build and a single-family caller share
// (V1-4752): a refused selection or exec member never reaches the family's
// build, an admitted request reaches it unchanged, and ParseWith normalizes
// the family's observation while keeping its error.
func TestBuildWithAppliesTheSharedAdmissionFirst(t *testing.T) {
	called := 0
	build := func(Request) (Invocation, error) { called++; return Invocation{Format: "family"}, nil }
	members := []string{"a", "b"}
	for name, r := range map[string]Request{
		"selection":  {Runner: "a", ExpectedSelection: &Selection{Version: "corvint-test-selection/0"}},
		"prefix":     {Runner: "c", ExecPrefix: []string{"run"}},
		"path map":   {Runner: "c", PathMap: []PathMapping{{Host: "/h", Container: "/c"}}},
		"formatter":  {Runner: "c", Formatter: "f"},
		"no members": {Runner: "a", ExecPrefix: []string{"run"}},
	} {
		list := members
		if name == "no members" {
			list = nil
		}
		if _, err := BuildWith(r, list, build); err == nil {
			t.Fatalf("%s admitted", name)
		} else if name == "prefix" && err.Error() != "execPrefix, pathMap and formatter are admitted only for a, b" {
			t.Fatalf("exec-member refusal %q", err)
		}
	}
	if called != 0 {
		t.Fatalf("a refused request reached the family build %d times", called)
	}
	if v, err := BuildWith(Request{Runner: "b", ExecPrefix: []string{"run"}}, members, build); err != nil || v.Format != "family" || called != 1 {
		t.Fatalf("admitted request: %+v %v %d", v, err, called)
	}
	failed := errors.New("unparsed")
	o, err := ParseWith(Input{Runner: "a", TimedOut: true}, func(Input) (Observation, error) { return Observation{Complete: true}, failed })
	if err != failed || o.Runner != "a" || o.Complete {
		t.Fatalf("ParseWith did not normalize: %+v %v", o, err)
	}
}
