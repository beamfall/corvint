package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// indexedTools are the corpus tools that answer from the derived query index.
var indexedTools = []struct{ name, args string }{
	{"corvint.docs_concept", `{"query":"checkout"}`},
	{"corvint.docs_flow", `{"id":"docs"}`},
	{"corvint.docs_claims", `{"path":"docs/p1.md"}`},
	{"corvint.docs_dependencies", `{"id":"docs"}`},
	{"corvint.docs_recommend_tests", `{"path":"docs/p1.md"}`},
	{"corvint.docs_gaps", `{}`},
	{"corvint.docs_intent", `{"id":"docs"}`},
	{"corvint.docs_navigation", `{"id":"docs"}`},
	{"corvint.docs_vocabulary", `{"query":"checkout"}`},
}

// indexedRegistry opens the lazy fixture and advertises every indexed tool,
// counting the index derivations the registry asks for.
func indexedRegistry(t *testing.T, derive func(context.Context, *doccorpus.Artifact) (*doccorpus.QueryIndex, error)) (*Registry, *int) {
	t.Helper()
	root, _ := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, tool := range indexedTools {
		name := doccorpus.CapabilityFor(tools[tool.name])
		found := false
		for i := range r.header.Capabilities {
			if r.header.Capabilities[i].Name == name {
				r.header.Capabilities[i].State, found = "present", true
			}
		}
		if !found {
			r.header.Capabilities = append(r.header.Capabilities, doccorpus.Capability{Name: name, State: "present"})
		}
	}
	calls := 0
	old := buildIndex
	buildIndex = func(ctx context.Context, a *doccorpus.Artifact) (*doccorpus.QueryIndex, error) {
		calls++
		return derive(ctx, a)
	}
	t.Cleanup(func() { buildIndex = old })
	return r, &calls
}

func batchArgs(n int) []byte {
	paths := []string{}
	for i := range n {
		paths = append(paths, fmt.Sprintf("docs/p%d.md", i))
	}
	args, _ := json.Marshal(map[string]any{"paths": paths})
	return args
}

// A capacity refusal is derived once per opened artifact and then returned
// for every indexed call and a whole paths batch without any index or
// freshness work.
func TestDCPV1195IndexRefusalDerivedOncePerArtifact(t *testing.T) {
	refusal := &doccorpus.Error{Code: doccorpus.CapacityCode, Message: "indexed postings 9000001 exceeds the limit of 8000000"}
	r, calls := indexedRegistry(t, func(context.Context, *doccorpus.Artifact) (*doccorpus.QueryIndex, error) { return nil, refusal })
	check := func(name string, args []byte) Timing {
		t.Helper()
		_, _, failure, protocol, timing := r.CallTimed(context.Background(), name, args)
		if protocol != nil || failure == nil || failure.Code != refusal.Code || failure.Message != refusal.Message {
			t.Fatalf("%s: want the cached capacity refusal, got %v %v", name, failure, protocol)
		}
		return timing
	}
	for i, tool := range indexedTools {
		timing := check(tool.name, []byte(tool.args))
		if i > 0 && (timing.Load != "reused" || timing.FreshnessMS != 0) {
			t.Fatalf("%s: refusal reloaded or computed freshness: %+v", tool.name, timing)
		}
	}
	// Derivation is counted, not timed: a wall-clock bound flaked under host
	// load (V1-2351), while a repeated derivation always moves the counter.
	if *calls != 1 {
		t.Fatalf("index derived %d times across the indexed tools, want once", *calls)
	}
	if timing := check("corvint.docs_claims", batchArgs(20)); timing.Load != "reused" || timing.FreshnessMS != 0 {
		t.Fatalf("20-path batch refusal reloaded or computed freshness: %+v", timing)
	}
	if *calls != 1 {
		t.Fatalf("index derived %d times after a 20-path batch, want once", *calls)
	}
	// Reads that need no index still answer.
	if _, _, failure, protocol := r.Call(context.Background(), "corvint.docs_locate", []byte(`{"path":"docs/p1.md"}`)); failure != nil || protocol != nil {
		t.Fatal(failure, protocol)
	}
	// A replaced artifact memo derives its own index.
	r.pins = append(r.pins, "0000000000000000000000000000000000000000")
	check("corvint.docs_gaps", []byte(`{}`))
	if *calls != 2 {
		t.Fatalf("replaced artifact reused the old index result: %d derivations", *calls)
	}
}

// An admitted index is derived once and reused, and each receipt is the one
// the query derives for itself; a cancelled derivation is not kept.
func TestDCPV1195IndexDerivedOncePerArtifact(t *testing.T) {
	cancelled := true
	r, calls := indexedRegistry(t, func(ctx context.Context, a *doccorpus.Artifact) (*doccorpus.QueryIndex, error) {
		if cancelled {
			cancelled = false
			return nil, context.Canceled
		}
		return doccorpus.BuildQueryIndex(ctx, a)
	})
	if _, _, failure, _ := r.Call(context.Background(), "corvint.docs_gaps", []byte(`{}`)); failure == nil {
		t.Fatal("cancelled derivation answered")
	}
	for _, tool := range indexedTools {
		_, text, failure, protocol := r.Call(context.Background(), tool.name, []byte(tool.args))
		if failure != nil || protocol != nil {
			t.Fatal(tool.name, failure, protocol)
		}
		var request doccorpus.Request
		if err := json.Unmarshal([]byte(tool.args), &request); err != nil {
			t.Fatal(err)
		}
		request.Operation, request.Limit = tools[tool.name], 20
		want, err := query(context.Background(), r.artifact, request, r.live.overlay)
		if err != nil || text != string(want) {
			t.Fatalf("%s: memoized index changed the receipt: %v\n%s\n%s", tool.name, err, text, want)
		}
	}
	if _, _, failure, protocol := r.Call(context.Background(), "corvint.docs_claims", batchArgs(3)); failure != nil || protocol != nil {
		t.Fatal(failure, protocol)
	}
	if *calls != 2 {
		t.Fatalf("index derived %d times, want once after the cancelled attempt", *calls)
	}
}
