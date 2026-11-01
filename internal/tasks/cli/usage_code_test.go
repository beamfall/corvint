package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0965_UsageRefusalCarriesMalformed: a usage refusal carries
// MALFORMED and retryable false, keeps outcome ERROR and exit 1 (V1-3644).
func TestCALV0965_UsageRefusalCarriesMalformed(t *testing.T) {
	x := atm(t, t.TempDir(), nil, "cutover")
	if x.code != 1 || x.res.Outcome != wire.OutcomeError || !wire.HasCode(x.res, wire.CodeMalformed) ||
		!strings.Contains(string(x.stdout), `"codes":["MALFORMED"]`) || !strings.Contains(string(x.stdout), `"retryable":false`) {
		t.Fatalf("usage refusal: exit %d %s", x.code, x.stdout)
	}
}
