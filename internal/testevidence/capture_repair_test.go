// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"reflect"
	"strings"
	"testing"
)

// RSP-V0-009, RSP-V0-054: modeling grants no permission to discard trace
// dependencies or to turn unobserved components into observed empty sets.
func TestRSPV0009ModeledBuildRetainsTraceAndUnion(t *testing.T) {
	for _, state := range []State{Present, Absent, Incomplete} {
		t.Run(string(state), func(t *testing.T) {
			r := fixture()
			template := blob()
			template.Path.Lexical, template.Path.Resolved = "generator.template", "generator.template"
			r.Reads.Items = []Blob{template}
			r.Listings.Items = []Path{{"repo", "templates", "templates"}}
			r.Lookups.Items = []Path{{"repo", "optional-template", "optional-template"}}
			r.Execs.Items = []Executable{{blob(), strings.Repeat("3", 64), false}}
			if state != Present {
				r.Reads = Component[Blob]{State: state, Reason: "unknown-build-read"}
			}
			loaded := Component[Blob]{State: Present, Items: []Blob{blob()}}
			got, err := Attribute(r, Attribution{Tests: []string{"a", "b"}, MarkedTest: "a", MarkersQualified: true,
				BuildPhase: true, BuildGraphModeled: true, BuildLoaded: loaded})
			if err != nil || len(got) != 2 {
				t.Fatalf("attribute: %+v %v", got, err)
			}
			for _, build := range got {
				want := r
				want.Test, want.Loaded = build.Test, loaded
				if !reflect.DeepEqual(build, want) {
					t.Fatalf("modeled build dropped evidence: got %+v want %+v", build, want)
				}
				// A later agreeing collection must preserve complete components;
				// an unknown build observation must remain unknown through union.
				u, err := Union(build, next(build))
				if err != nil {
					t.Fatal(err)
				}
				wantReads := build.Reads
				if state != Present {
					wantReads = Component[Blob]{State: Incomplete, Reason: "collection-component-incomplete"}
				}
				if !reflect.DeepEqual(u.Reads, wantReads) || !reflect.DeepEqual(u.Listings, build.Listings) ||
					!reflect.DeepEqual(u.Lookups, build.Lookups) || !reflect.DeepEqual(u.Execs, build.Execs) {
					t.Fatalf("union lost build trace: %+v", u)
				}
				ready, _ := u.CollectionReady(false)
				if ready != (state == Present) {
					t.Fatalf("readiness for %s: %v", state, ready)
				}
			}
		})
	}
}
