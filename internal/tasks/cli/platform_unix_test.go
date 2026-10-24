//go:build darwin || linux

package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-025: darwin and linux have no platform refusal, so a command reaches
// its own handling.
func TestCTSV0025_SupportedPlatformHasNoRefusal(t *testing.T) {
	if err := wire.PlatformRefusal(); err != nil {
		t.Fatalf("PlatformRefusal() = %v on a supported platform", err)
	}
	var stdout bytes.Buffer
	cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"ticket", "list"}, Stdout: &stdout})
	if strings.Contains(stdout.String(), "Corvint Tasks is unsupported on this platform") {
		t.Fatalf("supported platform refused: %s", stdout.String())
	}
}
