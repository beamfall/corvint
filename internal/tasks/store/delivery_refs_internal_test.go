package store

import "testing"

// TestTLCV0003_RemoteBranchIndex pins the TLC-V0-003 resolution over one
// ref index built per observation: local first, then origin, then the only
// other remote-tracking ref, with a shared remote name unresolved.
func TestTLCV0003_RemoteBranchIndex(t *testing.T) {
	x := newRefIndex([]byte("a refs/heads/local\nb refs/remotes/origin/local\nc refs/remotes/origin/viaorigin\n" +
		"d refs/remotes/upstream/viaorigin\ne refs/remotes/upstream/only\nf refs/remotes/upstream/two\n" +
		"g refs/remotes/fork/two\nh refs/remotes/upstream/nested/name\n"))
	for branch, want := range map[string]string{
		"local":                 "refs/heads/local",
		"viaorigin":             "refs/remotes/origin/viaorigin",
		"only":                  "refs/remotes/upstream/only",
		"two":                   "",
		"nested/name":           "refs/remotes/upstream/nested/name",
		"absent":                "",
		"refs/remotes/fork/two": "refs/remotes/fork/two",
		"":                      "",
	} {
		if got := x.resolve(branch); got != want {
			t.Errorf("resolve(%q) = %q, want %q", branch, got, want)
		}
	}
	if len(x.remote) != 5 || x.remote["two"] != "" {
		t.Fatalf("remote index %v", x.remote)
	}
}
