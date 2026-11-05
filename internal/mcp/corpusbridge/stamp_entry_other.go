// SPDX-License-Identifier: Apache-2.0
//go:build !linux || !amd64

package corpusbridge

import (
	"io/fs"
	"os"
)

func stampEntryReader(string) (func(os.DirEntry) (fs.FileInfo, error), func(), error) {
	return func(e os.DirEntry) (fs.FileInfo, error) { return e.Info() }, func() {}, nil
}
