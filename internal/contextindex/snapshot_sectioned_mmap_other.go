//go:build !unix

package contextindex

import "os"

func mapReadOnly(*os.File, int64) []byte { return nil }

func unmapReadOnly([]byte) {}

func duplicateSnapshotFile(*os.File) (*os.File, error) { return nil, os.ErrInvalid }
