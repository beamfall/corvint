//go:build !darwin && !linux

package opencodequalification

import (
	"context"
	"errors"
)

func runTerminal(context.Context, string) error {
	return errors.New("terminal qualification requires Darwin or Linux")
}
