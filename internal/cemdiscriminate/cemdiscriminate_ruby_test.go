package cemdiscriminate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate/minirspec"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// rubyScoring bands a count; line 4 is the first band edge.
const rubyScoring = `# Picks how many of the lowest scores to average.
module Scoring
  def self.lowest_count(count)
    if count >= 8
      3
    elsif count >= 4
      2
    else
      1
    end
  end
end
`

// rubyStrongSpec asserts both sides of the line-4 band edge.
const rubyStrongSpec = `require_relative '../../app/models/scoring'

describe 'Scoring.lowest_count' do
  it 'bands at seven' do
    expect(Scoring.lowest_count(7)).to eq(2)
  end
  it 'bands at eight' do
    expect(Scoring.lowest_count(8)).to eq(3)
  end
end
`

// rubyWeakSpec never reaches the line-4 band edge from below, so moving it
// by one changes nothing it can see.
const rubyWeakSpec = `require_relative '../../app/models/scoring'

describe 'Scoring.lowest_count' do
  it 'bands a large set' do
    expect(Scoring.lowest_count(20)).to eq(3)
  end
  it 'bands a small set' do
    expect(Scoring.lowest_count(5)).to eq(2)
  end
end
`

// TestTCQV0066JudgesARubyHunkThroughTheRSpecArm checks TCQ-V0-066: a Ruby hunk
// is judged by the RSpec runner on the same export; a spec that pins the band
// edge discriminates, one that does not leaves survivors inside the hunk, and
// with both cited a mutant survives only when every spec lets it live.
func TestTCQV0066JudgesARubyHunkThroughTheRSpecArm(t *testing.T) {
	t.Parallel()
	name, err := mutate.HostSandbox()
	if err != nil {
		t.Skipf("no sandbox on this host: %v", err)
	}
	probes := map[string][]string{
		"sandbox-exec": {"/usr/bin/sandbox-exec", "-p", "(version 1)(allow default)", "/usr/bin/true"},
		"bwrap":        {"bwrap", "--ro-bind", "/", "/", "--unshare-net", "--", "true"},
	}
	if probe := probes[name]; probe != nil {
		if output, err := exec.Command(probe[0], probe[1:]...).CombinedOutput(); err != nil {
			t.Skipf("host sandbox %s cannot start here: %v: %s", name, err, strings.TrimSpace(string(output)))
		}
	}
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby is not available")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	shim, err := minirspec.Install(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for relative, content := range map[string]string{
		"app/models/scoring.rb":      rubyScoring,
		"spec/models/strong_spec.rb": rubyStrongSpec,
		"spec/models/weak_spec.rb":   rubyWeakSpec,
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testsupport.RunGit(t, git, root, "init", "--quiet")
	testsupport.RunGit(t, git, root, "add", "-A")
	testsupport.RunGit(t, git, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "fixture")
	revision := strings.TrimSpace(testsupport.RunGit(t, git, root, "rev-parse", "HEAD"))

	ctx := context.Background()
	judge, err := Open(ctx, root, revision)
	if err != nil {
		t.Fatal(err)
	}
	defer judge.Close()
	judge.RSpec = []string{ruby, shim}
	hunk := wire.Hunk{Path: "app/models/scoring.rb", NewRange: wire.Range{Start: 4, Count: 1}}
	run := func(tests ...string) wire.DiscriminationWitness {
		return judge.Judge(ctx, hunk, tests, 8, 4*time.Minute)
	}

	strong := run("spec/models/strong_spec.rb")
	if strong.State == wire.DiscriminationNotRun && strings.Contains(strong.Detail, "sandbox") {
		t.Skipf("host refused the nested sandbox: %s", strong.Detail)
	}
	if strong.State != wire.DiscriminationDiscriminates || strong.Killed < 1 || strong.Survived != 0 || strong.Killed > strong.Mutants {
		t.Fatalf("strong spec witness %+v", strong)
	}
	weak := run("spec/models/weak_spec.rb")
	if weak.State != wire.DiscriminationSurvived || weak.Survived < 1 || int64(len(weak.Survivors)) != weak.Survived ||
		weak.Killed+weak.Survived > weak.Mutants || weak.Mutants != strong.Mutants {
		t.Fatalf("weak spec witness %+v", weak)
	}
	for _, survivor := range weak.Survivors {
		if survivor.Line != 4 || survivor.Operator == "" || !strings.Contains(survivor.Description, "app/models/scoring.rb:4") {
			t.Fatalf("survivor outside the hunk or undescribed: %+v", survivor)
		}
	}
	both := run("spec/models/strong_spec.rb", "spec/models/weak_spec.rb")
	if both.State != wire.DiscriminationDiscriminates || both.Survived != 0 || both.Killed != strong.Killed {
		t.Fatalf("a mutant one cited spec kills must not survive: %+v", both)
	}
	if empty := judge.Judge(ctx, wire.Hunk{Path: hunk.Path, NewRange: wire.Range{Start: 4}}, []string{"spec/models/strong_spec.rb"}, 8, time.Minute); empty.State != wire.DiscriminationNotRun {
		t.Fatalf("a hunk that adds no lines must be not-run: %+v", empty)
	}
}
