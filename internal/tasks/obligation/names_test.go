package obligation

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

func aliasLedger() *ticket.ObligationLedger {
	return &ticket.ObligationLedger{Prefix: "AC", Entries: []ticket.ObligationEntry{
		{ID: "AC-1", Alias: "M001"}, {ID: "AC-2", Alias: "ABC-12.V4"}, {ID: "AC-3", Alias: "login:M7"}, {ID: "AC-4"},
	}}
}

// TestTOLV0036_AliasWholeToken: an alias names its native id only as a whole
// run of the alias alphabet, trailing `.` and `:` dropped; a native id still
// matches by TOL-V0-010; every result is a native id.
func TestTOLV0036_AliasWholeToken(t *testing.T) {
	n := NamesOf(aliasLedger())
	cases := map[string][]string{
		"M001: login":               {"AC-1"},
		"[M001] login.":             {"AC-1"},
		"ends with M001.":           {"AC-1"},
		"M001-login":                nil,
		"xM001 M001x M001.1 a.M001": nil,
		"M001_extra _M001":          nil,
		"ABC-12.V4 export":          {"AC-2"},
		"ABC-12 export":             nil,
		"see login:M7 now":          {"AC-3"},
		"M7":                        nil,
		"AC-1 and M001":             {"AC-1"},
		"AC-4 then M001":            {"AC-4", "AC-1"},
	}
	for s, want := range cases {
		if got := n.Find(s); !reflect.DeepEqual(got, want) {
			t.Errorf("Find(%q) = %v, want %v", s, got, want)
		}
	}
	contains := []struct {
		content, id string
		want        bool
	}{
		{"test('M001: x')", "AC-1", true},
		{"test('xM001')", "AC-1", false},
		{"test('M001-x')", "AC-1", false},
		{"test('M001_x')", "AC-1", false},
		{"AC-1", "AC-1", true},
		{"x ABC-12.V4. y", "AC-2", true},
		{"ABC-12.V4x", "AC-2", false},
		{"M001", "AC-4", false},
	}
	for _, c := range contains {
		if got := n.Contains([]byte(c.content), c.id); got != c.want {
			t.Errorf("Contains(%q, %s) = %v, want %v", c.content, c.id, got, c.want)
		}
	}
}

// TestTOLV0036_AliasMasksDefectIDs: a defect-shaped part of an alias is not
// a defect id; a real defect id beside it still is.
func TestTOLV0036_AliasMasksDefectIDs(t *testing.T) {
	n := NamesOf(aliasLedger())
	if got := n.DefectIDs("ABC-12.V4 expected-fail BUG-17"); !reflect.DeepEqual(got, []string{"BUG-17"}) {
		t.Fatalf("DefectIDs = %v", got)
	}
	if got := DefectIDs("ABC-12.V4 expected-fail", "AC"); !reflect.DeepEqual(got, []string{"ABC-12"}) {
		t.Fatalf("unmasked DefectIDs = %v", got)
	}
}

// TestTOLV0036_ContractAliasAndNamedTests: `<contract>:<alias>` names the
// obligation in a string literal, and NamedTests reports native ids for
// alias-named titles and steps.
func TestTOLV0036_ContractAliasAndNamedTests(t *testing.T) {
	n := NamesOf(aliasLedger())
	if got := n.contractAliases("rows:M001 and flows:ABC-12.V4. and x:M001x and y:M001_z"); !reflect.DeepEqual(got, []string{"AC-1", "AC-2"}) {
		t.Fatalf("contractAliases = %v", got)
	}
	src := "import { test } from '@playwright/test';\n" +
		"test('M001: login', async () => { await test.step('ABC-12.V4 export', async () => {}); });\n" +
		"test('M001-legacy', async () => {});\n"
	got := NamedTests(map[string][]byte{"e2e/a.spec.ts": []byte(src)}, n)
	if len(got) != 1 || !reflect.DeepEqual(got[0].IDs, []string{"AC-1", "AC-2"}) {
		t.Fatalf("NamedTests = %+v", got)
	}
}
