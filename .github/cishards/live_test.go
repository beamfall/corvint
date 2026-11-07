package cishards

import (
	"os"
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
	} {
		if got := rootModuleSource(name, modules); got != want {
			t.Errorf("rootModuleSource(%q) = %v, want %v", name, got, want)
		}
	}
}

// AFP-V0-2701/2702: the repository's live tier is non-empty, and no PR shard go test line in ci.yml
// builds with the live tag or enables the tier, so PR shards skip every live test.
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
	for i, line := range strings.Split(string(ci), "\n") {
		if strings.Contains(line, "CORVINT_LIVE_TIER") || (strings.Contains(line, "go test") && liveTag.MatchString(line)) {
			t.Errorf("ci.yml:%d enables the live tier outside make live-test: %s", i+1, strings.TrimSpace(line))
		}
	}
}
