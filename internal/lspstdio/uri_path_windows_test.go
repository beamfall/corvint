// SPDX-License-Identifier: Apache-2.0
package lspstdio

import "testing"

// V1-2488 item 6: on Windows a drive-letter file URI below a `C:\` root names a repository subject
// instead of failing as an invalid LSP frame.
func TestContextParamsAdmitWindowsDriveURI(t *testing.T) {
	raw := `{"textDocument":{"uri":"file:///C:/repo/pkg/a.go"},"task":"investigate","limit":5}`
	input, uri, err := parseContextParams([]byte(raw), `C:\repo`)
	if err != nil || input.Subject != "pkg/a.go" || uri != "file:///C:/repo/pkg/a.go" {
		t.Fatalf("drive URI refused: %+v %q %v", input, uri, err)
	}
}
