//go:build !unix

package main

import "errors"

// openRegularNoFollow has no symlink-refusing open on this platform.
func openRegularNoFollow(string) (int, error) {
	return -1, errors.New("open-no-follow-unsupported")
}
