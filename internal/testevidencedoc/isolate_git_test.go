// SPDX-License-Identifier: Apache-2.0
package testevidencedoc

import (
	"github.com/Beamfall/corvint/internal/testsupport"
	"os"
	"testing"
)

func TestMain(m *testing.M) { testsupport.IsolateGit(); os.Exit(m.Run()) }
