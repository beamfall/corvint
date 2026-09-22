package main

import "testing"

func TestRunRequiresClosedNamedArguments(t *testing.T) {
	if got := run(nil); got != 2 {
		t.Fatalf("missing arguments exit %d, want 2", got)
	}
	if got := run([]string{"-unknown"}); got != 2 {
		t.Fatalf("unknown argument exit %d, want 2", got)
	}
}

func TestCoreOptionsRemainExplicitAndClosed(t *testing.T) {
	base := []string{"--bundle-dir", "/bundle", "--source-root", "/source", "--scratch", "/scratch", "--output", "/out", "--npm-cache", "/npm", "--browser-cache", "/browser", "--node", "/node", "--python", "/python", "--python-sha256", "digest", "--expected-commit", "commit", "--expected-tree", "tree"}
	for _, extra := range [][]string{{"--qualification", "unknown"}, {"--qualification", "core"}, {"--node-sha256", "digest"}, {"--go-authority-bundle", "/attachment"}} {
		if got := run(append(append([]string{}, base...), extra...)); got != 2 {
			t.Fatalf("accepted malformed profile options %v: %d", extra, got)
		}
	}
}
