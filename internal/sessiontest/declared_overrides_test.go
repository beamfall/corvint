package sessiontest

import (
	"reflect"
	"testing"
)

// STR-V0-173 (V1-3149): --go-parallel replaces the declared Go -p in place,
// in either spelling, appends one when none is declared, keeps every other
// flag, and leaves an unreadable declaration as it is.
func TestSTRV0173WithGoParallelReplacesTheDeclaredP(t *testing.T) {
	for name, c := range map[string]struct{ flags, want []string }{
		"pair":   {[]string{"-p", "1", "-timeout", "30m", "-count=1"}, []string{"-p", "4", "-timeout", "30m", "-count=1"}},
		"equals": {[]string{"-count=1", "-p=1"}, []string{"-count=1", "-p=4"}},
		"absent": {[]string{"-count=1"}, []string{"-count=1", "-p", "4"}},
	} {
		declared := Declared{GoEnv: []string{"GOTOOLCHAIN=local"}, GoFlags: c.flags}
		got := declared.WithGoParallel(4)
		if !reflect.DeepEqual(got.GoFlags, c.want) || !reflect.DeepEqual(got.GoEnv, declared.GoEnv) {
			t.Errorf("%s: flags %q env %q, want %q", name, got.GoFlags, got.GoEnv, c.want)
		}
		if !reflect.DeepEqual(declared.GoFlags, c.flags) {
			t.Errorf("%s: the declaration was modified: %q", name, declared.GoFlags)
		}
	}
	unreadable := Declared{Unreadable: "missing"}
	if got := unreadable.WithGoParallel(4); !reflect.DeepEqual(got, unreadable) {
		t.Errorf("unreadable declaration changed: %+v", got)
	}
}

// STR-V0-174 (V1-3155): a declared GOCACHE yields to a non-empty inherited
// CORVINT_GOCACHE at launch; without a declared GOCACHE (the ci profile) or
// without the override the env is unchanged.
func TestSTRV0174GoCacheOverride(t *testing.T) {
	declared := []string{"GOCACHE=/tmp/corvint-go-build-cache", "GOTOOLCHAIN=local"}
	if got := GoCacheOverride(declared, []string{"CORVINT_GOCACHE=/lane"}); !reflect.DeepEqual(got, []string{"GOCACHE=/lane", "GOTOOLCHAIN=local"}) {
		t.Errorf("override: %q", got)
	}
	if got := GoCacheOverride(declared, []string{"CORVINT_GOCACHE=", "GOCACHE=/x"}); !reflect.DeepEqual(got, declared) {
		t.Errorf("empty override changed the env: %q", got)
	}
	ci := []string{"GOTOOLCHAIN=local", "GOFLAGS="}
	if got := GoCacheOverride(ci, []string{"CORVINT_GOCACHE=/lane"}); !reflect.DeepEqual(got, ci) {
		t.Errorf("ci env changed: %q", got)
	}
	if !reflect.DeepEqual(declared, []string{"GOCACHE=/tmp/corvint-go-build-cache", "GOTOOLCHAIN=local"}) {
		t.Errorf("the declared env was modified: %q", declared)
	}
}
