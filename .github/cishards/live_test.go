package cishards

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// AFP-V0-2701: a test is live only when its own body calls RequireLiveTier through a selector.
func TestLiveTestsInFileFindsMarkedTopLevelTests_AFPV02701(t *testing.T) {
	src := []byte(`package x

import "github.com/Beamfall/corvint/internal/testsupport"

func TestLive(t *testing.T) { testsupport.RequireLiveTier(t); t.Log("x") }
func TestNested(t *testing.T) { t.Run("a", func(t *testing.T) { testsupport.RequireLiveTier(t) }) }
func TestPlain(t *testing.T) { t.Log("RequireLiveTier") }
func TestBare(t *testing.T) { RequireLiveTier(t) }
func helper(t *testing.T) { testsupport.RequireLiveTier(t) }
func (s suite) TestMethod(t *testing.T) { testsupport.RequireLiveTier(t) }
`)
	got, err := liveTestsInFile("x_test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"TestLive", "TestNested"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("live tests = %v, want %v", got, want)
	}
	if _, err := liveTestsInFile("bad_test.go", []byte("package")); err == nil {
		t.Fatal("an unparsable file must refuse, not drop out of the tier")
	}
}

// AFP-V0-2701: discovery follows the root module's ./... reach.
func TestRootModuleSourceMatchesGoTestReach_AFPV02701(t *testing.T) {
	modules := []string{"benchmarks/snapshot-reader/"}
	for name, want := range map[string]bool{
		"internal/x/a_test.go":                  true,
		"cmd/corvint/a_test.go":                 true,
		"internal/x/testdata/fixture/a_test.go": false,
		"vendor/y/a_test.go":                    false,
		".github/cishards/a_test.go":            false,
		"internal/_old/a_test.go":               false,
		"benchmarks/snapshot-reader/a_test.go":  false,
		"benchmarks/runner/a_test.go":           true,
		"root_test.go":                          true,
	} {
		if got := rootModuleSource(name, modules); got != want {
			t.Errorf("rootModuleSource(%q) = %v, want %v", name, got, want)
		}
	}
	// A tracked module-root test file is discovered as package ".", which live-test-run runs as ./.
	repo := t.TempDir()
	marked := "package x\n\nfunc TestRoot(t *testing.T) { testsupport.RequireLiveTier(t) }\n"
	if err := os.WriteFile(filepath.Join(repo, "root_test.go"), []byte(marked), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "root_test.go"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	got, err := LiveTests(repo)
	if err != nil {
		t.Fatal(err)
	}
	if want := []LivePackage{{Dir: ".", Tests: []string{"TestRoot"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("module-root discovery = %v, want %v", got, want)
	}
}

// AFP-V0-2701/2702: the repository's live tier is non-empty; CORVINT_SKIP_LIVE_TIER is set exactly
// once in ci.yml, in the go-product-shard Go test step, and never in test-commands.json; and no go
// test line builds with the live tag. So PR shards skip every live test and every other run keeps it.
func TestRepositoryLiveTierIsExcludedFromPRShards_AFPV02702(t *testing.T) {
	packages, err := LiveTests("../..")
	if err != nil {
		t.Fatal(err)
	}
	dirs := map[string]int{}
	for _, p := range packages {
		dirs[p.Dir] = len(p.Tests)
	}
	for _, dir := range []string{"internal/liveverify/affected", "internal/companionrelease"} {
		if dirs[dir] == 0 {
			t.Errorf("live tier has no tests in %s; got %v", dir, dirs)
		}
	}
	ci, err := os.ReadFile("../workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	liveTag := regexp.MustCompile(`-tags[= ]+["']?[^ ]*\blive\b`)
	lines := strings.Split(string(ci), "\n")
	var skips []int
	for i, line := range lines {
		if strings.Contains(line, "CORVINT_SKIP_LIVE_TIER") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			skips = append(skips, i)
		}
		if strings.Contains(line, "go test") && liveTag.MatchString(line) {
			t.Errorf("ci.yml:%d builds a go test with the live tag outside make live-test: %s", i+1, strings.TrimSpace(line))
		}
	}
	if len(skips) != 1 {
		t.Fatalf("ci.yml sets CORVINT_SKIP_LIVE_TIER on lines %v, want exactly once in the go-product-shard Go test step", skips)
	}
	if strings.TrimSpace(lines[skips[0]]) != `CORVINT_SKIP_LIVE_TIER: "1"` {
		t.Errorf("ci.yml:%d = %q, want CORVINT_SKIP_LIVE_TIER: \"1\"", skips[0]+1, strings.TrimSpace(lines[skips[0]]))
	}
	job, step := "", ""
	for i := skips[0]; i >= 0; i-- {
		line := lines[i]
		if step == "" && strings.HasPrefix(strings.TrimSpace(line), "- name: ") {
			step = strings.TrimPrefix(strings.TrimSpace(line), "- name: ")
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(line, ":") {
			job = strings.TrimSuffix(strings.TrimSpace(line), ":")
			break
		}
	}
	if job != "go-product-shard" || !strings.HasPrefix(step, "Go tests") {
		t.Errorf("CORVINT_SKIP_LIVE_TIER is in job %q step %q, want the go-product-shard Go tests step", job, step)
	}
	commands, err := os.ReadFile("test-commands.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(commands), "CORVINT_SKIP_LIVE_TIER") {
		t.Error("test-commands.json sets CORVINT_SKIP_LIVE_TIER, so corvint test and its consumers would skip the live tier")
	}
}
