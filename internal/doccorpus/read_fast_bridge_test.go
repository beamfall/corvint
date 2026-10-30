package doccorpus_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// TestDCRV1416CorpusMCPFirstAndWarmCalls: on the large read fixture the corpus MCP server's first
// tool call admits the artifact through the verified read, so it costs a decode and a live tree
// observation rather than a compile, and warm calls reuse that state. The first call must beat
// the rederivation it replaced by at least 3x; timings are logged, and CORVINT_READ_FIXTURE_LARGE
// selects the scale the PR measurements use (DCR-V1-416, DCR-V1-418), and the residual
// measurements of GitHub #1598 re-measure the first and warm calls at that scale (DCR-V1-430).
func TestDCRV1416CorpusMCPFirstAndWarmCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("large fixture")
	}
	ctx := context.Background()
	root := doccorpus.LargeReadFixtureForTest(t, t.TempDir(), os.Getenv("CORVINT_READ_FIXTURE_LARGE") != "")
	registry, startup := corpusbridge.New(root, "corpus.json")
	if startup != nil {
		t.Fatal(startup)
	}
	call := func(tool, arguments string) corpusbridge.Timing {
		t.Helper()
		_, _, failure, protocol, timing := registry.CallTimed(ctx, tool, []byte(arguments))
		if failure != nil || protocol != nil {
			t.Fatalf("%s: %v %v", tool, failure, protocol)
		}
		return timing
	}
	first := call("corvint.docs_search", `{"query":"ledger","limit":20}`)
	if first.Load != "first-call" {
		t.Fatalf("first call load %q", first.Load)
	}
	warm := []int64{}
	var last corpusbridge.Timing
	for range 10 {
		for _, c := range [][2]string{{"corvint.docs_get", `{"id":"adapter:lib0-1"}`}, {"corvint.docs_search", `{"query":"ledger","limit":20}`}, {"corvint.docs_info", `{}`}} {
			timing := call(c[0], c[1])
			if timing.Load != "reused" {
				t.Fatalf("warm call load %q", timing.Load)
			}
			warm = append(warm, timing.TotalMS)
			last = timing
		}
	}
	slices.Sort(warm)
	data, err := doccorpus.ReadCorpusFile(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var timings doccorpus.ReadTimings
	if _, err := doccorpus.OpenRead(ctx, root, data, doccorpus.ReadOptions{Rederive: true, Timings: &timings}); err != nil {
		t.Fatal(err)
	}
	t.Logf("artifact %d bytes: first call %+v; warm total_ms p50=%d p95=%d max=%d (last %+v); rederivation %v", len(data), first, warm[len(warm)/2], warm[len(warm)*95/100], warm[len(warm)-1], last, timings.Verify)
	if first.TotalMS*3 > timings.Verify.Milliseconds() {
		t.Fatalf("first call %d ms is not at least 3x faster than rederivation %v", first.TotalMS, timings.Verify)
	}
}
