// SPDX-License-Identifier: Apache-2.0
package lineevidence

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// Write replaces exactly one instance after complete encoding. Failed collection
// never calls Write; an encoding or rename failure keeps the previous index.
func Write(store string, d testinventory.Declared, ix *linecov.Index) (string, error) {
	if !testinventoryRunner(d.Runner) || d.Key != testinventory.Key(d.Runner, d.Root) {
		return "", errors.New("line-evidence: invalid instance")
	}
	var data bytes.Buffer
	if err := ix.Encode(&data); err != nil {
		return "", err
	}
	if data.Len() > MaxBytes {
		return "", fmt.Errorf("line-evidence: index exceeds %d bytes", MaxBytes)
	}
	if _, err := linecov.Decode(bytes.NewReader(data.Bytes())); err != nil {
		return "", err
	}
	if err := os.MkdirAll(store, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(store)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("line-evidence: store is not a private directory")
	}
	f, err := os.CreateTemp(store, ".refresh-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data.Bytes()); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	name := filepath.Join(store, Filename(d))
	if err := os.Rename(f.Name(), name); err != nil {
		return "", err
	}
	return name, nil
}

func testinventoryRunner(runner string) bool {
	switch runner {
	case "go", "vitest", "jest", "rspec", "minitest":
		return true
	}
	return false
}
