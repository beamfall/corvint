// SPDX-License-Identifier: Apache-2.0

package t6

import (
	"io/ioutil"
	"os"
	"testing"

	"example.com/fix/lib"
)

func TestCreate(t *testing.T) {
	_ = lib.A()
	_, _ = ioutil.TempFile(os.TempDir(), "typedreach-")
}

func TestReadParent(t *testing.T) { _, _ = ioutil.ReadDir(os.TempDir()) }

func TestControl(t *testing.T) { _ = lib.B() }
