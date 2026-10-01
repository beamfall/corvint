package doccorpus

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCorpusQueryIndexBounds(t *testing.T) {
	t.Run("DCP-V1-040 mandatory index admission bounds", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			artifact *Artifact
			want     string
		}{
			{"missing identity", &Artifact{Subjects: []Subject{{}}}, "identity missing"},
			{"duplicate identity", &Artifact{Subjects: []Subject{{ID: "same"}, {ID: "same"}}}, "duplicate indexed"},
			{"one over subjects", &Artifact{Subjects: make([]Subject, MaxCorpusRecords+1)}, "record family"},
			{"one over claims", &Artifact{Claims: make([]Claim, MaxCorpusRecords+1)}, "record family"},
			{"one over relations", &Artifact{Relations: make([]Relation, MaxCorpusRecords+1)}, "record family"},
			{"one over journeys", &Artifact{Journeys: make([]Journey, MaxCorpusJourneys+1)}, "record family"},
			{"one over observations", &Artifact{Observations: make([]Observation, MaxRecords+1)}, "record family"},
			{"one over key", &Artifact{Subjects: []Subject{{ID: strings.Repeat("k", 65537)}}}, "key bound"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := BuildQueryIndex(context.Background(), tc.artifact); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatal("wrong admission boundary", err)
				}
			})
		}
		// IDs with digits contribute one lexical term apiece. This control
		// reaches the full family bound without overflowing the other counters.
		t.Run("family boundary accepted", func(t *testing.T) {
			a := &Artifact{Subjects: make([]Subject, MaxCorpusRecords)}
			for i := range a.Subjects {
				a.Subjects[i].ID = fmt.Sprintf("s%06d", i)
			}
			if _, err := BuildQueryIndex(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("unique key byte boundary", func(t *testing.T) {
			a := &Artifact{Subjects: []Subject{}}
			// Unique full-length anchor keys cross 16 MiB well before the
			// posting or record-family bound. IDs and lexical terms also count.
			for i := 0; i < 257; i++ {
				a.Subjects = append(a.Subjects, Subject{ID: fmt.Sprintf("s%03d", i), Evidence: Evidence{Anchors: []Anchor{{Path: fmt.Sprintf("%03d", i) + strings.Repeat("x", 65533)}}}})
			}
			if _, err := BuildQueryIndex(context.Background(), a); err == nil || !strings.Contains(err.Error(), "key byte") {
				t.Fatal("key-byte guard not reached", err)
			}
			a.Subjects = a.Subjects[:255]
			if _, err := BuildQueryIndex(context.Background(), a); err != nil {
				t.Fatal("below key-byte bound refused", err)
			}
		})
		t.Run("posting boundary", func(t *testing.T) {
			a := &Artifact{Subjects: make([]Subject, MaxCorpusRecords)}
			for i := range a.Subjects {
				a.Subjects[i] = Subject{ID: fmt.Sprintf("s%06d", i)}
				for j := 0; j < 9; j++ {
					a.Subjects[i].Evidence.Anchors = append(a.Subjects[i].Evidence.Anchors, Anchor{Path: fmt.Sprintf("shared%d", j)})
				}
			}
			if _, err := BuildQueryIndex(context.Background(), a); err == nil || !strings.Contains(err.Error(), "postings") {
				t.Fatal("posting guard not reached", err)
			}
			a.Subjects = a.Subjects[:90000]
			if _, err := BuildQueryIndex(context.Background(), a); err != nil {
				t.Fatal("below posting bound refused", err)
			}
		})
		t.Run("path fanout across families", func(t *testing.T) {
			a := &Artifact{Subjects: make([]Subject, 50000), Claims: make([]Claim, 50000)}
			ev := Evidence{Anchors: []Anchor{{Path: "shared.go"}}}
			for i := range a.Subjects {
				a.Subjects[i] = Subject{ID: fmt.Sprintf("s%06d", i), Evidence: ev}
				a.Claims[i] = Claim{ID: fmt.Sprintf("c%06d", i), Subject: "s000000", Evidence: ev}
			}
			if _, err := BuildQueryIndex(context.Background(), a); err != nil {
				t.Fatal("exact fanout limit refused", err)
			}
			a.Claims = append(a.Claims, Claim{ID: "extra", Subject: "s000000", Evidence: ev})
			if _, err := BuildQueryIndex(context.Background(), a); err == nil || !strings.Contains(err.Error(), "fanout") {
				t.Fatal("path fanout guard not reached", err)
			}
		})
	})
}
