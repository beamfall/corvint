package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestPlanDeltaNamesNarrowedUnits_TSP_V0_666 compares two receipts of one
// path list, before and after a declaration change, and checks every delta
// list, then the refusals.
func TestPlanDeltaNamesNarrowedUnits_TSP_V0_666(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	before := write("before.json", `{"path_list":["a/a_test.go"],"plan":{"scope":"UNKNOWN","selected":[
		{"unitId":"go:a","witness":{"kind":"DIRECT_TEST_CHANGE"}},
		{"unitId":"go:reader","witness":{"kind":"UNBOUNDED_READER"}},
		{"unitId":"go:both","witness":{"kind":"UNBOUNDED_READER"}}],
		"unknown":[{"reason":"UNBOUNDED_READER","detail":"go:reader"},{"reason":"LANGUAGE_FRONTIER","detail":"x"}]}}`)
	after := write("after.json", `{"path_list":["a/a_test.go"],"plan":{"scope":"BOUNDED","selected":[
		{"unitId":"go:a","witness":{"kind":"DIRECT_TEST_CHANGE"}},
		{"unitId":"go:both","witness":{"kind":"DECLARED_READ_SCOPE"}},
		{"unitId":"go:new","witness":{"kind":"DECLARED_READ_SCOPE"}}],
		"unknown":[{"reason":"LANGUAGE_FRONTIER","detail":"x"}]}}`)
	var out bytes.Buffer
	if err := run([]string{before, after}, &out); err != nil {
		t.Fatal(err)
	}
	var got delta
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ScopeBefore != "UNKNOWN" || got.ScopeAfter != "BOUNDED" || got.SelectedBefore != 3 || got.SelectedAfter != 3 {
		t.Errorf("summary %+v", got)
	}
	if !slices.Equal(got.Removed, []unit{{"go:reader", "UNBOUNDED_READER"}}) || !slices.Equal(got.Added, []unit{{"go:new", "DECLARED_READ_SCOPE"}}) ||
		!slices.Equal(got.Rewitnessed, []unit{{"go:both", "UNBOUNDED_READER -> DECLARED_READ_SCOPE"}}) {
		t.Errorf("units %+v", got)
	}
	if !slices.Equal(got.UnknownRemoved, []unknown{{"UNBOUNDED_READER", "go:reader"}}) || len(got.UnknownAdded) != 0 {
		t.Errorf("unknowns %+v", got)
	}

	other := write("other.json", `{"path_list":["b"],"plan":{"scope":"BOUNDED"}}`)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{before}, "usage"},
		{[]string{before, other}, "different path lists"},
		{[]string{before, write("bare.json", `{"ok":false}`)}, "no plan.scope"},
		{[]string{before, write("bad.json", `{`)}, "bad.json"},
		{[]string{before, filepath.Join(dir, "absent.json")}, "absent.json"},
	} {
		if err := run(tc.args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: error %v, want %q", tc.args, err, tc.want)
		}
	}
}
