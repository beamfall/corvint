//go:build !darwin && !linux

package testrunner

import (
	"context"
	"fmt"
)

func Execute(_ context.Context, _ Request, _ Invocation) (Execution, error) {
	return Execution{}, fmt.Errorf("runner process containment unsupported on this platform")
}
