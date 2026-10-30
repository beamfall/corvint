//go:build !unix

package docmaintain

import "os"

const watchPlatformSupported = false

func openWatchPage(path string) (*os.File, error) {
	return nil, refuse("unsupported-watch-platform")
}
