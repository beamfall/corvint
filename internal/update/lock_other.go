//go:build !darwin && !linux

package update

import "errors"

func lockDirectory(string) (func(), error) {
	return nil, errors.New("advisory locking unsupported on host")
}
