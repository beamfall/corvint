//go:build !darwin

package main

import "errors"

func ensureRootDirectory(string) error { return errors.New("protected installation is Darwin-only") }
func emptyACL(string) error            { return errors.New("protected installation is Darwin-only") }
func openAuthorityDirectory(string, uint32) (int, error) {
	return -1, errors.New("protected installation is Darwin-only")
}
func renameExclusive(int, string, int, string) error {
	return errors.New("protected installation is Darwin-only")
}

func readRootFile(string, int) ([]byte, error) {
	return nil, errors.New("protected installation is Darwin-only")
}
func auditRootDirectory(string, bool) error {
	return errors.New("protected installation is Darwin-only")
}
