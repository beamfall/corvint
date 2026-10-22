package appflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

func testSelector(t *testing.T, root string, dirty []string, provider SelectionProvider, records []CoverageRecord) *e2eSelector {
	t.Helper()
	in := E2EInput{Provider: "e2e-provider.json", Revision: "HEAD", Graph: &affected.Graph{}, Plan: affected.Plan{Dirty: dirty}}
	if provider.Flows == "" {
		provider.Flows, provider.RunnerConfig = "flows", "playwright.config.ts"
	}
	return newE2ESelector(context.Background(), root, in, provider, records, nil)
}

func generatedPaths(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%06d.ts", prefix, i)
	}
	return out
}

// AFU-V1-065: one coverage tier's path list is bounded at 65,536 paths on its own, not only through
// the 131,072 total: 65,536 plans, 65,537 is the closed e2e-bound-exceeded refusal naming the list.
func TestAFUV1065CoverageTierListCeiling(t *testing.T) {
	t.Parallel()
	for _, n := range []int{MaxSelectionPaths, MaxSelectionPaths + 1} {
		records := []CoverageRecord{{TestKey: "a", Tiers: []CoverageTier{{Tier: "client", Paths: generatedPaths("g/", n)}}}}
		got := testSelector(t, t.TempDir(), nil, SelectionProvider{}, records).boundCodes()
		var want []E2EFallback
		if n > MaxSelectionPaths {
			want = []E2EFallback{{Code: CodeBoundExceeded, Subject: "coverage-tier-paths"}}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%d paths in one tier: codes %+v, want %+v", n, got, want)
		}
	}
	// Two full lists stay within both the per-list and the 131,072 total ceiling.
	full := generatedPaths("g/", MaxSelectionPaths)
	records := []CoverageRecord{{TestKey: "a", Tiers: []CoverageTier{{Tier: "client", Paths: full}, {Tier: "server", Paths: full}}}}
	if got := testSelector(t, t.TempDir(), nil, SelectionProvider{}, records).boundCodes(); len(got) != 0 {
		t.Fatalf("two lists of %d paths: codes %+v", MaxSelectionPaths, got)
	}
}

// AFU-V1-022 / AFU-V1-065: global-path matching keeps the exact under() semantics and is
// sub-quadratic, so 65,536 changed paths against 65,536 global paths stays fast.
func TestAFUV1065GlobalPathMatchingScales(t *testing.T) {
	t.Parallel()
	planted := []string{"web", "a//b", "lib/", "cfg/g000007"}
	globals := append(generatedPaths("cfg/g", MaxSelectionPaths-len(planted)), planted...)
	probes := []string{"web", "web/x.ts", "webapp/x.ts", "a//b/c.ts", "a/b/c.ts", "lib/z.ts", "lib/", "cfg/g000007", "cfg/g000007x",
		"cfg/g000012.ts", "cfg/g000012.ts/deeper", "/abs", "x/web/y.ts", "flows/f.json", "src/Dockerfile.dev", "e2e-provider.json"}
	dirty := append(generatedPaths("src/d/f", MaxSelectionPaths-len(probes)), probes...)
	s := testSelector(t, t.TempDir(), dirty, SelectionProvider{GlobalPaths: globals}, nil)
	want := []string{}
	for _, p := range probes {
		brute := under(p, s.provider.Flows) || p == s.in.Provider || strings.HasPrefix(p, "src/Dockerfile") ||
			slices.ContainsFunc(globals, func(g string) bool { return under(p, g) })
		if got := s.global(p); got != brute {
			t.Fatalf("global(%q) = %v, want %v", p, got, brute)
		}
		if brute {
			want = append(want, p)
		}
	}
	if !slices.Contains(want, "a//b/c.ts") || !slices.Contains(want, "cfg/g000012.ts/deeper") || slices.Contains(want, "lib/z.ts") || slices.Contains(want, "webapp/x.ts") {
		t.Fatalf("brute-force global set %v misses a planted case", want)
	}
	start := time.Now()
	codes := s.globalCodes()
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("globalCodes over %d changed and %d global paths took %s", len(dirty), len(globals), elapsed)
	}
	subjects := []string{}
	for _, c := range codes {
		subjects = append(subjects, c.Subject)
	}
	slices.Sort(subjects)
	slices.Sort(want)
	if !slices.Equal(subjects, want) {
		t.Fatalf("global changed paths %v, want %v", subjects, want)
	}
}

// evidenceRepo builds R, then one commit per path set (each holding R plus only that set), then a
// base commit back at R, so each evidence commit differs from base by exactly its own set.
func evidenceRepo(t *testing.T, sets [][]string) (string, []string, string) {
	t.Helper()
	root := t.TempDir()
	run := func(stdin string, args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(c.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
			"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		c.Stdin = strings.NewReader(stdin)
		var stderr bytes.Buffer
		c.Stderr = &stderr
		out, err := c.Output()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args[0], err, stderr.String())
		}
		return strings.TrimSpace(string(out))
	}
	run("", "init", "-q")
	blob := run("x\n", "hash-object", "-w", "--stdin")
	commit := func(paths []string, parent string) string {
		run("", "read-tree", "--empty")
		var index strings.Builder
		for _, p := range append([]string{"README.md"}, paths...) {
			fmt.Fprintf(&index, "100644 %s\t%s\n", blob, p)
		}
		run(index.String(), "update-index", "--index-info")
		tree := run("", "write-tree")
		args := []string{"commit-tree", tree, "-m", "c"}
		if parent != "" {
			args = append(args, "-p", parent)
		}
		return run("", args...)
	}
	parent := commit(nil, "")
	evidence := []string{}
	for _, set := range sets {
		parent = commit(set, parent)
		evidence = append(evidence, parent)
	}
	return root, evidence, commit(nil, parent)
}

// AFU-V1-065: the changed paths cached across distinct coverage evidence commits are bounded at
// 131,072. Up to the bound every diff is cached; the read that would exceed it drops the cache and
// refuses, and no later read caches anything, so memory stays bounded whatever the commit count.
func TestAFUV1065EvidenceCacheBounded(t *testing.T) {
	t.Parallel()
	first, second := 70000, MaxSelectionEvidencePaths-70000
	root, evidence, base := evidenceRepo(t, [][]string{generatedPaths("a/", first), generatedPaths("b/", second), {"c/x.ts"}})
	s := testSelector(t, root, nil, SelectionProvider{}, nil)
	s.base = base
	if d := s.evidenceDiff(evidence[0]); !d.ok || len(d.set) != first {
		t.Fatalf("first evidence diff ok=%v paths=%d, want %d", d.ok, len(d.set), first)
	}
	if d := s.evidenceDiff(evidence[1]); !d.ok || len(d.set) != second || s.evidencePaths != MaxSelectionEvidencePaths || s.evidenceExceeded {
		t.Fatalf("diff at the bound: ok=%v paths=%d cached=%d exceeded=%v", d.ok, len(d.set), s.evidencePaths, s.evidenceExceeded)
	}
	if d := s.evidenceDiff(evidence[2]); d.ok || !s.evidenceExceeded || len(s.evidence) != 0 {
		t.Fatalf("diff past the bound: ok=%v exceeded=%v cached commits=%d", d.ok, s.evidenceExceeded, len(s.evidence))
	}
	if d := s.evidenceDiff(evidence[0]); d.ok || len(d.set) != 0 || len(s.evidence) != 0 {
		t.Fatalf("a read after the refusal cached %d paths", len(d.set))
	}
	if !s.coverageStale("e2e/t.spec.ts", CoverageRecord{Commit: evidence[0]}, nil) {
		t.Fatal("coverage evidence read after the refusal is not stale")
	}
}

// AFU-V1-065: memoized reach answers across evidence commits stop being cached at the bound.
func TestAFUV1065ReachMemoBounded(t *testing.T) {
	t.Parallel()
	s := testSelector(t, t.TempDir(), nil, SelectionProvider{}, nil)
	diff := &evidenceDiff{ok: true, set: map[string]bool{"src/changed.ts": true}, changedBy: map[string]string{}, reached: map[string]bool{}}
	for _, p := range generatedPaths("src/q", MaxSelectionEvidencePaths+16) {
		if s.reachedSince(diff, p) {
			t.Fatalf("%s reached with no graph", p)
		}
	}
	if !s.reachedSince(diff, "src/changed.ts") || len(diff.reached) != MaxSelectionEvidencePaths {
		t.Fatalf("reach memo holds %d answers, want %d", len(diff.reached), MaxSelectionEvidencePaths)
	}
}

// AFU-V1-066: a proof's digest is over its canonical JSON with the digest member absent (keys sorted
// at every level), not over the struct encoding that still carries an empty digest.
func TestAFUV1066ProofDigestOmitsDigestMember(t *testing.T) {
	t.Parallel()
	proof := E2EProof{Digest: "sha256:ignored", Attestation: linkAttestation,
		Links: []ProofLink{{Flow: "checkout", From: "pay", Target: LinkTarget{Type: "source", Path: "src/<pay>&.ts", StartLine: 3}, ReviewedAt: strings.Repeat("a", 40)}}}
	canonical := `{"attestation":"link completeness is an author attestation","links":[{"flow":"checkout","from":"pay","reviewed_at":"` +
		strings.Repeat("a", 40) + `","target":{"path":"src/<pay>&.ts","start_line":3,"type":"source"}}]}`
	sum := sha256.Sum256([]byte(canonical))
	if got, want := proofDigest(proof), proofDigestPrefix+hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("digest %s, want %s over %s", got, want, canonical)
	}
	withEmpty := proof
	withEmpty.Digest = ""
	raw, err := gokernel.CanonicalJSON(withEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if old := sha256.Sum256(raw); proofDigest(proof) == proofDigestPrefix+hex.EncodeToString(old[:]) {
		t.Fatal("digest still covers the empty digest member")
	}
}

// inventoryProvider encodes a selection provider with n inventory tests, the last one carrying
// extra in its path.
func inventoryProvider(n int, extra string) []byte {
	var b bytes.Buffer
	b.WriteString(`{"schema":"` + SelectionProviderSchema + `","flows":"flows","runner_config":"playwright.config.ts","global_paths":[],"inventory":[`)
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		path := fmt.Sprintf("e2e/t%05d.spec.ts", i)
		if i == n-1 {
			path += extra
		}
		fmt.Fprintf(&b, `{"test_key":"t%05d","project":"chromium","path":%q}`, i, path)
	}
	b.WriteString(`]}`)
	return b.Bytes()
}

// AFU-V1-065 / V1-2164: Decode screens a provider once, so each inventory element is not screened
// again, and a secret inside an element, an unknown element field or a missing project is still
// refused.
func TestAFUV1065InventoryDecodeScreensOnce(t *testing.T) {
	screens := 0
	screenSecrets = func(s string) bool { screens++; return secretscreen.MatchString(s) }
	t.Cleanup(func() { screenSecrets = secretscreen.MatchString })
	var p SelectionProvider
	if err := Decode(inventoryProvider(MaxSelectionTests, ""), &p); err != nil || len(p.Inventory) != MaxSelectionTests {
		t.Fatalf("ceiling provider: %v, %d tests", err, len(p.Inventory))
	}
	if screens != 1 {
		t.Fatalf("Decode screened a %d-test provider %d times, want once", MaxSelectionTests, screens)
	}
	for name, tc := range map[string]struct{ data, want string }{
		"secret in an element": {string(inventoryProvider(3, "?token=ghp_"+strings.Repeat("a", 36))), "flow input contains secret-shaped data"},
		"unknown element key":  {strings.Replace(string(inventoryProvider(2, "")), `"project":"chromium"`, `"project":"chromium","extra":1`, 1), "invalid flow schema"},
		"missing project":      {strings.Replace(string(inventoryProvider(2, "")), `"project":"chromium",`, ``, 1), "invalid flow schema"},
		"null project":         {strings.Replace(string(inventoryProvider(2, "")), `"project":"chromium"`, `"project":null`, 1), "invalid flow schema"},
	} {
		var q SelectionProvider
		if err := Decode([]byte(tc.data), &q); err == nil || err.Error() != tc.want {
			t.Errorf("%s: Decode = %v, want %q", name, err, tc.want)
		}
	}
}

// BenchmarkAFUV1065InventoryDecodeAtCeiling measures Decode of a provider at the 16,384-test
// ceiling (V1-2164).
func BenchmarkAFUV1065InventoryDecodeAtCeiling(b *testing.B) {
	data := inventoryProvider(MaxSelectionTests, "")
	for b.Loop() {
		var p SelectionProvider
		if err := Decode(data, &p); err != nil {
			b.Fatal(err)
		}
	}
}
