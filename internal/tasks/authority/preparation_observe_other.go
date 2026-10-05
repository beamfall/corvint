//go:build !darwin && !linux

package authority

import "errors"

func loadPreparationLockView() (preparationLockView, error) {
	return nil, errors.New("no lock query on this platform")
}
