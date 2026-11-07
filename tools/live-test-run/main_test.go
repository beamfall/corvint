package main

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	cishards "github.com/Beamfall/corvint/.github/cishards"
)

func TestArgvRunsOnlyTheLiveTestsWithTheTag_AFPV02701(t *testing.T) {
	got := argv(cishards.LivePackage{Dir: "internal/x", Tests: []string{"TestA", "TestB_C"}}, "9m")
	want := []string{"go", "test", "-tags", "live", "-count=1", "-timeout", "9m", "-run", "^(TestA|TestB_C)$", "./internal/x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

// run executes one go test per listed package, reports failure as exit 1 and refuses an empty or
// unreadable tier with exit 2.
func TestRunExecutesEachLivePackage_AFPV02701(t *testing.T) {
	tier := func(string) ([]cishards.LivePackage, error) {
		return []cishards.LivePackage{{Dir: "a", Tests: []string{"TestA"}}, {Dir: "b/c", Tests: []string{"TestB", "TestC"}}}, nil
	}
	var calls [][]string
	pass := func(a []string, _, _ io.Writer) bool { calls = append(calls, a); return true }
	var out, errb bytes.Buffer
	if rc := run([]string{"-timeout", "5m"}, &out, &errb, tier, pass); rc != 0 {
		t.Fatalf("rc = %d, stderr %s", rc, errb.String())
	}
	if len(calls) != 2 || calls[1][len(calls[1])-1] != "./b/c" || calls[0][6] != "5m" {
		t.Fatalf("calls = %q", calls)
	}
	fail := func([]string, io.Writer, io.Writer) bool { return false }
	if rc := run(nil, &out, &errb, tier, fail); rc != 1 {
		t.Fatalf("failing packages: rc = %d, want 1", rc)
	}
	calls = nil
	if rc := run([]string{"--list"}, &out, &errb, tier, pass); rc != 0 || len(calls) != 0 || !strings.Contains(out.String(), "b/c\tTestB TestC\n") {
		t.Fatalf("--list rc = %d, calls %q, out %q", rc, calls, out.String())
	}
	empty := func(string) ([]cishards.LivePackage, error) { return nil, nil }
	broken := func(string) ([]cishards.LivePackage, error) { return nil, errors.New("no git") }
	for name, l := range map[string]lister{"empty": empty, "unreadable": broken} {
		if rc := run(nil, &out, &errb, l, pass); rc != 2 {
			t.Fatalf("%s tier: rc = %d, want 2", name, rc)
		}
	}
	if rc := run([]string{"extra"}, &out, &errb, tier, pass); rc != 2 {
		t.Fatalf("extra argument: rc = %d, want 2", rc)
	}
}
