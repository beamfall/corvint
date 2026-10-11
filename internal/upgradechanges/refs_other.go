//go:build !darwin && !linux

package upgradechanges

import "errors"

func readRefs(string) ([]byte, bool, error) {
	return nil, false, errors.New("changes-refs-unavailable")
}
