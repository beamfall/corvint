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

// CAP-V0-9300: help parses native option layouts without treating values as verbs.
func TestCLIProfile9300HelpOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--root", ".", "affected", "--help"},
		{"--root=.", "affected", "-h"},
		{"--json", "affected", "--help"},
		{"affected", "--base", "context", "--help"},
		{"affected", "--format", "text", "--help"},
		{"affected", "--full", "--require-clean", "--help"},
		{"affected", "--full=true", "--help"},
	} {
		var out, errout bytes.Buffer
		code := Profile(context.Background(), t.TempDir(), append([]string{"--binary", "corvint"}, args...), nil, &out, &errout)
		if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "corvint affected") {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out.String(), errout.String())
		}
	}
	for _, args := range [][]string{
		{"affected", "--", "--help"},
		{"--", "affected", "--help"},
		{"--root", "--help"},
	} {
		var out, errout bytes.Buffer
		code := Profile(context.Background(), t.TempDir(), append([]string{"--binary", "corvint"}, args...), nil, &out, &errout)
		if code != 2 || out.Len() != 0 || !strings.Contains(errout.String(), "binary-unavailable") {
			t.Errorf("%v: help after -- or used as a value must reach native dispatch: %d %q %q", args, code, out.String(), errout.String())
		}
	}
}

// CAP-V0-9300: authored literal values have the same arity as placeholders.
func TestCLIProfile9300LiteralHelpValues(t *testing.T) {
	for _, row := range []struct {
		command string
		args    []string
	}{
		{"cem prepare", []string{"cem", "prepare", "--non-regular", "unknown", "--help"}},
		{"cem prepare", []string{"cem", "prepare", "--spec", "cem/1.0", "--path-policy", "base", "--help"}},
		{"context", []string{"context", "--instruction-profile", "default", "--help"}},
		{"context", []string{"--instruction-profile", "default", "context", "--help"}},
		{"cem prepare", []string{"cem", "prepare", "--replace", "--help"}},
		{"eval", []string{"eval", "--reset-slot-weights", "--help"}},
		{"eval", []string{"eval", "--learn-slot-weights", "--help"}},
	} {
		t.Run(strings.Join(row.args, " "), func(t *testing.T) {
			var out, errout bytes.Buffer
			code := Profile(context.Background(), t.TempDir(), append([]string{"--binary", "corvint"}, row.args...), nil, &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "corvint "+row.command) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errout.String())
			}
		})
	}
}
