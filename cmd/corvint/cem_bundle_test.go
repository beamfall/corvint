package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCSBV0001CEMBundleRoutesBeforeRootResolution proves `corvint cem bundle`
// reaches its own closed parser before Core root resolution: an unresolvable
// request ancestor is its typed refusal, not a root or unknown-action error.
func TestCSBV0001CEMBundleRoutesBeforeRootResolution(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"cem", "bundle", "--request", "/definitely/not/a/dir/request.json", "--out", "/definitely/not/a/dir/out"}, &forbiddenHelpReader{}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), `{"code": "bundle-path-unresolved", `) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"cem", "bundle"}, &forbiddenHelpReader{}, &stdout, &stderr); code != 2 || !strings.HasPrefix(stderr.String(), `{"code": "invalid-arguments", `) {
		t.Fatalf("bare cem bundle: exit=%d stderr=%q", code, &stderr)
	}
}
