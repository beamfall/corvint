//go:build !unix

package playwrightruns

import "os"

func openNonblocking(name string) (*os.File, error) {
	return os.Open(name)
}
