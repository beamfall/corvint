package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

func selectionFile(t *testing.T, sel linecov.Selection, wrap bool) string {
	t.Helper()
	var v any = sel
	if wrap {
		v = map[string]any{"profile": "affected-plan/1", "lineSelection": sel}
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "selection.json")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// GLS-V0-042, GLS-V0-043: a NARROWED selection rewrites only decided whole
// packages; anything else leaves the lists as they are.
func TestCISelectionRewritesWholePackages(t *testing.T) {
	sel := linecov.Selection{Profile: linecov.SelectionProfile, State: linecov.StateNarrowed,
		Exclusion: &linecov.SelectionExclusion{Record: "sha256:x", Changed: []string{"change p/p.go:3-3"}, Invalidation: linecov.Invalidation},
		Units: []linecov.UnitSelection{
			{Unit: "go:m/excluded", Mode: linecov.Exclude, Declared: 3, Excluded: 3},
			{Unit: "go:m/tests", Mode: linecov.Tests, Run: "^(TestA|TestB)$", Declared: 5, Excluded: 3},
			{Unit: "go:m/compile", Mode: linecov.Compile, Run: "^$"},
			{Unit: "go:m/whole", Mode: linecov.Whole},
			{Unit: "go:m/split", Mode: linecov.Exclude},
			{Unit: "go:m/odd", Mode: linecov.Tests, Run: "^(TestA|.*)$"},
		}}
	whole := []string{"m/excluded", "m/tests", "m/compile", "m/whole", "m/undecided", "m/split", "m/odd"}
	slices := []string{"-run ^TestX$ m/split"}
	for _, wrap := range []bool{false, true} {
		out := t.TempDir()
		if err := ciSelection(selectionFile(t, sel, wrap), whole, slices, out); err != nil {
			t.Fatal(err)
		}
		read := func(name string) string {
			data, err := os.ReadFile(filepath.Join(out, name))
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		if got, want := read("packages"), "m/whole\nm/undecided\nm/split\nm/odd\n"; got != want {
			t.Errorf("packages %q, want %q", got, want)
		}
		if got, want := read("slices"), "-run ^(TestA|TestB)$ m/tests\n-run ^$ m/compile\n-run ^TestX$ m/split\n"; got != want {
			t.Errorf("slices %q, want %q", got, want)
		}
		var plan CIPlan
		if err := json.Unmarshal([]byte(read("plan.json")), &plan); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(plan.Excluded, []string{"m/excluded"}) || plan.Exclusion == nil || plan.State != linecov.StateNarrowed {
			t.Errorf("plan %+v", plan)
		}
	}
	for name, file := range map[string]string{
		"fallback":   selectionFile(t, linecov.Fallback(linecov.ReasonRunnerMismatch, "x"), true),
		"unreadable": filepath.Join(t.TempDir(), "absent.json"),
	} {
		out := t.TempDir()
		if err := ciSelection(file, whole, slices, out); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(filepath.Join(out, "packages"))
		gotSlices, _ := os.ReadFile(filepath.Join(out, "slices"))
		if string(got) != strings.Join(whole, "\n")+"\n" || string(gotSlices) != slices[0]+"\n" {
			t.Errorf("%s changed the lists: %q %q", name, got, gotSlices)
		}
	}
}

// GLS-V0-046: a failed test a narrowed run left out is a suspect; a kept
// one, a whole unit and a FALLBACK run are not.
func TestMissFindsExcludedFailures(t *testing.T) {
	dir := t.TempDir()
	failed := filepath.Join(dir, "failed.json")
	write(t, dir, "failed.json", `{"reasons":[{"package":"m/p","tests":["TestKept","TestLeft","TestLeft/sub"],"testCount":3,"reason":"test-failures"},`+
		`{"package":"m/q","tests":[],"testCount":0,"reason":"panic"},{"package":"m/w","tests":["TestW"],"testCount":1,"reason":"test-failures"}]}`)
	narrowed := linecov.Selection{Profile: linecov.SelectionProfile, State: linecov.StateNarrowed, Base: "b1", Head: "h1", Index: &linecov.SelectionIndex{Runner: "go1 r"},
		Exclusion: &linecov.SelectionExclusion{Record: "sha256:r"},
		Units: []linecov.UnitSelection{
			{Unit: "go:m/p", Mode: linecov.Tests, Run: "^(TestKept)$", Kept: []linecov.KeptTest{{Name: "TestKept", Why: linecov.WhySelected}}, Declared: 2, Excluded: 1},
			{Unit: "go:m/q", Mode: linecov.Exclude, Declared: 4, Excluded: 4},
			{Unit: "go:m/w", Mode: linecov.Whole, Declared: 1},
		}}
	mq := filepath.Join(dir, "mq")
	write(t, mq, "applied", "NARROWED\n")
	write(t, mq, "after/plan.json", `{"state":"NARROWED","sliced":["m/p"],"excluded":["m/q"]}`)
	write(t, mq, "meta", "run 11\nevent merge_group\nbase b1\nhead h1\n")
	data, _ := json.Marshal(narrowed)
	write(t, mq, "selection.json", string(data))
	fb := filepath.Join(dir, "fb")
	write(t, fb, "meta", "run 12\nevent pull_request\nbase b2\nhead h2\n")
	data, _ = json.Marshal(linecov.Fallback(linecov.ReasonNotAncestor, "x"))
	write(t, fb, "selection.json", string(data))
	stdout := captureStdout(t, func() {
		if err := miss([]string{"-failed", failed, "-run", mq, "-run", fb}); err != nil {
			t.Fatal(err)
		}
	})
	var got []Suspect
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	want := []Suspect{
		{Test: "TestLeft", Package: "m/p", Run: "11", Event: "merge_group", Base: "b1", Head: "h1", Record: "sha256:r", Mode: "tests", Runner: "go1 r"},
		{Test: "(outside every test)", Package: "m/q", Run: "11", Event: "merge_group", Base: "b1", Head: "h1", Record: "sha256:r", Mode: "exclude", Runner: "go1 r"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("suspects %+v, want %+v", got, want)
	}
	for _, control := range []struct{ name, file, body string }{
		{"shadow", "applied", "SHADOW\n"},
		{"split packages unchanged", "after/plan.json", `{"state":"NARROWED","sliced":[],"excluded":[]}`},
		{"range mismatch", "meta", "run 11\nevent merge_group\nbase other\nhead h1\n"},
	} {
		t.Run(control.name, func(t *testing.T) {
			old, err := os.ReadFile(filepath.Join(mq, control.file))
			if err != nil {
				t.Fatal(err)
			}
			write(t, mq, control.file, control.body)
			defer write(t, mq, control.file, string(old))
			stdout := captureStdout(t, func() {
				if err := miss([]string{"-failed", failed, "-run", mq}); err != nil {
					t.Fatal(err)
				}
			})
			var suspects []Suspect
			if err := json.Unmarshal([]byte(stdout), &suspects); err != nil || len(suspects) != 0 {
				t.Fatalf("control reported suspects: %s (%v)", stdout, err)
			}
		})
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan []byte)
	go func() {
		var b []byte
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			b = append(b, buf[:n]...)
			if err != nil {
				done <- b
				return
			}
		}
	}()
	f()
	w.Close()
	os.Stdout = old
	return string(<-done)
}
