//go:build !darwin && !linux

package criterionexperiment

import (
	"context"
	"fmt"
)

func runScenario(context.Context, Request, Criterion, map[string][]byte, map[string]string, string) ([]byte, []byte, int, bool, error) {
	return nil, nil, 255, false, fmt.Errorf("runner unsupported on this platform")
}
