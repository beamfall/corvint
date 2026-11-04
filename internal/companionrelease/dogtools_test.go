// SPDX-License-Identifier: Apache-2.0
package companionrelease

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDTLV0011StderrOnce(t *testing.T) {
	t.Run("DTL-V0-011", func(t *testing.T) {
		if os.Getenv("CORVINT_DOGTOOLS_CHILD") == "1" {
			os.Stderr.WriteString("unique-stderr-marker")
			os.Exit(7)
		}
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		got := runWithStdin(context.Background(), t.TempDir(), append(os.Environ(), "CORVINT_DOGTOOLS_CHILD=1"), time.Minute, nil, exe, "-test.run=^TestDTLV0011StderrOnce$")
		if got.err == nil || strings.Count(got.err.Error(), "unique-stderr-marker") != 1 || strings.Count(got.err.Error(), "stderr=") != 1 {
			t.Fatalf("%v", got.err)
		}
	})
}
