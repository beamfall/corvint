//go:build !darwin && !linux

package gitstatus

import "time"

var errPlatform = unsupported("cannot be read without no-follow support on this platform")

func readRegular(string, int) ([]byte, bool, error) { return nil, false, errPlatform }

type metadataReader struct{}

func (*metadataReader) Close()                      {}
func (*metadataReader) unchangedDirectories() error { return errPlatform }
func (*metadataReader) readRegular(string, int) ([]byte, bool, time.Time, error) {
	return nil, false, time.Time{}, errPlatform
}
