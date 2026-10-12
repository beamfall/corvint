//go:build !unix

package receiptbundle

import (
	"errors"
	"os"
)

// openStableEntry fails closed where a nonblocking no-follow open is not
// available; the stable verifier itself runs on Darwin and Linux only.
func openStableEntry(*os.Root, string) (*os.File, error) {
	return nil, errors.New("stable bundle entries cannot be opened safely on this platform")
}
