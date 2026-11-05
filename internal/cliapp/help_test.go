// SPDX-License-Identifier: Apache-2.0
package cliapp

import (
	"bytes"
	"testing"
)

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
	code, handled := Answer("corvint-pulse", []string{"--root", ".", "serve", "--help"}, &out, &errout)
	if code != 0 || handled || out.Len() != 0 || errout.Len() != 0 {
		t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, out.String(), errout.String())
	}
}

// CAP-V0-9300: emitting the exact-node synopsis also completes dispatch.
func TestNativeNodeHelpFullyHandled(t *testing.T) {
	for _, args := range [][]string{{"serve", "--help"}, {"serve", "-h"}} {
		var out, errout bytes.Buffer
		code, handled := Answer("corvint-pulse", args, &out, &errout)
		if code != 0 || !handled || out.Len() == 0 || errout.Len() != 0 {
			t.Fatalf("%v: code=%d handled=%v stdout=%q stderr=%q", args, code, handled, out.String(), errout.String())
		}
	}
}
