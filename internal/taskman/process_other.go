//go:build !darwin && !linux

package taskman

import (
	"context"
	"errors"
	"os"
)

func runRead(context.Context, string, string, []string) ([]byte, error) {
	return nil, errors.New("native fixture planner unsupported platform")
}

func openInput(string) (*os.File, error) {
	return nil, errors.New("native fixture planner unsupported platform")
}
