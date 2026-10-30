// Command cem-release-suites runs the sealed protocol/cem-1.0 stable suites
// against two built release binaries, corvint and cem01-go, and refuses when
// the two were not built from the same clean commit
// (docs/specs/cem-stable-v1.md CEM-V1-572, V1-2323).
//
// Usage:
//
//	cem-release-suites --corvint BIN --cem01 BIN [--root DIR] [--readiness PATH [--record OUT] [--outcome OUT]]
//	cem-release-suites --aggregate [--root DIR] --readiness PATH [--outcome OUT] [--ci-unobservable ROW[,ROW...]] RECORD...
//
// Each binary must carry Go build information naming one vcs.revision with
// vcs.modified=false, the two revisions must be equal, and the checkout at
// --root must be at that revision. Build stamps are unauthenticated, so the
// command reports the revision as the binaries' reported one and pins each
// binary by SHA-256; it does not prove how the binaries were built. Every
// input is then read from the revision's committed Git objects, written to a
// private tree, never from the checkout's worktree. The tamper matrix,
// citation vectors, neutral criterion and receipt authentication cases run
// through `corvint cem verify-stable` and `cem01-go verify-stable-canonical`;
// each result is checked against the sealed expectation and the two binaries'
// result lines, runtime.goVersion aside, must be byte-identical. The
// repository-envelope cells are prose recipes rebuilt by the S0E harnesses:
// they run as source-level Go tests in that private tree, and then once per
// binary through the portable consumer's cell recipes with the built binary
// as each cell's verifier (CEM-V1-609, CEM-V1-610).
//
// With --readiness PATH, the command also judges release readiness
// (CEM-V1-600..603): PATH names a cem-release-readiness/1 declaration in the
// same committed tree, every accepted stable result line both binaries
// produced is read by internal/companionrelease as the exact cem/1.0 stable
// result, and each declared runner, platform and external row must be
// observed PASSED. The command observes only its own suite rows and the one
// platform both binaries were built for; a binary-level repository-envelope
// row is PASSED only when all 29 cells passed against that binary, and every
// external row is NOT_OBSERVED, so a declaration that requires one is refused
// with exit 3 after the suites pass.
//
// --record writes the run's cem-release-run/1 record (CEM-V1-611) and
// --outcome its cem-release-gate-outcome/1 verdict (CEM-V1-613). --aggregate
// judges readiness over records of one revision from several hosts, so each
// release platform row can be observed by its own run (CEM-V1-612). Records
// and outcomes are unauthenticated run output. --ci-unobservable, accepted
// only with --aggregate, is the PR and main-push CI scope of AFP-V0-432: the
// caller attests that the named rows, which must be exactly the declaration's
// external rows, are not CI-observable. The judgment and the outcome stay
// strict and those rows stay NOT_OBSERVED; only the exit becomes 0 when the
// strict refusal is the external class alone, and each such row is printed as
// a known blocker. Without the flag the command is strict.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

const (
	stableDir     = "protocol/cem-1.0/stable"
	corvintModule = "github.com/Beamfall/corvint"
	corvintMain   = "github.com/Beamfall/corvint/cmd/corvint"
	cem01Main     = "github.com/Beamfall/corvint/interop/cem01-go"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cem-release-suites", flag.ContinueOnError)
	flags.SetOutput(stderr)
	corvint := flags.String("corvint", "", "built corvint binary")
	cem01 := flags.String("cem01", "", "built cem01-go binary")
	root := flags.String("root", ".", "repository checkout the binaries were built from")
	readiness := flags.String("readiness", "", "repository-relative cem-release-readiness/1 declaration read from the build commit")
	record := flags.String("record", "", "write this run's cem-release-run/1 record here (requires --readiness)")
	aggregate := flags.Bool("aggregate", false, "judge readiness over the cem-release-run/1 records named as arguments")
	outcome := flags.String("outcome", "", "write the cem-release-gate-outcome/1 readiness verdict here (requires --readiness)")
	ciUnobservable := flags.String("ci-unobservable", "", "with --aggregate: comma-separated rows the caller attests no CI host observes, exactly the declaration's external rows; exit 0 when only they refuse (AFP-V0-432); the outcome stays strict")
	usage := "usage: cem-release-suites --corvint BIN --cem01 BIN [--root DIR] [--readiness PATH [--record OUT] [--outcome OUT]]\n       cem-release-suites --aggregate [--root DIR] --readiness PATH [--outcome OUT] [--ci-unobservable ROW[,ROW...]] RECORD..."
	err := flags.Parse(args)
	if err != nil || *readiness != "" && !repositoryRelative(*readiness) || (*record != "" || *outcome != "") && *readiness == "" ||
		*aggregate && (*readiness == "" || *corvint != "" || *cem01 != "" || *record != "" || flags.NArg() == 0) ||
		!*aggregate && (*corvint == "" || *cem01 == "" || flags.NArg() != 0 || *ciUnobservable != "") {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites:", err)
		return 2
	}
	if *aggregate {
		return aggregateRuns(absRoot, *readiness, flags.Args(), *outcome, *ciUnobservable, stdout, stderr)
	}
	for _, out := range []string{*record, *outcome} {
		if err := absentOutput(out); err != nil {
			fmt.Fprintln(stderr, "cem-release-suites:", err)
			return 2
		}
	}
	revision, platform, err := releaseBuild(*corvint, *cem01, absRoot)
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites: refused:", err)
		return 1
	}
	sums, err := binaryDigests(*corvint, *cem01)
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites:", err)
		return 2
	}
	fmt.Fprintln(stdout, provenanceLine(revision, sums))
	r := &runner{bins: [2]binary{{name: "corvint", argv: []string{mustAbs(*corvint), "cem", "verify-stable"}}, {name: "cem01-go", argv: []string{mustAbs(*cem01), "verify-stable-canonical"}}}}
	tmp, err := privateDir()
	if tmp != "" {
		defer os.RemoveAll(tmp)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cem-release-suites:", err)
		return 2
	}
	r.tmp = tmp
	src := filepath.Join(tmp, "src")
	if err := materialize(absRoot, revision, src); err != nil {
		fmt.Fprintln(stderr, "cem-release-suites: refused:", err)
		return 1
	}
	var declaration companionrelease.StableReleaseDeclaration
	var declarationRaw []byte
	if *readiness != "" {
		declarationRaw, err = os.ReadFile(filepath.Join(src, filepath.FromSlash(*readiness)))
		if err == nil {
			declaration, err = companionrelease.ReadStableReleaseDeclaration(declarationRaw)
		}
		if err != nil {
			fmt.Fprintln(stderr, "cem-release-suites: refused: readiness declaration:", err)
			return 1
		}
	}
	for _, s := range []struct {
		name string
		fn   func(string) error
	}{{"tamper-matrix", r.tamperMatrix}, {"citation-vectors", r.citationVectors}, {"neutral-criterion", r.neutralCriterion}, {"receipt-authentication", r.receiptAuthentication}} {
		before := r.runs
		if err := s.fn(filepath.Join(src, stableDir, s.name)); err != nil {
			fmt.Fprintf(stderr, "cem-release-suites: %s: FAIL: %v\n", s.name, err)
			return 1
		}
		fmt.Fprintf(stdout, "cem-release-suites: %s: PASS (%d cases on both binaries)\n", s.name, r.runs-before)
	}
	for _, cmd := range sourceSuites(src) {
		cmd.Stdout, cmd.Stderr = stdout, stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(stderr, "cem-release-suites: repository-envelope: FAIL: %v in %s: %v\n", cmd.Args, cmd.Dir, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "cem-release-suites: repository-envelope: PASS at source level on the committed tree of %s\n", revision)
	envelope := map[string]string{}
	for _, b := range r.bins {
		cmd, report := binaryEnvelopeSuite(src, tmp, b)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		outcome, detail := envelopeOutcome(cmd.Run(), report)
		if outcome == "FAILED" {
			fmt.Fprintf(stderr, "cem-release-suites: %s:repository-envelope: FAIL: %s\n", b.name, detail)
			return 1
		}
		envelope[b.name] = outcome
		fmt.Fprintf(stdout, "cem-release-suites: %s:repository-envelope: %s against the built binary (%s)\n", b.name, outcome, detail)
	}
	if *readiness == "" {
		return 0
	}
	runnerRows, platformRows, externalRows := observedRows(platform, envelope)
	var records []string
	if *record != "" {
		rec := runRecord{Profile: runRecordProfile, Revision: revision, Platform: platform,
			Declaration: recordFile{Path: *readiness, Sha256: digest(declarationRaw)},
			Binaries:    []recordBinary{{Name: "corvint", Main: corvintMain, Sha256: sums[0]}, {Name: "cem01-go", Main: cem01Main, Sha256: sums[1]}},
			Runner:      runnerRows, Platforms: platformRows, Accepted: []string{}}
		for _, line := range r.accepted {
			rec.Accepted = append(rec.Accepted, string(line))
		}
		raw, err := encodeRunRecord(rec)
		if err == nil {
			err = writeNew(*record, raw)
		}
		if err != nil {
			fmt.Fprintln(stderr, "cem-release-suites: record:", err)
			return 2
		}
		records = []string{digest(raw)}
		fmt.Fprintf(stdout, "cem-release-suites: record: %s sha256 %s (unauthenticated run output)\n", *record, records[0])
	}
	code, reason := judgeReadiness(declaration, r.accepted, runnerRows, platformRows, externalRows, stdout)
	return writeOutcome(*outcome, revision, digest(declarationRaw), records, code, reason, stdout, stderr)
}

// releaseSuites are the binary-level suites, in run order.
var releaseSuites = []string{"tamper-matrix", "citation-vectors", "neutral-criterion", "receipt-authentication"}

// observedRows is what one passing run observed (CEM-V1-602, CEM-V1-610):
// each binary suite on each binary, the source-level repository-envelope
// suite, each binary's repository-envelope outcome as envelope reports it
// (NOT_RUN when absent), and the one platform the two binaries report. It
// never observes an external row.
func observedRows(platform string, envelope map[string]string) (runner, platforms, external map[string]string) {
	runner = map[string]string{"source:repository-envelope": "PASSED"}
	for _, name := range []string{"corvint", "cem01-go"} {
		outcome := envelope[name]
		if outcome == "" {
			outcome = "NOT_RUN"
		}
		runner[name+":repository-envelope"] = outcome
		for _, suite := range releaseSuites {
			runner[name+":"+suite] = "PASSED"
		}
	}
	platforms = map[string]string{}
	if platform != "" {
		platforms[platform] = "PASSED"
	}
	return runner, platforms, map[string]string{}
}

// judgeReadiness prints every declared row's observed outcome and the
// readiness verdict: exit 0 when ready, 3 when refused (CEM-V1-603), with the
// refusal's text.
func judgeReadiness(d companionrelease.StableReleaseDeclaration, accepted [][]byte, runner, platforms, external map[string]string, stdout io.Writer) (int, string) {
	for _, class := range []struct {
		name     string
		rows     []string
		observed map[string]string
	}{{"runner", d.Runner, runner}, {"platform", d.Platform, platforms}, {"external", d.External, external}} {
		for _, row := range class.rows {
			outcome, ok := class.observed[row]
			if !ok {
				outcome = "NOT_OBSERVED"
			}
			fmt.Fprintf(stdout, "cem-release-suites: readiness: %s %s %s\n", class.name, row, outcome)
		}
	}
	if err := companionrelease.StableReleaseReadiness(d, accepted, runner, platforms, external); err != nil {
		fmt.Fprintf(stdout, "cem-release-suites: readiness: REFUSED (%d accepted stable results read): %v\n", len(accepted), err)
		return 3, err.Error()
	}
	fmt.Fprintf(stdout, "cem-release-suites: readiness: READY (%d accepted stable results read); owner promotion is separate\n", len(accepted))
	return 0, ""
}

// repositoryRelative reports whether p is a clean slash-separated path inside
// the repository tree.
func repositoryRelative(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") && filepath.ToSlash(filepath.Clean(filepath.FromSlash(p))) == p && p != "." && !strings.HasPrefix(p, "../") && p != ".."
}

// privateDir makes the run's private directory with every symbolic link in
// its path resolved. The stable verifiers refuse a repository whose path runs
// through a symbolic link (macOS TMPDIR lives under the /var link), so the case
// repositories must sit on a resolved path.
func privateDir() (string, error) {
	tmp, err := os.MkdirTemp("", "cem-release-suites-")
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		return tmp, err
	}
	return resolved, nil
}

// provenanceReport names the revision both binaries report and pins each
// binary's bytes by SHA-256. A Go build stamp is unauthenticated metadata that
// an edited binary can carry, so the line says reported, never proven.
func provenanceReport(revision, corvint, cem01 string) (string, error) {
	sums, err := binaryDigests(corvint, cem01)
	if err != nil {
		return "", err
	}
	return provenanceLine(revision, sums), nil
}

func provenanceLine(revision string, sums [2]string) string {
	return fmt.Sprintf("cem-release-suites: both binaries report clean vcs.revision %s (unauthenticated build stamps, not verified build provenance); corvint sha256 %s, cem01-go sha256 %s", revision, sums[0], sums[1])
}

// binaryDigests is the SHA-256 of each binary's bytes.
func binaryDigests(corvint, cem01 string) ([2]string, error) {
	sums := [2]string{}
	for i, path := range []string{corvint, cem01} {
		raw, err := os.ReadFile(path)
		if err != nil {
			return sums, err
		}
		sums[i] = digest(raw)
	}
	return sums, nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// sourceSuites are the source-level repository-envelope tests, run in the
// committed tree src rather than in the mutable checkout. They inherit no
// envelope binary or conformance output, so an inherited selection cannot
// replace the source-level verifier (CEM-V1-609).
func sourceSuites(src string) []*exec.Cmd {
	native := exec.Command("go", "test", "-count=1", "-timeout", "30m", "-run", "^(TestStableS0EPublicCases|TestStableS0ELinkedControl|TestStableCanonicalEnvelopeCellsNative|TestStableEnvelopeAdversarialShapes)$", "./internal/cem/verify")
	native.Dir = src
	portable := exec.Command("go", "test", "-count=1", "-timeout", "30m", "-run", "^(TestStableCanonicalEnvelopeCells|TestStableEnvelopeAdversarialShapes)$", ".")
	portable.Dir = filepath.Join(src, "interop", "cem01-go")
	for _, cmd := range []*exec.Cmd{native, portable} {
		cmd.Env = withoutEnvelopeSelection(withoutGitSelection(os.Environ()))
	}
	return []*exec.Cmd{native, portable}
}

// envelopeCells is the number of public repository-envelope cells.
const envelopeCells = 29

// binaryEnvelopeSuite runs the 29 public repository-envelope cells against one
// built binary (CEM-V1-609). The cells' recipes are the portable consumer's
// TestStableCanonicalEnvelopeCells in the committed tree src; each cell's
// verification runs b's argv instead of the test binary, and the per-cell
// conformance record is written to the returned report path.
func binaryEnvelopeSuite(src, tmp string, b binary) (*exec.Cmd, string) {
	report := filepath.Join(tmp, "envelope-"+b.name+".json")
	argv, _ := json.Marshal(b.argv)
	cmd := exec.Command("go", "test", "-count=1", "-timeout", "30m", "-run", "^TestStableCanonicalEnvelopeCells$", ".")
	cmd.Dir = filepath.Join(src, "interop", "cem01-go")
	cmd.Env = append(withoutEnvelopeSelection(withoutGitSelection(os.Environ())), "CEM01_ENVELOPE_BINARY="+string(argv), "CEM01_S0E_CONFORMANCE_OUT="+report)
	return cmd, report
}

// withoutEnvelopeSelection drops an inherited envelope binary or conformance
// output, so only this run names them.
func withoutEnvelopeSelection(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "CEM01_ENVELOPE_BINARY=") && !strings.HasPrefix(kv, "CEM01_S0E_CONFORMANCE_OUT=") {
			out = append(out, kv)
		}
	}
	return out
}

// envelopeOutcome judges one binary envelope run (CEM-V1-610): FAILED when the
// test run failed, its report is unreadable, or any recorded cell did not pass
// against the named binary; PASSED only when all 29 distinct cells passed in
// RUN_BINARY mode; otherwise NOT_RUN, the cells this host could not build.
func envelopeOutcome(runErr error, reportPath string) (string, string) {
	if runErr != nil {
		return "FAILED", fmt.Sprintf("envelope cells run: %v", runErr)
	}
	raw, err := os.ReadFile(reportPath)
	if errors.Is(err, os.ErrNotExist) {
		return "NOT_RUN", fmt.Sprintf("0 of %d cells ran on this host", envelopeCells)
	}
	if err != nil {
		return "FAILED", fmt.Sprintf("conformance report: %v", err)
	}
	var report struct {
		Profile string `json:"profile"`
		Cells   []struct {
			Name   string `json:"name"`
			Mode   string `json:"mode"`
			Status string `json:"status"`
		} `json:"envelopeCells"`
	}
	if err := json.Unmarshal(raw, &report); err != nil || report.Profile != "cem01-go-s0e-portable-conformance/1" {
		return "FAILED", fmt.Sprintf("conformance report is not cem01-go-s0e-portable-conformance/1: %v", err)
	}
	seen := map[string]bool{}
	for _, c := range report.Cells {
		if c.Mode != "RUN_BINARY" || c.Status != "PASS" || seen[c.Name] {
			return "FAILED", fmt.Sprintf("cell %s recorded %s %s", c.Name, c.Mode, c.Status)
		}
		seen[c.Name] = true
	}
	if len(seen) != envelopeCells {
		return "NOT_RUN", fmt.Sprintf("%d of %d cells ran on this host", len(seen), envelopeCells)
	}
	return "PASSED", fmt.Sprintf("%d of %d cells passed", len(seen), envelopeCells)
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// releaseBuild refuses unless both binaries report the same clean commit and
// the checkout at root is at that commit. Inputs are read from the commit's
// objects by materialize, so worktree edits, skip-worktree and
// assume-unchanged entries cannot reach a suite.
func releaseBuild(corvint, cem01, root string) (string, string, error) {
	a, err := buildinfo.ReadFile(corvint)
	if err != nil {
		return "", "", fmt.Errorf("corvint build information: %v", err)
	}
	b, err := buildinfo.ReadFile(cem01)
	if err != nil {
		return "", "", fmt.Errorf("cem01-go build information: %v", err)
	}
	revision, err := sameBuild(a, b)
	if err != nil {
		return "", "", err
	}
	head, err := gitOutput(root, "", nil, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	if head != revision {
		return "", "", fmt.Errorf("checkout HEAD %s is not the build revision %s; check out the build commit", head, revision)
	}
	return revision, buildPlatform(a, b), nil
}

// buildPlatform is the GOOS/GOARCH both builds report, or "" when either is
// missing or the two differ: an unreported platform is never observed.
func buildPlatform(builds ...*debug.BuildInfo) string {
	platform := ""
	for i, info := range builds {
		settings := map[string]string{}
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		if settings["GOOS"] == "" || settings["GOARCH"] == "" {
			return ""
		}
		p := settings["GOOS"] + "/" + settings["GOARCH"]
		if i > 0 && p != platform {
			return ""
		}
		platform = p
	}
	return platform
}

// materialize writes every file of revision's tree, byte for byte from its
// committed blob, under dst. Only regular blobs are admitted.
func materialize(root, revision, dst string) error {
	listing, err := gitOutput(root, "", nil, "ls-tree", "-r", "-z", "--full-tree", revision)
	if err != nil {
		return err
	}
	type entry struct{ mode, oid, path string }
	var entries []entry
	var oids strings.Builder
	for _, record := range strings.Split(listing, "\x00") {
		if record == "" {
			continue
		}
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" || fields[0] != "100644" && fields[0] != "100755" || !filepath.IsLocal(path) {
			return fmt.Errorf("tree entry %q at %s is not a regular committed file", record, revision)
		}
		entries = append(entries, entry{fields[0], fields[2], path})
		oids.WriteString(fields[2] + "\n")
	}
	cmd := gitCommand(root, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(oids.String())
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	read := func() error {
		out := bufio.NewReader(pipe)
		for _, e := range entries {
			header, err := out.ReadString('\n')
			if err != nil {
				return err
			}
			var oid, kind string
			var size int64
			if _, err := fmt.Sscanf(header, "%s %s %d\n", &oid, &kind, &size); err != nil || oid != e.oid || kind != "blob" {
				return fmt.Errorf("git cat-file returned %q for %s", header, e.path)
			}
			body := make([]byte, size+1)
			if _, err := io.ReadFull(out, body); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if e.mode == "100755" {
				mode = 0o755
			}
			target := filepath.Join(dst, filepath.FromSlash(e.path))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, body[:size], mode); err != nil {
				return err
			}
		}
		return nil
	}
	readErr := read()
	if readErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return fmt.Errorf("materialize %s: %v", revision, readErr)
	}
	if waitErr != nil {
		return fmt.Errorf("materialize %s: git cat-file: %v", revision, waitErr)
	}
	return nil
}

// sameBuild returns the shared vcs.revision of a corvint and a cem01-go build,
// refusing a wrong main package, a missing or modified revision, or two
// different revisions.
func sameBuild(corvint, cem01 *debug.BuildInfo) (string, error) {
	if corvint.Path != corvintMain || corvint.Main.Path != corvintModule {
		return "", fmt.Errorf("corvint binary is %s (module %s), not %s", corvint.Path, corvint.Main.Path, corvintMain)
	}
	if cem01.Path != cem01Main {
		return "", fmt.Errorf("cem01-go binary is %s, not %s", cem01.Path, cem01Main)
	}
	revisions := [2]string{}
	for i, info := range []*debug.BuildInfo{corvint, cem01} {
		settings := map[string]string{}
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		name := [2]string{"corvint", "cem01-go"}[i]
		if settings["vcs"] != "git" || settings["vcs.revision"] == "" {
			return "", fmt.Errorf("%s carries no Git vcs.revision; build it from a Git checkout without -buildvcs=false", name)
		}
		if settings["vcs.modified"] != "false" {
			return "", fmt.Errorf("%s was built from a modified worktree at %s", name, settings["vcs.revision"])
		}
		revisions[i] = settings["vcs.revision"]
	}
	if revisions[0] != revisions[1] {
		return "", fmt.Errorf("builds differ: corvint %s, cem01-go %s", revisions[0], revisions[1])
	}
	return revisions[0], nil
}

type binary struct {
	name string
	argv []string
}

type runner struct {
	tmp  string
	bins [2]binary
	runs int
	// accepted holds both binaries' result lines of every case the two
	// accepted, for release readiness (CEM-V1-601).
	accepted [][]byte
}

type stableRun struct {
	exit   int
	line   []byte
	result map[string]any
}

// verify runs one case through both binaries and requires their result lines,
// runtime aside, to be byte-identical; it returns the corvint run.
func (r *runner) verify(repo string, mapBytes []byte, base, target, artifacts, trustedKeys string) (stableRun, error) {
	r.runs++
	mapPath := filepath.Join(r.tmp, fmt.Sprintf("map-%d.json", r.runs))
	if err := os.WriteFile(mapPath, mapBytes, 0o600); err != nil {
		return stableRun{}, err
	}
	var runs [2]stableRun
	for i, b := range r.bins {
		argv := append(append([]string(nil), b.argv[1:]...), "--repository", repo, "--map", mapPath, "--expected-base", base, "--target", target, "--artifacts", artifacts)
		if trustedKeys != "" {
			argv = append(argv, "--trusted-keys", trustedKeys)
		}
		cmd := exec.Command(b.argv[0], argv...)
		cmd.Dir = r.tmp
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		var exitErr *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &exitErr):
			runs[i].exit = exitErr.ExitCode()
		default:
			return stableRun{}, fmt.Errorf("%s: %v", b.name, err)
		}
		runs[i].line = bytes.TrimSuffix(out.Bytes(), []byte("\n"))
		if err := json.Unmarshal(runs[i].line, &runs[i].result); err != nil {
			return stableRun{}, fmt.Errorf("%s printed no result line: %v: %s", b.name, err, out.Bytes())
		}
		if err := runtimeConsistent(runs[i].result); err != nil {
			return stableRun{}, fmt.Errorf("%s: %v: %s", b.name, err, runs[i].line)
		}
	}
	a, b := withoutGoVersion(runs[0].line), withoutGoVersion(runs[1].line)
	if runs[0].exit != runs[1].exit || !bytes.Equal(a, b) {
		return stableRun{}, fmt.Errorf("consumers differ: corvint exit %d %s; cem01-go exit %d %s", runs[0].exit, a, runs[1].exit, b)
	}
	if runs[0].result["accept"] == true {
		r.accepted = append(r.accepted, runs[0].line, runs[1].line)
	}
	return runs[0], nil
}

// withoutGoVersion empties runtime.goVersion, the one value another admitted
// toolchain changes in a sealed line, and keeps every other byte, including
// runtime.structuralQualification.
func withoutGoVersion(line []byte) []byte {
	key := []byte(`"runtime":{"goVersion":"`)
	start := bytes.Index(line, key)
	if start < 0 {
		return line
	}
	value := start + len(key)
	end := bytes.IndexByte(line[value:], '"')
	if end < 0 {
		return line
	}
	return append(append([]byte(nil), line[:value]...), line[value+end:]...)
}

// runtimeConsistent refuses a runtime record that is not exactly goVersion and
// structuralQualification, or whose qualification contradicts the structural
// proof axis: no structural hunk means NOT_REQUIRED, and UNSUPPORTED_RUNTIME
// is only reported with an UNSUPPORTED qualification.
func runtimeConsistent(result map[string]any) error {
	runtime, _ := result["runtime"].(map[string]any)
	version, _ := runtime["goVersion"].(string)
	qualification, _ := runtime["structuralQualification"].(string)
	if len(runtime) != 2 || version == "" || qualification == "" {
		return fmt.Errorf("runtime %v is not one goVersion and structuralQualification", result["runtime"])
	}
	axes, _ := result["axes"].(map[string]any)
	proof, _ := axes["structuralProof"].(string)
	if proof == "NOT_REQUIRED" && qualification != "NOT_REQUIRED" || (proof == "UNSUPPORTED_RUNTIME") != (qualification == "UNSUPPORTED") {
		return fmt.Errorf("structuralQualification %s contradicts structuralProof %s", qualification, proof)
	}
	return nil
}

func resultExceptRuntime(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if k != "runtime" {
			out[k] = v
		}
	}
	return out
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// recipe builds a public case's recipe repository with Git plumbing and the
// recipe's fixed identity, as each case README specifies.
func (r *runner) recipe(dir string) (repo, base, target string, err error) {
	var recipe struct {
		ObjectFormat string                             `json:"objectFormat"`
		Identity     struct{ Name, Email, Date string } `json:"identity"`
		Commits      []struct {
			Name    string            `json:"name"`
			Message string            `json:"message"`
			Files   map[string]string `json:"files"`
		} `json:"commits"`
	}
	if err = readJSON(filepath.Join(dir, "repository.recipe.json"), &recipe); err != nil {
		return
	}
	if recipe.ObjectFormat != "sha1" {
		return "", "", "", fmt.Errorf("recipe object format %q is not sha1", recipe.ObjectFormat)
	}
	repo = filepath.Join(r.tmp, "repo-"+filepath.Base(dir))
	if err = os.Mkdir(repo, 0o755); err != nil {
		return
	}
	env := []string{"HOME=/nonexistent-cem-release-suites-home", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C",
		"GIT_AUTHOR_NAME=" + recipe.Identity.Name, "GIT_AUTHOR_EMAIL=" + recipe.Identity.Email, "GIT_AUTHOR_DATE=" + recipe.Identity.Date,
		"GIT_COMMITTER_NAME=" + recipe.Identity.Name, "GIT_COMMITTER_EMAIL=" + recipe.Identity.Email, "GIT_COMMITTER_DATE=" + recipe.Identity.Date}
	git := func(stdin string, args ...string) string {
		if err != nil {
			return ""
		}
		var out string
		out, err = gitOutput(repo, stdin, env, args...)
		return out
	}
	git("", "init", "-q", "--object-format=sha1")
	for _, commit := range recipe.Commits {
		names := make([]string, 0, len(commit.Files))
		for name := range commit.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		var tree strings.Builder
		for _, name := range names {
			tree.WriteString("100644 blob " + git(commit.Files[name], "hash-object", "-w", "--stdin") + "\t" + name + "\n")
		}
		args := []string{"commit-tree", git(tree.String(), "mktree"), "-m", commit.Message}
		if target != "" {
			args = append(args, "-p", target)
		}
		target = git("", args...)
		if commit.Name == "base" {
			base = target
		}
	}
	git("", "update-ref", "refs/heads/main", target)
	git("", "symbolic-ref", "HEAD", "refs/heads/main")
	return
}

// gitCommand runs git on the repository at dir only: inherited repository
// selection (GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE and every other GIT_*
// variable) is dropped, so -C dir names the repository that is read.
func gitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = withoutGitSelection(os.Environ())
	return cmd
}

func withoutGitSelection(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "GIT_") {
			out = append(out, kv)
		}
	}
	return out
}

func gitOutput(dir, stdin string, env []string, args ...string) (string, error) {
	cmd := gitCommand(dir, args...)
	if env != nil {
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	}
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *runner) caseRepository(dir, wantBase, wantTarget string) (string, error) {
	repo, base, target, err := r.recipe(dir)
	if err != nil {
		return "", err
	}
	if base != wantBase || target != wantTarget {
		return "", fmt.Errorf("recipe commits %s..%s, want %s..%s", base, target, wantBase, wantTarget)
	}
	return repo, nil
}

func (r *runner) tamperMatrix(dir string) error {
	var want struct {
		Profile        string `json:"profile"`
		ArtifactRoot   string `json:"artifactRoot"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Cases          []struct {
			Name       string         `json:"name"`
			Map        string         `json:"map"`
			Exit       int            `json:"exit"`
			Outcome    string         `json:"outcome"`
			Stage      string         `json:"stage"`
			Code       *string        `json:"code"`
			IssueCodes []string       `json:"issueCodes"`
			Citation   map[string]any `json:"citation"`
		} `json:"cases"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	if want.Profile != "cem-stable-tamper-matrix/1" || len(want.Cases) == 0 {
		return fmt.Errorf("EXPECTED.json profile %q with %d cases", want.Profile, len(want.Cases))
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	for _, c := range want.Cases {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(c.Map)))
		if err != nil {
			return err
		}
		got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, filepath.Join(dir, want.ArtifactRoot), "")
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		var citation map[string]any
		if v, ok := got.result["citation"].(map[string]any); ok {
			citation = v
		}
		var code *string
		if s, ok := got.result["code"].(string); ok {
			code = &s
		}
		if got.exit != c.Exit || got.result["outcome"] != c.Outcome || got.result["stage"] != c.Stage || !reflect.DeepEqual(code, c.Code) ||
			fmt.Sprint(got.result["issueCodes"]) != fmt.Sprint(stringsAny(c.IssueCodes)) || !reflect.DeepEqual(citation, c.Citation) {
			return fmt.Errorf("%s: exit %d %s, want the sealed verdict", c.Name, got.exit, got.line)
		}
	}
	return nil
}

func stringsAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func (r *runner) citationVectors(dir string) error {
	var want struct {
		Profile        string `json:"profile"`
		ArtifactRoot   string `json:"artifactRoot"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Cases          []struct {
			Name string `json:"name"`
			Map  string `json:"map"`
			Exit int    `json:"exit"`
		} `json:"cases"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	if want.Profile != "cem-stable-citation-vectors/1" || len(want.Cases) == 0 {
		return fmt.Errorf("EXPECTED.json profile %q with %d cases", want.Profile, len(want.Cases))
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	for _, c := range want.Cases {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(c.Map)))
		if err != nil {
			return err
		}
		sealed, err := os.ReadFile(filepath.Join(dir, "results", c.Name+".json"))
		if err != nil {
			return err
		}
		got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, filepath.Join(dir, want.ArtifactRoot), "")
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		if got.exit != c.Exit || !bytes.Equal(withoutGoVersion(got.line), withoutGoVersion(bytes.TrimSuffix(sealed, []byte("\n")))) {
			return fmt.Errorf("%s: exit %d %s, want exit %d and the sealed result line", c.Name, got.exit, got.line, c.Exit)
		}
	}
	return nil
}

func (r *runner) neutralCriterion(dir string) error {
	var want struct {
		Profile        string `json:"profile"`
		ArtifactRoot   string `json:"artifactRoot"`
		Map            string `json:"map"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			Exit   int            `json:"exit"`
			Result map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		Refusals []struct {
			Name       string          `json:"name"`
			Criterion  json.RawMessage `json:"criterion"`
			RebindLink bool            `json:"rebindLink"`
			Exit       int             `json:"exit"`
			Outcome    string          `json:"outcome"`
			Stage      string          `json:"stage"`
			Code       string          `json:"code"`
		} `json:"refusals"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, want.Map))
	if err != nil {
		return err
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, artifacts, "")
	if err != nil {
		return fmt.Errorf("accept: %v", err)
	}
	sealed, err := os.ReadFile(filepath.Join(dir, "results", "accept.json"))
	if err != nil {
		return err
	}
	if got.exit != want.Accept.Exit || !reflect.DeepEqual(resultExceptRuntime(got.result), want.Accept.Result) || !bytes.Equal(withoutGoVersion(got.line), withoutGoVersion(bytes.TrimSuffix(sealed, []byte("\n")))) {
		return fmt.Errorf("accept: exit %d %s, want the sealed ACCEPT", got.exit, got.line)
	}
	if len(want.Refusals) == 0 {
		return fmt.Errorf("EXPECTED.json has no refusals")
	}
	for _, c := range want.Refusals {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		var criterion any
		if err := json.Unmarshal(c.Criterion, &criterion); err != nil {
			return err
		}
		bindings, _ := m["criterionBindings"].([]any)
		links, _ := m["criterionLinks"].([]any)
		if len(bindings) != 1 || len(links) != 1 {
			return fmt.Errorf("%s: the map has %d bindings and %d links, want one each", c.Name, len(bindings), len(links))
		}
		bindings[0] = criterion
		if c.RebindLink {
			id, _ := criterion.(map[string]any)["id"]
			links[0].(map[string]any)["criterionId"] = id
		}
		mutated, err := json.Marshal(m)
		if err != nil {
			return err
		}
		got, err := r.verify(repo, mutated, want.BaseRevision, want.TargetRevision, artifacts, "")
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		if code, _ := got.result["code"].(string); got.exit != c.Exit || got.result["outcome"] != c.Outcome || got.result["stage"] != c.Stage || code != c.Code {
			return fmt.Errorf("%s: exit %d %s, want exit %d %s %s %s", c.Name, got.exit, got.line, c.Exit, c.Outcome, c.Stage, c.Code)
		}
	}
	return nil
}

func (r *runner) receiptAuthentication(dir string) error {
	type mutation struct {
		Name           string                     `json:"name"`
		Set            map[string]json.RawMessage `json:"set"`
		TrustedKeys    string                     `json:"trustedKeys"`
		Exit           int                        `json:"exit"`
		Outcome        string                     `json:"outcome"`
		Stage          string                     `json:"stage"`
		Code           string                     `json:"code"`
		IssueCode      string                     `json:"issueCode"`
		Authentication string                     `json:"authentication"`
	}
	var want struct {
		ArtifactRoot   string `json:"artifactRoot"`
		Map            string `json:"map"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			TrustedKeys string         `json:"trustedKeys"`
			Exit        int            `json:"exit"`
			Result      map[string]any `json:"resultExceptRuntime"`
		} `json:"accept"`
		AxisCases []mutation `json:"axisCases"`
		Refusals  []mutation `json:"refusals"`
	}
	if err := readJSON(filepath.Join(dir, "EXPECTED.json"), &want); err != nil {
		return err
	}
	repo, err := r.caseRepository(dir, want.BaseRevision, want.TargetRevision)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(dir, want.Map))
	if err != nil {
		return err
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	keys := func(name string) string {
		if name == "" {
			return ""
		}
		return filepath.Join(dir, name)
	}
	got, err := r.verify(repo, raw, want.BaseRevision, want.TargetRevision, artifacts, keys(want.Accept.TrustedKeys))
	if err != nil {
		return fmt.Errorf("accept: %v", err)
	}
	if got.exit != want.Accept.Exit || !reflect.DeepEqual(resultExceptRuntime(got.result), want.Accept.Result) {
		return fmt.Errorf("accept: exit %d %s, want the sealed ACCEPT", got.exit, got.line)
	}
	cases := append(append([]mutation(nil), want.AxisCases...), want.Refusals...)
	if len(want.AxisCases) == 0 || len(want.Refusals) == 0 {
		return fmt.Errorf("EXPECTED.json has %d axis cases and %d refusals", len(want.AxisCases), len(want.Refusals))
	}
	for _, c := range cases {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		for k, v := range c.Set {
			m[k] = v
		}
		mutated, err := json.Marshal(m)
		if err != nil {
			return err
		}
		got, err := r.verify(repo, mutated, want.BaseRevision, want.TargetRevision, artifacts, keys(c.TrustedKeys))
		if err != nil {
			return fmt.Errorf("%s: %v", c.Name, err)
		}
		axes, _ := got.result["axes"].(map[string]any)
		code, _ := got.result["code"].(string)
		issue := ""
		if issues, _ := got.result["issueCodes"].([]any); len(issues) > 0 {
			issue, _ = issues[0].(string)
		}
		if got.exit != c.Exit || got.result["outcome"] != c.Outcome || axes["authentication"] != c.Authentication || code != c.Code || issue != c.IssueCode || c.Stage != "" && got.result["stage"] != c.Stage {
			return fmt.Errorf("%s: exit %d %s, want exit %d %s code %q issue %q authentication %s", c.Name, got.exit, got.line, c.Exit, c.Outcome, c.Code, c.IssueCode, c.Authentication)
		}
	}
	return nil
}
