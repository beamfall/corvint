//go:build darwin

package processidentity

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const Source = "PS_LSTART"

func Start(ctx context.Context, pid int) (string, error) {
	command := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	command.Env = []string{"LC_ALL=C", "TZ=UTC"}
	// A load-safe output-copy bound after ps exits, not 250 ms: a starved copy
	// would report a successful ps as an observation failure (V1-3435, CEM-V1-820).
	command.WaitDelay = 5 * time.Second
	raw, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrObservation, err)
	}
	start := strings.TrimSpace(string(raw))
	if start == "" {
		return "", fmt.Errorf("%w: empty identity", ErrObservation)
	}
	return start, nil
}
