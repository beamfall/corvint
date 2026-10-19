package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func TestDCPV1158CLITypesGitCapacityLikeTheBridge(t *testing.T) {
	t.Run("DCP-V1-158 the CLI maps an exhausted Git budget to corpus-capacity-exceeded, not internal-error", func(t *testing.T) {
		var out bytes.Buffer
		emitCorpusError(&out, cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted"))
		if !strings.Contains(out.String(), `"corpus-capacity-exceeded"`) || strings.Contains(out.String(), "internal-error") {
			t.Fatalf("emitCorpusError wrote %s", out.String())
		}
	})
}
