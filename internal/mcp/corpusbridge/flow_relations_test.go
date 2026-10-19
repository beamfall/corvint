package corpusbridge

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func TestDCPV1098TraceToolDepth(t *testing.T) {
	r := &Registry{header: header{Schema: doccorpus.SchemaV2, Capabilities: []doccorpus.Capability{{Name: "subjects", State: "present"}}}}
	for _, tool := range r.Tools() {
		props := tool.InputSchema["properties"].(map[string]any)
		depth, ok := props["depth"].(map[string]any)
		if ok != (tool.Name == "corvint.docs_trace") {
			t.Fatalf("depth schema on %s: %v", tool.Name, ok)
		}
		if ok && (depth["minimum"] != 1 || depth["maximum"] != doccorpus.MaxFlowDepth) {
			t.Fatalf("depth bounds: %v", depth)
		}
	}
	for name, args := range map[string]string{
		"corvint.docs_get":   `{"id":"adapter:flow","depth":2}`,
		"corvint.docs_trace": `{"id":"adapter:flow","depth":0}`,
	} {
		if _, _, _, err := r.Call(context.Background(), name, []byte(args)); err == nil || err.Code != "invalid-arguments" {
			t.Fatalf("%s %s admitted: %v", name, args, err)
		}
	}
	for _, args := range []string{`{"id":"adapter:flow","depth":17}`, `{"id":"adapter:flow","depth":-1}`, `{"id":"adapter:flow","depth":null}`} {
		if _, _, _, err := r.Call(context.Background(), "corvint.docs_trace", []byte(args)); err == nil || err.Code != "invalid-arguments" {
			t.Fatalf("%s admitted: %v", args, err)
		}
	}
}
