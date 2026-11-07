// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/clierror"
)

// ACX-V0-7707: preserve exact recovery reasons across the shared CLI boundary.
func TestACXV07707OutdatedMapEnvelope(t *testing.T) {
	const message = "cannot read CEM map: the existing map records a different base or patch; pass --replace to regenerate"
	var stderr bytes.Buffer
	clierror.Write(&stderr, "corvint", "map-unavailable", message, map[string]any{"evidence": "retained"})
	if !outdatedCEMRefusal(stderr.Bytes()) {
		t.Fatalf("framing metadata hid the admitted recovery: %s", stderr.Bytes())
	}
	for _, raw := range []string{
		`{"code":"map-unavailable","error":"invalid map bytes","ok":false}`,
		`{"code":"other","error":"` + message + `","ok":false}`,
		`{"code":"map-unavailable","error":"` + message + `","ok":true}`,
		`{"code":"map-unavailable","error":"` + message + `"}`,
	} {
		if outdatedCEMRefusal([]byte(raw)) {
			t.Fatalf("unadmitted replacement: %s", raw)
		}
	}
}
