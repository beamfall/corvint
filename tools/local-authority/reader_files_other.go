//go:build !darwin

package main

import "errors"

func closeReaderEvidence(int) {}
func createReaderEvidence(*readerLedger) (int, error) {
	return -1, errors.New("reader operator is Darwin-only")
}
func ownReaderEvidence(int, readerAudit) error { return errors.New("reader operator is Darwin-only") }
func exposeReaderEvidence(int, readerLedger) error {
	return errors.New("reader operator is Darwin-only")
}
func restrictReaderEvidence(readerLedger) error { return errors.New("reader operator is Darwin-only") }
func writeReaderLedger(readerLedger) error      { return errors.New("reader operator is Darwin-only") }
func retireReaderLedger() error                 { return errors.New("reader operator is Darwin-only") }
func requireReaderWithdrawn() error             { return errors.New("reader operator is Darwin-only") }
func archiveRetiredReaderEvidence() error       { return errors.New("reader operator is Darwin-only") }
