// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The two profiles of .corvint/test-selection.json, spelled out so that a
// renamed constant cannot rename a frozen label unseen (CMP-V1-006).
const (
	contractProfile0 = "corvint-test-selection/0"
	contractProfile2 = "corvint-test-selection/2"
)

// contractSelection0 and contractSelection2 name every documented member of
// their profile (AFP-V0-2480, AZU-V0-8921).
const (
	contractSelection0 = `{"profile":"corvint-test-selection/0",` +
		`"owners":[{"paths":"docs/**","tests":["tests/docs.spec.ts"]}],` +
		`"mustRun":[{"tests":"smoke/","reason":"release smoke suite"}]}`
	contractSelection2 = `{"profile":"corvint-test-selection/2",` +
		`"owners":[{"paths":"docs/**","tests":["tests/docs.spec.ts"]}],` +
		`"mustRun":[{"tests":"smoke/","reason":"release smoke suite"}],` +
		`"treeReaders":[{"runs":"script/check-eol.sh","reason":"lists every tracked path"}]}`
)

// TestCMPV1003TestSelectionFileContract (CMP-V1-003 item 3, CMP-V1-004; V1-5434, V1-4685): the
// documented file .corvint/test-selection.json keeps every member the 1.x compatibility promise
// freezes, under both of its profiles. corvint-test-selection/0 holds owners and mustRun;
// corvint-test-selection/2 adds treeReaders (AZU-V0-8921) and changes nothing else. A declaration
// naming each member reads back unchanged, so a renamed or removed member is refused as unknown;
// an unknown member or another profile is refused. V1-4685 deliberately widened this contract from
// /0 alone: AFP-V0-2480, the `corvint-dir-files` row of the compatibility table and this test
// change together.
func TestCMPV1003TestSelectionFileContract(t *testing.T) {
	if DeclaredSelectionProfile != contractProfile0 || DeclaredTreeReaderSelectionProfile != contractProfile2 {
		t.Fatalf("profile labels are %q and %q; the frozen labels are %q and %q (CMP-V1-006)",
			DeclaredSelectionProfile, DeclaredTreeReaderSelectionProfile, contractProfile0, contractProfile2)
	}
	owners := []TestOwner{{Paths: "docs/**", Tests: []string{"tests/docs.spec.ts"}}}
	mustRun := []MustRunEntry{{Tests: "smoke/", Reason: "release smoke suite"}}
	for label, tc := range map[string]struct {
		declaration string
		want        *testSelection
	}{
		"profile 0": {contractSelection0, &testSelection{Owners: owners, MustRun: mustRun}},
		"profile 2": {contractSelection2, &testSelection{Owners: owners, MustRun: mustRun,
			TreeReaders: []TreeReader{{Runs: "script/check-eol.sh", Reason: "lists every tracked path"}}}},
		// A /2 file without treeReaders means what the /0 file means.
		"profile 2 without treeReaders": {strings.Replace(contractSelection0, contractProfile0, contractProfile2, 1),
			&testSelection{Owners: owners, MustRun: mustRun}},
	} {
		got, err := readTestSelection(memorySource(t, map[string]string{DeclaredSelectionPath: tc.declaration}))
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: read %+v, want %+v", label, got, tc.want)
		}
	}
	for label, tc := range map[string]struct{ body, want string }{
		"profile 0, unknown member":                  {strings.Replace(contractSelection0, `"owners":`, `"added":1,"owners":`, 1), `unknown object member name "added"`},
		"profile 2, unknown member":                  {strings.Replace(contractSelection2, `"owners":`, `"added":1,"owners":`, 1), `unknown object member name "added"`},
		"profile 2, unknown member of a tree reader": {strings.Replace(contractSelection2, `"runs":`, `"added":1,"runs":`, 1), `unknown object member name "added"`},
		// The refusal names both labels this reader accepts.
		"profile 1": {strings.Replace(contractSelection0, contractProfile0, "corvint-test-selection/1", 1), "profile must be corvint-test-selection/0 or corvint-test-selection/2"},
		"profile 3": {strings.Replace(contractSelection2, contractProfile2, "corvint-test-selection/3", 1), "profile must be corvint-test-selection/0 or corvint-test-selection/2"},
		// treeReaders is no member of the /0 object, even when empty.
		"profile 0 with treeReaders":       {strings.Replace(contractSelection2, contractProfile2, contractProfile0, 1), "treeReaders needs profile corvint-test-selection/2"},
		"profile 0 with empty treeReaders": {strings.Replace(contractSelection0, `"owners":`, `"treeReaders":[],"owners":`, 1), "treeReaders needs profile corvint-test-selection/2"},
	} {
		if _, err := readTestSelection(memorySource(t, map[string]string{DeclaredSelectionPath: tc.body})); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", label, err, tc.want)
		}
	}
}

// readerOfProfile0 is the reader every planner before corvint-test-selection/2 carries, kept here
// as the frozen /0 contract (AFP-V0-2480 before V1-4685): a closed object of profile, owners and
// mustRun, an unknown member refused, the profile compared with the one label it knows.
func readerOfProfile0(body string) error {
	var declared struct {
		Profile string         `json:"profile"`
		Owners  []TestOwner    `json:"owners"`
		MustRun []MustRunEntry `json:"mustRun"`
	}
	if err := json.UnmarshalRead(bytes.NewReader([]byte(body)), &declared, json.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("%s: %w", DeclaredSelectionPath, err)
	}
	if declared.Profile != contractProfile0 {
		return fmt.Errorf("%s: profile must be %s", DeclaredSelectionPath, contractProfile0)
	}
	return nil
}

// TestCMPV1004TestSelectionProfilesAcrossReaders (CMP-V1-004, AFP-V0-2480, AZU-V0-8921; V1-4685):
// what a reader of each profile does with the other profile's file, in both directions.
//
// Forward: this reader, which knows /2, reads every /0 file as the /0 reader reads it, to the same
// declaration and so the same graph digest.
//
// Backward: the /0 reader refuses every /2 file, with treeReaders (an unknown member) and without
// (another profile), so a planner that predates treeReaders never plans a tree whose declaration
// names whole-tree readers as if it named none. `corvint affected` reports that refusal as
// `unsupported-affected-graph`, exit 2.
func TestCMPV1004TestSelectionProfilesAcrossReaders(t *testing.T) {
	read := func(body string) (*testSelection, error) {
		return readTestSelection(memorySource(t, map[string]string{DeclaredSelectionPath: body}))
	}
	// Forward.
	if err := readerOfProfile0(contractSelection0); err != nil {
		t.Fatalf("the /0 reader refuses the /0 file: %v", err)
	}
	got, err := read(contractSelection0)
	if err != nil {
		t.Fatalf("the /2 reader refuses the /0 file: %v", err)
	}
	if len(got.TreeReaders) != 0 || len(got.Owners) != 1 || len(got.MustRun) != 1 {
		t.Fatalf("the /2 reader read the /0 file as %+v", got)
	}
	// Carried as the /0 reader carried it: no member the /0 digest lacks.
	carried, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"owners":[{"paths":"docs/**","tests":["tests/docs.spec.ts"]}],"mustRun":[{"tests":"smoke/","reason":"release smoke suite"}]}`; string(carried) != want {
		t.Fatalf("a /0 declaration is carried as %s, want %s", carried, want)
	}
	// Backward.
	for label, tc := range map[string]struct{ body, want string }{
		"with treeReaders":    {contractSelection2, `unknown object member name "treeReaders"`},
		"without treeReaders": {strings.Replace(contractSelection0, contractProfile0, contractProfile2, 1), "profile must be corvint-test-selection/0"},
	} {
		if _, err := read(tc.body); err != nil {
			t.Fatalf("%s: the /2 reader refuses its own file: %v", label, err)
		}
		if err := readerOfProfile0(tc.body); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: the /0 reader gave %v, want a refusal with %q", label, err, tc.want)
		}
	}
}
