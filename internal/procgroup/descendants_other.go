//go:build !darwin && !linux

package procgroup

import (
	"context"
	"errors"
)

func descendantSnapshot(context.Context) (map[int]ObservedProcess, error) {
	return nil, errors.New("descendant observation unsupported")
}
func signalObservedProcess(map[int]ObservedProcess, ObservedProcess) error {
	return errors.New("descendant observation unsupported")
}
