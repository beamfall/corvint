//go:build !darwin && !linux

package upgradechanges

import (
	"errors"
	"os"
)

func openRefs(string) (*os.File, error) {
	return nil, errors.New("changes-refs-unavailable")
}
