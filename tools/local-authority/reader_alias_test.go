package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestReaderAliasesRetainIdentityChecks(t *testing.T) {
	const appleAlias = "com.apple.idms.appleid.prd.000000-00-00000000-0000-4000-8000-000000000000"
	record := func(names []string) map[string][]string {
		return map[string][]string{"dsAttrTypeStandard:RecordName": names, "dsAttrTypeStandard:UniqueID": {"501"}, "dsAttrTypeStandard:GeneratedUID": {"22222222-2222-2222-2222-222222222222"}}
	}
	tooMany := []string{"tester"}
	for i := 0; i < 16; i++ {
		tooMany = append(tooMany, fmt.Sprintf("alias%d", i))
	}
	cases := []struct {
		name              string
		names             []string
		human             bool
		badNode, badField string
		failLookup        bool
		wantOK            bool
		wantCalls         int
	}{
		{name: "human singleton", names: []string{"tester"}, human: true, wantOK: true, wantCalls: 2},
		{name: "PLE-V0-011 reader alias identity", names: []string{"tester", appleAlias}, human: true, wantOK: true, wantCalls: 4},
		{name: "canonical second", names: []string{appleAlias, "tester"}, human: true, wantOK: true, wantCalls: 4},
		{name: "service singleton", names: []string{"tester"}, wantOK: true},
		{name: "service alias", names: []string{"tester", appleAlias}},
		{name: "missing", human: true},
		{name: "canonical absent", names: []string{appleAlias}, human: true},
		{name: "canonical wrong case", names: []string{"Tester"}, human: true},
		{name: "duplicate", names: []string{"tester", "tester"}, human: true},
		{name: "case collision", names: []string{"tester", "Tester"}, human: true},
		{name: "alias collision", names: []string{"tester", "Alias", "alias"}, human: true},
		{name: "path", names: []string{"tester", "../alias"}, human: true},
		{name: "empty", names: []string{"tester", ""}, human: true},
		{name: "control", names: []string{"tester", "a\n"}, human: true},
		{name: "space", names: []string{"tester", "a b"}, human: true},
		{name: "nonascii", names: []string{"tester", "é"}, human: true},
		{name: "overlong", names: []string{"tester", strings.Repeat("a", 257)}, human: true},
		{name: "too many", names: tooMany, human: true},
		{name: "local UID conflict", names: []string{"tester", appleAlias}, human: true, badNode: ".", badField: "UniqueID", wantCalls: 3},
		{name: "search UUID conflict", names: []string{"tester", appleAlias}, human: true, badNode: "/Search", badField: "GeneratedUID", wantCalls: 4},
		{name: "lookup failure", names: []string{"tester", appleAlias}, human: true, failLookup: true, wantCalls: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			lookup := func(node, path string) (map[string][]string, error) {
				calls++
				r := record([]string{"tester", appleAlias})
				if path == "/Users/"+appleAlias {
					if tc.failLookup {
						return nil, errors.New("unavailable")
					}
					if node == tc.badNode {
						r["dsAttrTypeStandard:"+tc.badField] = []string{"different"}
					}
				}
				return r, nil
			}
			err := validateReaderUser(record(tc.names), "tester", "501", "22222222-2222-2222-2222-222222222222", tc.human, lookup)
			if (err == nil) != tc.wantOK {
				t.Fatalf("error=%v wantOK=%v", err, tc.wantOK)
			}
			if calls != tc.wantCalls {
				t.Fatalf("lookup calls=%d want=%d", calls, tc.wantCalls)
			}
		})
	}
	for _, field := range []string{"UniqueID", "GeneratedUID"} {
		t.Run("original "+field+" drift", func(t *testing.T) {
			r := record([]string{"tester", appleAlias})
			r["dsAttrTypeStandard:"+field] = []string{"different"}
			err := validateReaderUser(r, "tester", "501", "22222222-2222-2222-2222-222222222222", true, func(string, string) (map[string][]string, error) {
				t.Fatal("lookup before identity validation")
				return nil, nil
			})
			if err == nil {
				t.Fatal("accepted original identity drift")
			}
		})
	}
}
