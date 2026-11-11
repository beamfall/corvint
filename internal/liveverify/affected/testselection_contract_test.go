// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"reflect"
	"strings"
	"testing"
)

// TestCMPV1003TestSelectionFileContract (CMP-V1-003 item 3, CMP-V1-004; V1-5434): the documented
// file .corvint/test-selection.json (corvint-test-selection/0) keeps every member the 1.x
// compatibility promise freezes. A declaration naming each member reads back unchanged, so a
// renamed or removed member is refused as unknown; an unknown member or another profile is
// refused.
func TestCMPV1003TestSelectionFileContract(t *testing.T) {
	const declaration = `{"profile":"corvint-test-selection/0",` +
		`"owners":[{"paths":"docs/**","tests":["tests/docs.spec.ts"]}],` +
		`"mustRun":[{"tests":"smoke/","reason":"release smoke suite"}]}`
	got, err := readTestSelection(memorySource(t, map[string]string{DeclaredSelectionPath: declaration}))
	if err != nil {
		t.Fatal(err)
	}
	want := &testSelection{
		Owners:  []TestOwner{{Paths: "docs/**", Tests: []string{"tests/docs.spec.ts"}}},
		MustRun: []MustRunEntry{{Tests: "smoke/", Reason: "release smoke suite"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read %+v, want %+v", got, want)
	}
	for label, body := range map[string]string{
		"unknown member": strings.Replace(declaration, `"owners":`, `"added":1,"owners":`, 1),
		"other profile":  strings.Replace(declaration, DeclaredSelectionProfile, "corvint-test-selection/1", 1),
	} {
		if _, err := readTestSelection(memorySource(t, map[string]string{DeclaredSelectionPath: body})); err == nil {
			t.Errorf("%s: accepted", label)
		}
	}
}
