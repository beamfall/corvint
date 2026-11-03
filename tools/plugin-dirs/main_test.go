// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestPluginDirs_LGPV0021 prints a known plugin's directories and refuses an
// unknown name or a wrong argument count.
func TestPluginDirs_LGPV0021(t *testing.T) {
	var out, errOut bytes.Buffer
	if rc := run([]string{"ruby"}, &out, &errOut); rc != 0 {
		t.Fatalf("ruby: rc %d, stderr %s", rc, errOut.String())
	}
	if !strings.Contains(out.String(), "internal/liveverify/rbmutate\n") {
		t.Errorf("ruby dirs lack its mutation engine:\n%s", out.String())
	}
	for _, args := range [][]string{{"no-such-plugin"}, nil, {"ruby", "go"}} {
		out.Reset()
		errOut.Reset()
		if rc := run(args, &out, &errOut); rc != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "ruby") {
			t.Errorf("%q: rc %d, stdout %q, stderr %q; want a refusal naming the plugins", args, rc, out.String(), errOut.String())
		}
	}
}
