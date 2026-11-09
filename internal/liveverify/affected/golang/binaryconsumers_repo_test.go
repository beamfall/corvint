package golang

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-762 (V1-1017): this repository declares the web-flows e2e scripts,
// which script/web-flows-gate runs against the built cmd/corvint-web-flows
// binary, as its consumers. They also run cmd/corvint, but nearly every change
// reaches that build, so declaring it would name them on almost every plan.
func TestRepositoryDeclaresWebFlowsConsumers_AFPV0762(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	consumers, err := readBinaryConsumers(affected.DiskSource(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{"tools/web-flows/test/e2e.mjs", "tools/web-flows/test/navigation-e2e.mjs"} {
		if !slices.Equal(consumers[script], []string{"cmd/corvint-web-flows"}) {
			t.Errorf("%s consumes %v, want cmd/corvint-web-flows", script, consumers[script])
		}
	}
}
