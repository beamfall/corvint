// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// CMS-V0-913: `corvint flows ...` and `corvint how ...` are exec-only shims onto corvint-docs
// with the argument vector, stdin, stdout, stderr and exit status passed through unchanged.
func TestCMSV0913FlowsShimForwardsToCorvintDocs(t *testing.T) {
	capture := fakeDocsApp(t)
	for _, args := range [][]string{
		{"flows", "map", "--flows", "flows"},
		{"flows", "--manifest", "m.json"},
		{"flows"},
		{"--root", "/some/where", "flows", "export", "--flows", "flows", "--emit", "inventory"},
		{"--root=/some/where", "flows", "appmap", "build", "--manifest", "appmap.json"},
		{"how", "--map", "map.json", "--source", "e2e", "ship an order"},
		{"--root", "r", "how", "--map", "map.json", "--source", "e2e", "--", "--require-clean"},
	} {
		_ = os.Remove(capture)
		var stdout, stderr bytes.Buffer
		code := run(args, strings.NewReader("stdin bytes"), &stdout, &stderr)
		recorded, err := os.ReadFile(capture)
		if err != nil {
			t.Fatalf("%q was not forwarded: exit %d stderr %q", args, code, &stderr)
		}
		if got, want := string(recorded), strings.Join(args, "\n")+"\n"; got != want {
			t.Errorf("%q forwarded as %q", args, got)
		}
		if code != 3 || stdout.String() != "forwarded:stdin bytes" || stderr.String() != "app stderr\n" {
			t.Errorf("%q: exit %d stdout %q stderr %q; want the app's own", args, code, &stdout, &stderr)
		}
	}
}

// CMS-V0-913: help for the flows group and for how is corvint's own page, names corvint-docs,
// and never starts the app; the adoption chain's build-step help stays native (CCE-V0-118).
func TestCMSV0913FlowsShimHelpIsNativeAndNamesCorvintDocs(t *testing.T) {
	capture := fakeDocsApp(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"flows", "--help"}, "served by corvint-docs"},
		{[]string{"help", "flows"}, "corvint-docs help flows"},
		{[]string{"how", "--help"}, "served by corvint-docs"},
		{[]string{"help", "how"}, "corvint-docs help how"},
		{[]string{"flows", "appmap", "build", "--help"}, "--from MAP"},
		{[]string{"flows", "test-evidence", "--help"}, "flows test-evidence --flows DIR"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(test.args, &forbiddenHelpReader{}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), test.want) {
			t.Errorf("%q: exit %d stderr %q stdout lacks %q: %s", test.args, code, &stderr, test.want, &stdout)
		}
		if _, err := os.Stat(capture); err == nil {
			t.Errorf("%q reached corvint-docs", test.args)
		}
	}
	if help := helpText("root"); !strings.Contains(help, "\n    flows ") || !strings.Contains(help, "\n    how ") {
		t.Errorf("root help does not list flows and how")
	}
}

// CMS-V0-913: a missing corvint-docs is the named NOT_INSTALLED gap, exit 2, and names the form
// the user ran.
func TestCMSV0913FlowsShimReportsNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, test := range []struct {
		args  []string
		names string
	}{
		{[]string{"flows", "map", "--flows", "flows"}, "corvint flows is served"},
		{[]string{"--root", "r", "flows"}, "corvint flows is served"},
		{[]string{"how", "--map", "map.json", "--source", "e2e", "ship"}, "corvint how is served"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(test.args, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code": "not-installed"`) || !strings.Contains(stderr.String(), "NOT_INSTALLED") || !strings.Contains(stderr.String(), "corvint-docs") || !strings.Contains(stderr.String(), test.names) {
			t.Errorf("%q: exit %d stdout %q stderr %q", test.args, code, &stdout, &stderr)
		}
	}
}

// DIRTY-CACHE-016, CMS-V0-913: the flows group and how refuse --require-clean in the shim,
// before it looks for corvint-docs, and name the flag.
func TestCMSV0913FlowsShimRefusesRequireClean(t *testing.T) {
	capture := fakeDocsApp(t)
	for _, args := range [][]string{
		{"flows", "map", "--flows", "flows", "--require-clean"},
		{"--root", "r", "flows", "--require-clean=true"},
		{"how", "--require-clean", "--map", "map.json", "--source", "e2e", "ship"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(args, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--require-clean") || !strings.Contains(stderr.String(), `"code": "invalid-arguments"`) {
			t.Errorf("%q: exit %d stdout %q stderr %q", args, code, &stdout, &stderr)
		}
		if _, err := os.Stat(capture); err == nil {
			t.Errorf("%q reached corvint-docs", args)
		}
	}
}

// CMS-V0-914, RSP-V0-051: `flows test-evidence` runs in corvint, with no corvint-docs on PATH,
// and joins the linked flows onto the retained test evidence.
func TestRSPV0051FlowProjection(t *testing.T) {
	fx := newShopFixture(t)
	path := os.Getenv("PATH")
	capture := fakeDocsApp(t)
	// The stand-in stays first on PATH, so a forward would be caught; git stays reachable.
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+path)
	runNative := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"--root", fx.root, "flows"}, args...), strings.NewReader(""), &stdout, &stderr)
		if _, err := os.Stat(capture); err == nil {
			t.Fatalf("%q reached corvint-docs", args)
		}
		return code, stdout.String(), stderr.String()
	}
	code, out, diag := runNative("test-evidence", "--flows", "flows", "checkout/checkout.go")
	if code != 0 || diag != "" {
		t.Fatalf("%d %s", code, diag)
	}
	var d struct {
		Schema   string
		Flows    json.RawMessage
		Evidence struct {
			Status  string
			Reasons []string
		}
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatal(err)
	}
	if d.Schema != "application-flow-test-evidence/1" || d.Evidence.Status != "UNKNOWN" || !strings.Contains(string(d.Flows), "checkout") {
		t.Fatal(out)
	}
	for _, args := range [][]string{{"test-evidence", "checkout/checkout.go"}, {"test-evidence", "--flows", "flows", "checkout/checkout.go", "--revision", fx.b}} {
		if code, _, diag = runNative(args...); code != 2 || !strings.Contains(diag, `"code"`) {
			t.Fatalf("accepted %v: %d %s", args, code, diag)
		}
	}
}
