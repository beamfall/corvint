// SPDX-License-Identifier: Apache-2.0
package runnerselection

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// Replay the existing index adapter, including its missing identities and
// components. No fixture manufactures qualifications for a legacy record.
func TestRSPV0040LegacyIndexShadowReplay(t *testing.T) {
	in := fixture()
	ix, err := linecov.Build("example.test/module", in.Base, "legacy-runner", map[string]string{"source.go": in.Base}, []linecov.Record{{Package: "suite", Name: "test", Profile: []byte("mode: set\nexample.test/module/source.go:2.1,4.20 1 1\n")}})
	if err != nil {
		t.Fatal(err)
	}
	var before bytes.Buffer
	if err := ix.Encode(&before); err != nil {
		t.Fatal(err)
	}
	records, err := ix.EvidenceRecords("repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("legacy records: %d", len(records))
	}
	in.Runners[0].Records = []Evidence{{Record: records[0], Admitted: true}}
	p := Select(in)
	if !p.Shadow || len(p.Runners[0].Excluded) != 0 || len(p.Runners[0].Selected) != 1 {
		t.Fatalf("legacy evidence narrowed: %+v", p)
	}
	var after bytes.Buffer
	if err := ix.Encode(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("shadow mutated legacy index")
	}
}
func TestRSPV0040InvalidLegacyIndexCannotNarrow(t *testing.T) {
	in := fixture()
	ix := linecov.Index{Commit: in.Base, Evidence: []testevidence.Record{{}}}
	if _, err := ix.EvidenceRecords("repo"); err == nil {
		t.Fatal("invalid evidence admitted")
	}
	in.Runners[0].IndexVerified = false
	full(t, in, "index-missing-stale-or-unverifiable")
}
