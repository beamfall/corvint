// SPDX-License-Identifier: Apache-2.0

//go:build !linux || !amd64

package corpusbridge

import "os"

func (s *stamper) describeAt(_ *os.File, entries []os.DirEntry) stampChunk {
	return s.describe(entries)
}
