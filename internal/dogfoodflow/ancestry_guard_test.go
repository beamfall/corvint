package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-772: every dogfood flow refuses a grafted or shallow checkout before an ancestry read,
// naming the reason and the fix, and runs no verifier.
func TestFlowsRefuseRewrittenAncestry_EEPV0772(t *testing.T) {
	repo := newBatchRepo(t)
	repo.write("a.txt", "a1\nA2\na3\n")
	repo.commit("change")
	for _, fixture := range []struct {
		name, reason string
		apply        func() func()
	}{
		{"grafted", "checkout rewrites ancestry with info/grafts", func() func() { return ancestrytest.Graft(t, repo.root, "HEAD") }},
		{"shallow", "checkout is shallow", func() func() { return ancestrytest.Shallow(t, repo.root, "HEAD") }},
	} {
		undo := fixture.apply()
		want := "REFUSE ancestry-rewritten\n  " + fixture.reason + ", so BASE..HEAD ancestry cannot be trusted\n"
		if code, _, stderr := repo.check(); code != 2 || !strings.HasPrefix(stderr, "dogfood-batch-check: "+want) || len(repo.args) != 0 {
			t.Errorf("%s batch-check exit=%d stderr=%q verifier=%q", fixture.name, code, stderr, repo.args)
		}
		var stdout, stderr bytes.Buffer
		code, err := Check(context.Background(), CheckOptions{Root: repo.root, Base: repo.base}, &stdout, &stderr)
		if err != nil || code != 2 || !strings.HasPrefix(stderr.String(), "dogfood-check: "+want) || !strings.Contains(stderr.String(), "git fetch --unshallow") {
			t.Errorf("%s check exit=%d err=%v stderr=%q", fixture.name, code, err, stderr.String())
		}
		undo()
	}
	if code, _, stderr := repo.check(); strings.Contains(stderr, "ancestry-rewritten") {
		t.Fatalf("plain checkout refused: exit=%d stderr=%q", code, stderr)
	}
}
