// SPDX-License-Identifier: Apache-2.0
package taskbarrier

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestPURV02210_CompanionWireFailuresStayUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Tasks companion does not support Windows")
	}
	root, bin := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".taskman"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskman", "queue.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := Observe(root); !strings.Contains(got, "companion unavailable") {
		t.Fatalf("missing: %s", got)
	}
	run := func(raw string) string {
		t.Helper()
		script := "#!/bin/sh\n[ \"$*\" = 'queue status --fields barrier' ] || exit 9\nprintf '%s' '" + raw + "'\n"
		if err := os.WriteFile(filepath.Join(bin, "corvint-tasks"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		return Observe(root)
	}
	if got := run("{}"); !strings.Contains(got, "NOT_OBSERVED") {
		t.Fatalf("malformed: %s", got)
	}
	for _, tc := range []struct {
		barrier wire.Value
		want    string
	}{
		{wire.Null(), ""},
		{wire.ObjectValue(wire.NewObject().Set("scope", wire.String("ADMISSION"))), "companion predates age warnings"},
		{wire.ObjectValue(wire.NewObject().Set("warningPolicySeconds", wire.String("3600"))), ""},
		{wire.ObjectValue(wire.NewObject().Set("warningPolicySeconds", wire.Bool(true))), "invalid age policy observation"},
	} {
		r := &wire.Result{Command: []string{"queue", "status"}, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(wire.NewObject().Set("barrier", tc.barrier))}}
		raw, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got := run(string(raw))
		if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
			t.Fatalf("wire: %s want %q", got, tc.want)
		}
	}
}

func TestPURV02210_OutputCapCannotBeBypassedByCopy(t *testing.T) {
	var b boundedOutput
	// LimitReader avoids strings.Reader.WriteTo so io.Copy also probes ReaderFrom.
	_, err := io.Copy(&b, io.LimitReader(strings.NewReader(strings.Repeat("x", 70000)), 70000))
	if err == nil || !b.overflow || b.buf.Len() > 65536 {
		t.Fatalf("cap bypassed: %d %v %v", b.buf.Len(), b.overflow, err)
	}
}
