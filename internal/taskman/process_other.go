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

func runPortable(context.Context, string, string, []string) ([]byte, int, error) {
	return nil, 0, errors.New("portable consumer unsupported platform")
}

func openInput(string) (*os.File, error) {
	return nil, errors.New("native fixture planner unsupported platform")
}
