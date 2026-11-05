// SPDX-License-Identifier: Apache-2.0
package cliprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clierror"
)

func TestCLIProfile9301RefusalShapes(t *testing.T) {
	for _, row := range []struct{ out, stderr, code, message string }{
		{"", `{"code":"missing","error":"File is missing"}`, "missing", "File is missing"},
		{"", `{"error":{"code":"missing","message":"File is missing"}}`, "missing", "File is missing"},
		{`{"codes":[],"warnings":["Ticket does not exist"]}`, "", "not-found", "Ticket does not exist"},
		{`{"codes":["MALFORMED"],"warnings":["Invalid input"]}`, "", "MALFORMED", "Invalid input"},
	} {
		code, message := refusal([]byte(row.out), []byte(row.stderr))
		if code != row.code || message != row.message {
			t.Fatalf("%+v: %q %q", row, code, message)
		}
	}
	for _, args := range [][]string{nil, {"--binary", "../corvint"}, {"--binary", "corvint-cli"}, {"--binary", "corvint", "--version", "extra"}, {"--binary", "corvint-docs-mcp"}, {"--binary", "corvint", "help", "no-such-command"}, {"--binary", "corvint", "context", "--task", "test"}} {
		var out, errout bytes.Buffer
		code := Profile(context.Background(), t.TempDir(), args, nil, &out, &errout)
		var e clierror.Envelope
		if code != 2 || out.Len() != 0 || json.Unmarshal(errout.Bytes(), &e) != nil || clierror.Validate(e) != nil {
			t.Fatalf("%v: %d %q %q", args, code, out.String(), errout.String())
		}
	}
}

func TestCLIProfile9301BoundedCapture(t *testing.T) {
	var capture boundedOutput
	n, err := io.Copy(&capture, strings.NewReader(strings.Repeat("x", (1<<20)+1)))
	if err != nil || n != (1<<20)+1 || len(capture.Bytes()) != 1<<20 || !capture.truncated {
		t.Fatalf("n=%d err=%v bytes=%d truncated=%v", n, err, len(capture.Bytes()), capture.truncated)
	}
}
