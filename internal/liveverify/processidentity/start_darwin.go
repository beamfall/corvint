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
	command.WaitDelay = 250 * time.Millisecond
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
