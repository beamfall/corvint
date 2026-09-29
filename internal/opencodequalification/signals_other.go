//go:build !darwin && !linux

package opencodequalification

import "errors"

func terminateProcess(int) error { return errors.New("unsupported process cleanup platform") }
