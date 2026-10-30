package main

import (
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// TestCAXV0064BinaryInstallsDataReaderFinder checks CAX-V0-064: the binary
// installs the data-reader search the cem-path-policy/1 profile uses, so a
// data row from `corvint cem status --path-policy base` names its readers
// rather than data-reader-not-produced.
func TestCAXV0064BinaryInstallsDataReaderFinder(t *testing.T) {
	if cemcli.DataReaders == nil {
		t.Fatal("cemcli.DataReaders is not installed")
	}
	if _, err := cemcli.DataReaders(t.Context(), t.TempDir(), "0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("a finder over a directory that is no repository was opened")
	}
}
