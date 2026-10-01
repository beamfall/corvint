package corpusbridge

import (
	"github.com/Beamfall/corvint/internal/doccorpus"
	"testing"
)

func TestCorpusCompleteReadAndTypedToolSchema(t *testing.T) {
	t.Run("DCP-V1-038 all CLI reads and typed capabilities", func(t *testing.T) {
		caps := []doccorpus.Capability{}
		for _, name := range []string{"info", "subjects", "claims", "relations", "journeys", "coverage", "gaps", "stability"} {
			caps = append(caps, doccorpus.Capability{Name: name, State: "present"})
		}
		r := &Registry{artifact: &doccorpus.Artifact{Schema: doccorpus.SchemaV2, Capabilities: caps}}
		seen := map[string]bool{}
		for _, tool := range r.Tools() {
			seen[tool.Name] = true
			op := tools[tool.Name]
			props := tool.InputSchema["properties"].(map[string]any)
			if props["retirement"] == nil {
				t.Fatal("trust policy schema missing")
			}
			if kind := doccorpus.OperationInput(op); kind != "" && props[kind] == nil {
				t.Fatal("typed required input missing", op)
			}
		}
		for name, op := range tools {
			if !seen[name] {
				t.Fatal("CLI operation absent MCP", op)
			}
		}
		r.artifact.Capabilities = []doccorpus.Capability{{Name: "subjects", State: "absent"}}
		if len(r.Tools()) != 0 {
			t.Fatal("absent capability exposed")
		}
	})
}
