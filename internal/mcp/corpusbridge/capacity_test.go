package corpusbridge

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func TestDCPV1158BridgeTypesGitCapacityLikeTheCLI(t *testing.T) {
	t.Run("DCP-V1-158 the bridge maps an exhausted Git budget to corpus-capacity-exceeded", func(t *testing.T) {
		code, message := failureCode(cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted"))
		if code != "corpus-capacity-exceeded" || message == "" {
			t.Fatalf("failureCode = %q %q", code, message)
		}
	})
}
