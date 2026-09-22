//go:build !darwin && !linux

package gitstatus

import "time"

func readRegular(string, int) ([]byte, bool, error) { return nil, false, errUnsafe }

type metadataReader struct{}

func (*metadataReader) Close()                      {}
func (*metadataReader) unchangedDirectories() error { return errUnsafe }
func (*metadataReader) readRegular(string, int) ([]byte, bool, time.Time, error) {
	return nil, false, time.Time{}, errUnsafe
}
