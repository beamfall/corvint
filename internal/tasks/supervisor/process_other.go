//go:build !darwin && !linux

package supervisor

import (
	"context"
	"fmt"
)

func Leader(context.Context, string, string) error {
	return fmt.Errorf("unsupported supervised platform")
}

func Run(context.Context, string, string, Capsule, Journal) (Outcome, error) {
	return Outcome{}, fmt.Errorf("unsupported supervised platform")
}

func Recover(Boot) bool { return false }
