package contextindex_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

const releaseContextArguments = `{"task":"change the cache demux","limit":5}`

// TestMCPV0090ContextCallReleasesItsSnapshotMapping: a long-lived bridge that
// answers corvint.context from a mapped pack snapshot holds no mapping once
// the retention ring lets go, and its results, encoded after the mapping is
// unmapped, are the bytes encoded while it was mapped. Sequential and
// concurrent calls run in one process (MCPV0-090, V1-1004).
func TestMCPV0090ContextCallReleasesItsSnapshotMapping(t *testing.T) {
	root := contextindex.PackFixtureRootForTest(t)
	contextindex.ResetSnapshotRetentionForTest()
	defer contextindex.ResetSnapshotRetentionForTest()
	base := contextindex.LiveSnapshotMappingsForTest()
	registry, err := bridge.NewTaskReview(root)
	if err != nil {
		t.Fatal(err)
	}
	call := func() (bridge.Result, []byte) {
		result, callErr := registry.Call(context.Background(), bridge.ToolContext, []byte(releaseContextArguments))
		if callErr != nil || result.State != "READY" {
			t.Errorf("context call = %+v, %v", result, callErr)
			return result, nil
		}
		encoded, encodeErr := result.CanonicalJSON()
		if encodeErr != nil {
			t.Errorf("encode: %v", encodeErr)
		}
		return result, encoded
	}
	first, want := call()
	if t.Failed() {
		t.FailNow()
	}
	if live := contextindex.LiveSnapshotMappingsForTest() - base; live != 1 {
		t.Fatalf("a mapped context call left %d live mappings, want the ring's 1 (did the call read the pack?)", live)
	}
	const workers, rounds = 6, 4
	results := make([]bridge.Result, workers*rounds)
	var pending sync.WaitGroup
	for worker := range workers {
		pending.Add(1)
		go func() {
			defer pending.Done()
			for round := range rounds {
				results[worker*rounds+round], _ = call()
			}
		}()
	}
	pending.Wait()
	if t.Failed() {
		t.FailNow()
	}
	contextindex.ResetSnapshotRetentionForTest()
	if live := contextindex.LiveSnapshotMappingsForTest() - base; live != 0 {
		t.Fatalf("%d mappings stay mapped after every context call returned and the ring was evicted", live)
	}
	for position, result := range append([]bridge.Result{first}, results...) {
		got, encodeErr := result.CanonicalJSON()
		if encodeErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("result %d encodes differently after the mapping was unmapped (%v)", position, encodeErr)
		}
	}
}

// TestMCPV0090RefusedSlotWeightsReleaseTheSnapshotMapping: a malformed or
// symlinked .context-corvint/slot-weights.json refuses after the mapped
// snapshot loaded; the refusal must still release that mapping's lease, so
// evicting the retention ring leaves nothing mapped (MCPV0-090, V1-1004).
func TestMCPV0090RefusedSlotWeightsReleaseTheSnapshotMapping(t *testing.T) {
	for _, form := range []string{"malformed", "symlink"} {
		t.Run(form, func(t *testing.T) {
			root := contextindex.PackFixtureRootForTest(t)
			contextindex.ResetSnapshotRetentionForTest()
			defer contextindex.ResetSnapshotRetentionForTest()
			base := contextindex.LiveSnapshotMappingsForTest()
			weights := filepath.Join(root, filepath.FromSlash(contextindex.SlotWeightsPath))
			if err := os.MkdirAll(filepath.Dir(weights), 0o755); err != nil {
				t.Fatal(err)
			}
			if form == "malformed" {
				if err := os.WriteFile(weights, []byte("{not json"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(t.TempDir(), "weights.json")
				if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, weights); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			}
			registry, err := bridge.NewTaskReview(root)
			if err != nil {
				t.Fatal(err)
			}
			for range 3 {
				result, callErr := registry.Call(context.Background(), bridge.ToolContext, []byte(releaseContextArguments))
				if callErr == nil && result.State == "READY" {
					t.Fatalf("context call with a %s slot-weights file answered READY: %+v", form, result)
				}
			}
			contextindex.ResetSnapshotRetentionForTest()
			if live := contextindex.LiveSnapshotMappingsForTest() - base; live != 0 {
				t.Fatalf("%d mappings stay mapped after %s slot-weights refusals and ring eviction", live, form)
			}
		})
	}
}
