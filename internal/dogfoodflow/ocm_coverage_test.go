package dogfoodflow

import (
	"context"
	"io"
	"testing"
)

// OCM-V0-900 (V1-3455): the ctxpkt-w1 shape. Tests named with the compact
// suffix _TCPV0991 and a traceability row naming TCP-V0-990..993 trace the
// new requirements; a compact form inside a longer token, a range of another
// prefix and a number outside the range do not.
func TestOCMV0900TracesCompactTestSuffixesAndRowRanges(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{
		"internal/x/x_test.go": "package x\n\nimport \"testing\"\n\nfunc TestRowsAreCut_TCPV0991(t *testing.T) {}\n\nfunc TestOther_TCPV09920(t *testing.T) {}\n\nfunc TestPrefixed_XTCPV0994(t *testing.T) {}\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "tests")
	f := &flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: io.Discard, target: testGit(t, root, "rev-parse", "HEAD")}
	referenced := map[string]bool{"TCP-V0-991": true, "TCP-V0-992": true, "TCP-V0-994": true}
	traced := map[string]bool{}
	if !f.testsNaming(referenced, traced) {
		t.Fatal("testsNaming could not read the tests")
	}
	if !traced["TCP-V0-991"] || traced["TCP-V0-992"] || traced["TCP-V0-994"] {
		t.Fatalf("traced = %v, want only TCP-V0-991 (TCPV09920 continues the token, XTCPV0994 has no _ before it)", traced)
	}

	row := "| TCP-V0-990..993 (experimental, V1-1269) | `specCitedRows` | `TestX_TCPV0990` |"
	for id, want := range map[string]bool{"TCP-V0-990": true, "TCP-V0-992": true, "TCP-V0-993": true, "TCP-V0-994": false, "TCP-V1-991": false, "ABC-V0-991": false} {
		if got := rowRangeNames(row, id); got != want {
			t.Errorf("rowRangeNames(%q) = %v, want %v", id, got, want)
		}
	}
	if !rowRangeNames("| ASD-V0-901..ASD-V0-907 | x |", "ASD-V0-905") {
		t.Error("a PREFIX-LO..PREFIX-HI range was not read")
	}
	if rowRangeNames("| XTCP-V0-990..993 | x |", "TCP-V0-991") {
		t.Error("a range inside a longer token traced TCP-V0-991")
	}
}
