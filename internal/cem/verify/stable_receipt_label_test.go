package verify

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1312OpaqueReceiptLabelVerifies proves the native stable verifier
// ACCEPTs the public neutral case with its runner receipt labelled opaque/0,
// with the same result axes as the legacy-labelled public case, so the label
// raises nothing; the portable consumer asserts the same (CEM-V1-312).
func TestCEMV1312OpaqueReceiptLabelVerifies(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := loadNeutralCase(t)
	repo, base, target := c.materialize(t)
	legacy := []byte(`"profile":"` + wire.StableRunnerReceiptProfile + `"`)
	if bytes.Count(c.raw, legacy) != 1 {
		t.Fatal("public case does not carry exactly one legacy runner label")
	}
	raw := bytes.Replace(c.raw, legacy, []byte(`"profile":"`+wire.StableOpaqueReceiptProfile+`"`), 1)
	result, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root})
	got := neutralResult(t, result)
	if exit != 0 || result.Outcome != "ACCEPT" || !reflect.DeepEqual(got["axes"], c.expect.Accept.Result["axes"]) {
		t.Fatalf("Stable exit=%d result=%s, want ACCEPT with the public case axes", exit, stableResultDebug(result))
	}
}
