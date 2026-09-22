//go:build !unix

package contextindex

import "os"

func mapReadOnly(*os.File, int64) []byte { return nil }

func unmapReadOnly([]byte) {}
