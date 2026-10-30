//go:build !darwin && !linux

package main

import "errors"

func replaceNativeHook(_ string, _, _ []string) error {
	return errors.New("protected native bootstrap unsupported")
}
func readNativeHookFile(_ string, _ int64) ([]byte, error) {
	return nil, errors.New("protected native bootstrap unsupported")
}
