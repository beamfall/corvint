package corpusbridge

import (
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func TestStabilityToolIsCapabilityGated(t *testing.T) {
	t.Run("DCP-V1-025 MCP stability join", func(t *testing.T) {
		registry := &Registry{artifact: &doccorpus.Artifact{Capabilities: []doccorpus.Capability{{Name: "stability", State: "present"}}}}
		for _, tool := range registry.Tools() {
			if tool.Name == "corvint.docs_get_stability" {
				return
			}
		}
		t.Fatal("stability evidence capability did not expose its MCP join")
	})
}
