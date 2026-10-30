//go:build !unix

package main

import "errors"

func execUnconfined(string, []string) error {
	return errors.New("test-confine runs only on unix")
}
