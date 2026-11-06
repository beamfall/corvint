// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
)

// IDX-SNAP-V0-7804: profile the context consumer after fixture/index setup,
// through its public deferred loader. This test also runs against the isolated
// /3 source comparator, so the extraction and consumer remain the same.
func TestDocperfSnapshotProfile(t *testing.T) {
	root, path := os.Getenv("CORVINT_DOCPERF_ROOT"), os.Getenv("CORVINT_DOCPERF_PROFILE")
	if root == "" || path == "" {
		t.Skip("set CORVINT_DOCPERF_ROOT and CORVINT_DOCPERF_PROFILE")
	}
	t.Setenv("CORVINT_INDEX_STORE", t.TempDir())
	if os.Getenv("CORVINT_DOCPERF_PACK") != "" {
		t.Setenv(snapshotFormatEnv, packFormatValue)
	} else {
		t.Setenv(snapshotFormatEnv, "")
	}
	ctx := context.Background()
	index, err := Build(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	index.Release()
	index = nil
	runtime.GC()
	defer resetPackRetention()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := pprof.StartCPUProfile(file); err != nil {
		t.Fatal(err)
	}
	defer pprof.StopCPUProfile()
	for range 10 {
		resetPackRetention() // a CLI invocation cannot reuse a prior process's mapping
		loaded, hit, err := LoadSnapshotDeferred(ctx, root)
		if err != nil || !hit {
			t.Fatalf("load hit=%v error=%v", hit, err)
		}
		packet, err := TaskContext(ctx, loaded, "snapshot decode", "", 20)
		if err != nil {
			loaded.Release()
			t.Fatal(err)
		}
		if _, err := json.Marshal(packet); err != nil {
			loaded.Release()
			t.Fatal(err)
		}
		if err := loaded.SnapshotRefusal(); err != nil {
			loaded.Release()
			t.Fatal(err)
		}
		loaded.Release()
	}
}
