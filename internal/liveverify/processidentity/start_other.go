//go:build !darwin && !linux

package processidentity

import (
	"context"
	"fmt"
)

const Source = "UNAVAILABLE"

func Start(context.Context, int) (string, error) {
	return "", fmt.Errorf("%w on this platform", ErrObservation)
}
