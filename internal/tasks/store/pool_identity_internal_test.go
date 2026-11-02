package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// TestPSRV0084_IdentityOutputRefusesTrailingDelimiter pins the strict
// single-value decode of an identity command's stdout: More() reports false
// before a stray '}' or ']', so only an io.EOF end check refuses it (V1-3358).
func TestPSRV0084_IdentityOutputRefusesTrailingDelimiter(t *testing.T) {
	const oid = "0123456789abcdef0123456789abcdef01234567"
	value := `{"revision":"` + oid + `","tree":"` + oid + `"}`
	if class, src := identityClass([]byte(value + "\n \t")); class != snapshot.PoolIdentityReported || src == nil {
		t.Fatalf("well-formed output with trailing white space = %q, %v; want %q", class, src, snapshot.PoolIdentityReported)
	}
	for _, tail := range []string{"}", "]", "\n}", " ]", "{}", "1", "x"} {
		if class, src := identityClass([]byte(value + tail)); class != "MALFORMED_OUTPUT" || src != nil {
			t.Errorf("output with trailing %q = %q, %v; want MALFORMED_OUTPUT", tail, class, src)
		}
	}
}
