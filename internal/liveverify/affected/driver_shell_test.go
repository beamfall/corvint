// SPDX-License-Identifier: Apache-2.0

package affected

import "testing"

// AZU-V0-001: truncated wrapper operands must retain uncertainty instead
// of panicking or silently claiming the command is fully understood.
func TestDriverTruncatedWrapperOptions_AZU_V0_001(t *testing.T) {
	for _, command := range []string{
		"env -u", "env -C", "env -S", "env NAME=value -u",
		"nice -n", "timeout -k", "xargs -I", "ionice -c", "taskset -c",
		"env -u NAME nice -n",
	} {
		t.Run(command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(command, ".", "example.test/m")
			if !scan.computed {
				t.Fatal("malformed wrapper lost uncertainty")
			}
		})
	}
	for _, command := range []string{"env -u NAME go test ./core/...", "nice -n 10 go test ./core/..."} {
		t.Run(command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(command, ".", "example.test/m")
			if scan.computed || !scan.paths["core/..."] {
				t.Fatalf("valid wrapper failed: %+v", scan)
			}
		})
	}
}
