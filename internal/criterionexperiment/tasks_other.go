//go:build !darwin && !linux

package criterionexperiment

import (
	"context"
	"fmt"
)

func runTasks(context.Context, string, string, []string, []byte) ([]byte, error) {
	return nil, fmt.Errorf("Tasks subprocess verification unsupported on this platform")
}
