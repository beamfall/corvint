// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package testevidencedoc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runevidence"
)

// rvdReceipt is a corvint-test-run/0 receipt with one Go package outcome, in the shape
// sessiontest.Encode writes.
func rvdReceipt(commit, outcome string) []byte {
	return []byte(fmt.Sprintf(`{"base":%q,"checks":[],"graphDigest":"","incomplete":[],"invocations":[{"adapter":"go","adapterManifest":"m","argv":["go","test"],"elapsedMilliseconds":40,"env":[],"envDigest":"","exit":0,"outcomes":[{"durationGranularity":"file","durationMs":12,"durationSource":"go-test2json","outcome":%q,"slice":"whole","unit":"go:m/a"}],"runner":"/usr/bin/go","version":"go1.27.3"}],"manifestDigest":"","partitionDigest":"","placement":"universe","planDigest":"","profile":"corvint-test-run/0","reason":"","result":"PASS","revision":%q,"runnerProfile":"local","scope":"NARROWED","shardCount":1,"shardIndex":0,"source":{"head":%q,"paths":[],"tree":%q},"sourceDigest":"","summary":"","tool":"corvint test"}`+"\n",
		commit, outcome, commit, commit, strings.Repeat("1", 40)))
}

// RVD-V1-012: `corvint test-evidence` reads the store through storedRuns and writes nothing: the
// repository, its Git directory and every store file keep their bytes and modes, for an intact
// store and for a damaged one (a corrupt record and a torn log tail, which only a writer repairs).
func TestRVDV1012TestEvidenceReadsStoreWithoutWriting(t *testing.T) {
	root := repository(t)
	gitDir := filepath.Join(root, ".git")
	store := runevidence.Store{Dir: runevidence.Locate(gitDir)}
	commit := strings.Repeat("a", 40)
	if _, err := store.Append(rvdReceipt(commit, "PASS")); err != nil {
		t.Fatal(err)
	}
	broken, err := store.Append(rvdReceipt(commit, "FAIL"))
	if err != nil {
		t.Fatal(err)
	}
	query := Query{Test: "go:m/a", Revision: "HEAD"}
	for _, damaged := range []bool{false, true} {
		if damaged {
			record := filepath.Join(store.Dir, "records", broken+".json")
			if err := os.WriteFile(record, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(store.Dir, "log.jsonl")
			raw, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(log, append(raw, `{"id":"`...), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		before := snapshot(t, root)
		d := Read(context.Background(), root, query)
		if d.Runs == nil || len(d.Runs.Results) != 1 || d.Runs.Results[0].Test != "go:m/a" {
			t.Fatalf("damaged=%v runs %+v", damaged, d.Runs)
		}
		if damaged && (d.Runs.Results[0].Outcome != "passed" || len(d.Runs.CorruptRecords) != 1 || d.Runs.CorruptRecords[0] != broken) {
			t.Fatalf("damaged store runs %+v", d.Runs)
		}
		if !damaged && d.Runs.Results[0].Outcome != "failed" {
			t.Fatalf("intact store runs %+v", d.Runs)
		}
		if runs := storedRuns(context.Background(), root, d, ""); runs == nil {
			t.Fatal("storedRuns found no runs")
		}
		if after := snapshot(t, root); after != before {
			t.Fatalf("damaged=%v: test-evidence changed the repository or the store", damaged)
		}
	}
}
