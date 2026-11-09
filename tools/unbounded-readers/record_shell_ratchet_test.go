// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-3424, AFP-V0-3425: this test uses only the pre-change API, so the
// identical test can prove the old ratchet admitted a new undeclared shell
// reader. Its fixture must initialize before the negative assertion runs.
func TestRecordEntryShellReaderRatchet_AFPV3424(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":      "module example.test/m\n\ngo 1.27\n",
		recordPath:    `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`,
		"new_test.sh": "#!/bin/sh\ncat docs/build-log/*.md\n", //corvint:fixture-path
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "new_test.sh") || !strings.Contains(stderr.String(), "declare no read scope") {
		t.Fatalf("undeclared shell record reader admitted: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
}
