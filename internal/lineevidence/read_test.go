// SPDX-License-Identifier: Apache-2.0
package lineevidence

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testinventory"
)

func fixture(t *testing.T) (Input, *linecov.Index) {
	t.Helper()
	d := testinventory.Declared{Key: "go@.", Runner: "go", Root: ".", Config: "go.mod"}
	ix, err := linecov.Build("example", "commit", "runner", map[string]string{"calc.go": "blob"}, []linecov.Record{
		{Package: ".", Name: "go:.::TestCalc", Profile: []byte("mode: atomic\nexample/calc.go:1.1,2.2 1 1\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ix.Language, ix.RunnerFamily, ix.Config, ix.Command = "go", "go", "go.mod", []string{"go", "test", "./..."}
	in := Input{Store: t.TempDir(), Instance: d, Runner: "runner", Command: ix.Command,
		BaseFiles: map[string]string{"calc.go": "blob"}, CurrentFiles: map[string]string{"calc.go": "edited"},
		Changed: []string{"calc.go"}, Ancestor: func(string) bool { return true }}
	return in, ix
}

func TestLEVV1003Freshness(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(*Input, *linecov.Index)
		tier   Tier
		reason string
	}{
		{"unqualified", func(*Input, *linecov.Index) {}, File, Unqualified},
		{"runner", func(in *Input, _ *linecov.Index) { in.Runner = "different" }, Full, Stale},
		{"language", func(_ *Input, ix *linecov.Index) { ix.Language = "other" }, Full, Stale},
		{"config", func(_ *Input, ix *linecov.Index) { ix.Config = "other" }, Full, Stale},
		{"command", func(_ *Input, ix *linecov.Index) { ix.Command = []string{"other"} }, Full, Stale},
		{"ancestor", func(in *Input, _ *linecov.Index) { in.Ancestor = func(string) bool { return false } }, Full, Stale},
		{"blob", func(in *Input, _ *linecov.Index) { in.BaseFiles["calc.go"] = "different" }, File, Stale},
		{"untracked", func(in *Input, _ *linecov.Index) {
			in.Changed = append(in.Changed, "new.go")
			in.CurrentFiles["new.go"] = "new"
		}, File, Stale},
		{"deleted", func(in *Input, _ *linecov.Index) { delete(in.CurrentFiles, "calc.go") }, File, Stale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, ix := fixture(t)
			tc.edit(&in, ix)
			if _, err := Write(in.Store, in.Instance, ix); err != nil {
				t.Fatal(err)
			}
			got := Read(in)
			if got.Tier != tc.tier || got.Reason != tc.reason || got.Exact {
				t.Fatalf("%+v", got)
			}
		})
	}
}

func TestLEVV1004AbsentDamagedReadOnly(t *testing.T) {
	in, ix := fixture(t)
	if got := Read(in); got.Reason != Absent {
		t.Fatal(got)
	}
	name, err := Write(in.Store, in.Instance, ix)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	got := Read(in)
	if got.Reason != Unqualified || len(got.Rank) != 1 {
		t.Fatal(got)
	}
	after, _ := os.ReadFile(name)
	if !bytes.Equal(before, after) {
		t.Fatal("reader mutated evidence")
	}
	before[len(before)-1] ^= 1
	if err := os.WriteFile(name, before, 0600); err != nil {
		t.Fatal(err)
	}
	if got := Read(in); got.Reason != Damaged {
		t.Fatal(got)
	}
}

func TestLEVV1002PrivateDeterministicInstances(t *testing.T) {
	in, ix := fixture(t)
	name, err := Write(in.Store, in.Instance, ix)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(name)
	if _, err := Write(in.Store, in.Instance, ix); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(name)
	if !bytes.Equal(first, second) {
		t.Fatal("same index changed")
	}
	stat, _ := os.Stat(name)
	if stat.Mode().Perm()&0077 != 0 {
		t.Fatal("evidence is not private")
	}
	in.Instance.Root = "apps/b"
	in.Instance.Key = "go@apps/b"
	other, err := Write(in.Store, in.Instance, ix)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(name) == filepath.Base(other) {
		t.Fatal("instances collide")
	}
}

func TestLEVV1009TierReasonGolden(t *testing.T) {
	verdicts := []Verdict{
		{Instance: "go@.", Tier: Line, Reason: "qualified-fixture-only"},
		{Instance: "go@.", Tier: File, Reason: Unqualified},
		{Instance: "playwright@web", Tier: Flow, Reason: "accepted-flow"},
		{Instance: "jest@web", Tier: Full, Reason: Stale},
	}
	var out bytes.Buffer
	for _, v := range verdicts {
		out.WriteString(v.String() + "\n")
	}
	want, err := os.ReadFile("testdata/tiers.golden")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("%s", out.Bytes())
	}
}

func TestLEVV1005EveryRunnerUnqualified(t *testing.T) {
	for _, runner := range []string{"go", "vitest", "jest", "rspec", "minitest"} {
		t.Run(runner, func(t *testing.T) {
			in, ix := fixture(t)
			in.Instance.Runner = runner
			in.Instance.Key = runner + "@."
			ix.Language, ix.RunnerFamily = language(runner), runner
			ix.Tests[0].Name = runner + ":calc_test.go::covers"
			if _, err := Write(in.Store, in.Instance, ix); err != nil {
				t.Fatal(err)
			}
			got := Read(in)
			if got.Tier != File || got.Reason != Unqualified || len(got.Rank) != 1 {
				t.Fatalf("%+v", got)
			}
		})
	}
}
