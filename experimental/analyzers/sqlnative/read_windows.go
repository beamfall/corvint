//go:build windows

package sqlnative

import (
	"context"
	"errors"
	"os"
)

func ReadNoFollow(ctx context.Context, root *os.File, path string, limit int) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return nil, errors.New("unsupported")
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	return ctx.Err()
}
