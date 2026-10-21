//go:build !unix

package sessiontest

import "errors"

func mkfifo(string) error { return errors.New("no FIFOs on this platform") }
