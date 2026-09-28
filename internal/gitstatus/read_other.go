//go:build !darwin && !linux

package gitstatus

import (
	"context"
	"time"
)

var errPlatform = unsupported(classMetadataUnreadable, "cannot be read without no-follow support on this platform")

func readRegular(string, int) ([]byte, bool, error) { return nil, false, errPlatform }

type metadataReader struct{}

func (*metadataReader) Close()                      {}
func (*metadataReader) unchangedDirectories() error { return errPlatform }
func (*metadataReader) readRegular(string, int) ([]byte, bool, time.Time, error) {
	return nil, false, time.Time{}, errPlatform
}

func worktreeInputsOpen(context.Context, string, []byte) error { return errPlatform }

func snapshotGitlink(string, string, string, *metadataReader, *[]capturedFile) error {
	return errPlatform
}
