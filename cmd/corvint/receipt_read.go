// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
	"os"
	"path/filepath"
)

func readProviderReceipt(path string) (testvaliditydoc.Input, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot resolve --receipt: " + err.Error())
	}
	parent, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot open --receipt parent: " + err.Error())
	}
	defer parent.Close()
	data, err := readReceiptFile(parent, filepath.Base(resolved))
	if errors.Is(err, testvaliditydoc.ErrPlatformUnsupported) {
		return testvaliditydoc.Input{}, receiptPlatformRefusal()
	}
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("cannot read --receipt: " + err.Error())
	}
	receipt, err := testvaliditydoc.Decode(data)
	if err != nil {
		return testvaliditydoc.Input{}, invalidReceipt("--receipt " + err.Error())
	}
	return receipt, nil
}

// readReceiptFile is testvaliditydoc.ReadFile; tests force its platform
// refusal (PRS-V1-920).
var readReceiptFile = testvaliditydoc.ReadFile

// receiptPlatformRefusal is the typed receipt platform refusal (PRS-V1-920).
func receiptPlatformRefusal() *gokernel.Error {
	return &gokernel.Error{
		Code:    testvaliditydoc.PlatformUnsupportedCode,
		Message: testvaliditydoc.ErrPlatformUnsupported.Error(),
	}
}

func invalidReceipt(message string) error {
	return &gokernel.Error{Code: "invalid-test-validity-receipt", Message: message}
}
