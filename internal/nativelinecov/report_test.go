// SPDX-License-Identifier: Apache-2.0
package nativelinecov

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// LPCV-V0-8206: malformed, unbound and oversized reports never yield partial
// coverage; JaCoCo and Cobertura keep zero-hit lines in the same index format.
func TestNativeReports(t *testing.T) {
	for _, format := range []string{"jacoco", "cobertura", "lcov"} {
		t.Run(format, func(t *testing.T) {
			r := reports(format, "src", 1)
			lines, err := Decode(r, map[string]string{"src": "src"})
			if err != nil {
				t.Fatal(err)
			}
			if !lines["src"][3] || lines["src"][4] || len(lines["src"]) != 2 {
				t.Fatalf("lines %v", lines)
			}
			ix, err := linecov.Build("m", "commit", "native", map[string]string{"src": "blob"}, []linecov.Record{{Package: "tests", Name: "alpha", Profile: lines.Profile("m")}})
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if err = ix.Encode(&b); err != nil {
				t.Fatal(err)
			}
			got, err := linecov.Decode(&b)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ix.Files, got.Files) {
				t.Fatal("record format changed")
			}
			for _, sources := range []map[string]string{nil, {"src": "../outside"}, {"src": "/outside"}, {"src": "C:\\outside"}} {
				if lines, err := Decode(r, sources); err == nil || lines != nil {
					t.Fatal("unmapped source accepted")
				}
			}
			if _, err := Decode(Report{Format: format, Data: r.Data[:len(r.Data)-4]}, map[string]string{"src": "src"}); err == nil {
				t.Fatal("truncated report accepted")
			}
			if _, err := Decode(Report{Format: format, Data: bytes.Repeat([]byte("x"), MaxReportBytes+1)}, nil); err == nil {
				t.Fatal("oversized report accepted")
			}
		})
	}
	for _, data := range []string{"SF:src\nDA:3,-1\nend_of_record\n", "SF:src\nDA:0,1\nend_of_record\n", "SF:src\nDA:3,1\n", "DA:3,1\nend_of_record\n", "SF:src\nDA:3,1\nSF:child\nDA:4,1\nend_of_record\n", "SF:src\nWHAT:1\nend_of_record\n"} {
		if _, err := Decode(Report{Format: "lcov", Data: []byte(data)}, map[string]string{"src": "src", "child": "child"}); err == nil {
			t.Fatalf("malformed LCOV accepted %q", data)
		}
	}
	for _, bad := range []string{
		`<report><package><sourcefile name="src"><line nr="3" ci="1"/></sourcefile></package></report>`,
		`<report><package><sourcefile name="src"><line nr="3" ci="-1" mi="0" mb="0" cb="0"/></sourcefile></package></report>`,
		`<report><package><sourcefile name="src"><line nr="3" nr="4" ci="1" mi="0" mb="0" cb="0"/></sourcefile></package></report>`,
		`<!DOCTYPE report [<!ENTITY x SYSTEM "file:///etc/passwd">]><report/>`,
		`<report/><report/>`,
	} {
		if _, err := Decode(Report{Format: "jacoco", Data: []byte(bad)}, map[string]string{"src": "src"}); err == nil {
			t.Fatalf("malformed XML accepted %s", bad)
		}
	}
	r := reports("jacoco", "src", 1)
	r.Data = append([]byte(`<?xml version="1.0"?><!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">`), r.Data...)
	if _, err := Decode(r, map[string]string{"src": "src"}); err != nil {
		t.Fatal(err)
	}
}

// LPCV-V0-8207: fixture fault matrix, not live LPCV-V0-963 qualification.
// Each hypothetical mutant's failing set is explicitly retained and must be
// included in the candidate selection. No native toolchain output is faked.
func TestSelectionFaults(t *testing.T) {
	for _, family := range []string{"jvm", "dotnet", "cfamily", "rust", "swift"} {
		t.Run(family, func(t *testing.T) {
			format := "lcov"
			if family == "jvm" {
				format = "jacoco"
			}
			if family == "dotnet" {
				format = "cobertura"
			}
			records := []linecov.Record{}
			for _, test := range []struct {
				name string
				hit  int
			}{{"alpha", 1}, {"beta", 0}} {
				lines, err := Decode(reports(format, "src", test.hit), map[string]string{"src": "src"})
				if err != nil {
					t.Fatal(err)
				}
				// A child-only source is reached only by alpha.
				child, err := Decode(reports(format, "child", test.hit), map[string]string{"child": "child"})
				if err != nil {
					t.Fatal(err)
				}
				lines.Merge(child)
				records = append(records, linecov.Record{Package: "tests", Name: test.name, Profile: lines.Profile("m")})
			}
			ix, err := linecov.Build("m", "commit", "fixture", map[string]string{"src": "blob", "child": "child-blob"}, records)
			if err != nil {
				t.Fatal(err)
			}
			bodies := map[string]Bodies{"src": {OID: "blob", Spans: []Body{{Open: 1, Close: 8}}}, "child": {OID: "child-blob", Spans: []Body{{Open: 1, Close: 8}}}}
			for _, c := range []struct {
				name, file, oid string
				lo, hi          int
				want            []string
				fallback        bool
			}{
				{"statement deletion", "src", "blob", 3, 3, []string{"alpha"}, false},
				{"constant change negative control", "src", "blob", 4, 4, nil, false},
				{"condition negation gap", "src", "blob", 5, 5, []string{"alpha"}, false},
				{"declaration", "src", "blob", 1, 1, nil, true},
				{"insertion neighbour", "src", "blob", 3, 2, []string{"alpha"}, false},
				{"subprocess only", "child", "child-blob", 3, 3, []string{"alpha"}, false},
				{"stale", "src", "other-blob", 3, 3, nil, true},
				{"new file", "missing", "", 3, 3, nil, true},
			} {
				t.Run(c.name, func(t *testing.T) {
					e := Analyze(ix, []linecov.FileDiff{{Path: c.file, OldOID: c.oid, NewOID: "new", Hunks: []linecov.Hunk{{Start: c.lo, End: c.hi}}}}, bodies)
					if (e.Fallback != "") != c.fallback {
						t.Fatalf("fallback %s", e.Fallback)
					}
					var got []string
					for i, test := range ix.Tests {
						if e.Tests[int32(i)] {
							got = append(got, test.Name)
						}
					}
					if !reflect.DeepEqual(got, c.want) {
						t.Fatalf("selected %v want failing set %v", got, c.want)
					}
				})
			}
			if e := Analyze(ix, []linecov.FileDiff{{Path: "src", OldOID: "blob", NewOID: "new", Hunks: []linecov.Hunk{{Start: 3, End: 3}}}}, nil); !strings.Contains(e.Fallback, "boundaries") {
				t.Fatal("missing parser result narrowed")
			}
		})
	}
}
