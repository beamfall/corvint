package docsbridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
)

// SDD-V0-700 (V1-3706): an owner source absent from HEAD, untracked or only
// staged, refuses before any index build, in under 100 ms, with exactly the
// tool failure the full rederivation returns; a committed source still builds.
func TestSDDV0700AbsentOwnerSourceRefusesBeforeTheIndex(t *testing.T) {
	root := docsRepository(t)
	for name, text := range map[string]string{"untracked.md": ownerMarkdown, "staged.md": ownerMarkdown} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "add", "staged.md").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, output)
	}
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	full, buildErr := contextindex.BuildContext(context.Background(), root, "")
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	builds := 0
	registry.build = func(ctx context.Context, root, revision string) (*contextindex.Index, error) {
		builds++
		return contextindex.BuildContext(ctx, root, revision)
	}
	for _, source := range []string{"missing.md", "untracked.md", "staged.md", "cache/demux.go"} {
		_, slow := doccompiler.DraftSources(full, source, "cache")
		want, ok := asDoccompilerError(slow)
		if !ok {
			t.Fatalf("%s: the full path does not refuse: %v", source, slow)
		}
		for tool, arguments := range map[string][]byte{
			ToolDraft:   draftArguments(t, source, "cache"),
			ToolConsume: consumeArguments(t, source, "cache", "split keys", "# draft\n"),
		} {
			started := time.Now()
			_, _, refusal, transportErr := registry.Call(context.Background(), tool, arguments)
			elapsed := time.Since(started)
			if transportErr != nil || refusal == nil || refusal.Code != want.Code || refusal.Message != want.Message {
				t.Fatalf("%s %s: refusal %#v transport %#v, want %s %q", tool, source, refusal, transportErr, want.Code, want.Message)
			}
			if elapsed >= 100*time.Millisecond {
				t.Fatalf("%s %s refused after %v", tool, source, elapsed)
			}
		}
	}
	if builds != 0 {
		t.Fatalf("%d index builds for refused sources", builds)
	}
	if _, _, refusal, transportErr := registry.Call(context.Background(), ToolDraft, draftArguments(t, "owner.md", "cache")); refusal != nil || transportErr != nil || builds != 1 {
		t.Fatalf("committed owner: refusal %#v transport %#v builds %d", refusal, transportErr, builds)
	}
}
