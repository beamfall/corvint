// SPDX-License-Identifier: Apache-2.0
package cliapp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/versionline"
)

// CAP-V0-8800: a literal help node has its own page, not its root's inputs.
func TestCAPV08800HelpNode(t *testing.T) {
	for _, binary := range []string{"corvint-cli", "corvint-mcp"} {
		for _, flag := range []string{"--help", "-h"} {
			var out, errout bytes.Buffer
			code, handled := Answer(binary, []string{"help", flag}, &out, &errout)
			if code != 0 || !handled || errout.Len() != 0 || !strings.Contains(out.String(), binary+" help [COMMAND...] [--json]") || !strings.Contains(out.String(), "Required: none") {
				t.Fatalf("%s %s: code=%d handled=%t stdout=%q stderr=%q", binary, flag, code, handled, out.String(), errout.String())
			}
			out.Reset()
			code, handled = Answer(binary, []string{"help", "zz-unknown", flag}, &out, &errout)
			if code != 0 || handled || out.Len() != 0 || errout.Len() != 0 {
				t.Fatalf("unknown help was consumed: code=%d handled=%t stdout=%q stderr=%q", code, handled, out.String(), errout.String())
			}
		}
	}
}

// ACP-011 / ACX-V0-7707: unhandled argv must leave both streams untouched
// so the native analyzer can emit its protocol frame.
func TestAnalyzerNativeDispatch(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"bogus", "--help"}, {"help", "bogus"}, {"--version"}, {"--version", "extra"}} {
		var out, errout bytes.Buffer
		code, handled := Answer("corvint-analyzer-go", args, &out, &errout)
		if handled || code != 0 || out.Len() != 0 || errout.Len() != 0 {
			t.Fatalf("%v: code=%d handled=%v stdout=%q stderr=%q", args, code, handled, out.String(), errout.String())
		}
	}
}

// ACX-V0-9301: a native preamble refusal must have untouched streams.
func TestNativeHelpPreambleDoesNotWrite(t *testing.T) {
	var out, errout bytes.Buffer
	code, handled := Answer("corvint-go-test-provider", []string{"--dir", ".", "test-run", "--help"}, &out, &errout)
	if code != 0 || handled || out.Len() != 0 || errout.Len() != 0 {
		t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, out.String(), errout.String())
	}
}

// CAP-V0-9300: emitting the exact-node synopsis also completes dispatch.
func TestNativeNodeHelpFullyHandled(t *testing.T) {
	for _, args := range [][]string{{"test-run", "--help"}, {"test-run", "-h"}} {
		var out, errout bytes.Buffer
		code, handled := Answer("corvint-go-test-provider", args, &out, &errout)
		if code != 0 || !handled || out.Len() == 0 || errout.Len() != 0 {
			t.Fatalf("%v: code=%d handled=%v stdout=%q stderr=%q", args, code, handled, out.String(), errout.String())
		}
	}
}

// CAP-V0-9300: provider help belongs to its authored handler, which appends
// flag-set detail to the synopsis (V1-1221); delegation must leave streams untouched.
func TestNativeProviderHelpDoesNotWrite(t *testing.T) {
	for _, sub := range []string{"negate", "external-origins"} {
		for _, help := range []string{"-h", "--help", "-help"} {
			var out, errout bytes.Buffer
			args := []string{sub, help}
			code, handled := Answer("corvint-js-test-provider", args, &out, &errout)
			if code != 0 || handled || out.Len() != 0 || errout.Len() != 0 {
				t.Fatalf("%v: code=%d handled=%v stdout=%q stderr=%q", args, code, handled, out.String(), errout.String())
			}
		}
	}
}

// CAP-V0-8800 / ACP-011: even an exact analyzer help node must leave
// protocol dispatch untouched; offline root discovery remains supported.
func TestCAPV08800AnalyzerNodeHelpDelegates(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		for _, node := range []string{"help", "zz-unknown"} {
			var out, errout bytes.Buffer
			code, handled := Answer("corvint-analyzer-go", []string{node, flag}, &out, &errout)
			if code != 0 || handled || out.Len() != 0 || errout.Len() != 0 {
				t.Fatalf("%s %s: code=%d handled=%t stdout=%q stderr=%q", node, flag, code, handled, out.String(), errout.String())
			}
		}
		var out, errout bytes.Buffer
		code, handled := Answer("corvint-analyzer-go", []string{flag}, &out, &errout)
		if code != 0 || !handled || !strings.Contains(out.String(), "corvint-analyzer-go < REQUEST.json") || errout.Len() != 0 {
			t.Fatalf("root %s: code=%d handled=%t stdout=%q stderr=%q", flag, code, handled, out.String(), errout.String())
		}
	}
}

// ARTIFACT-BIN-V0-002: an independently released binary reports the version the release
// builder stamped, with its build and commit; unstamped it reports the development line as
// before, and an empty version is the compiled development line.
func TestAnswerReleasedReportsTheStampedVersion(t *testing.T) {
	commit := strings.Repeat("a", 40)
	cases := map[string]struct {
		version, build, commit string
		want                   string
	}{
		"stamped":   {"1.4.2", "183", commit, "corvint-update 1.4.2 (build 183, commit " + commit + ")"},
		"unstamped": {"", "0", "", "corvint-update " + versionline.Development},
		"compiled":  {"", "183", commit, "corvint-update " + versionline.Release + " (build 183, commit " + commit + ")"},
	}
	for name, c := range cases {
		var out, errout bytes.Buffer
		code, handled := AnswerReleased("corvint-update", c.version, c.build, c.commit, []string{"--version"}, &out, &errout)
		if code != 0 || !handled || errout.Len() != 0 || strings.TrimSpace(out.String()) != c.want {
			t.Errorf("%s: code=%d handled=%t stdout=%q stderr=%q, want %q", name, code, handled, out.String(), errout.String(), c.want)
		}
	}
}
