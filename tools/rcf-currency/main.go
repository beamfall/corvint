// SPDX-License-Identifier: Apache-2.0
// rcf-currency enforces RCF-V1-010/013 at a dispatched release-candidate gate.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivebuild"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const (
	pinPath              = "docs/specs/rcf-qualification.json"
	selectionPath        = ".corvint/test-selection.json"
	qualificationTest    = "TestRCFV1008Qualification"
	qualificationDir     = "cmd/corvint-docs"
	qualificationPackage = "./" + qualificationDir
	qualificationSource  = qualificationDir + "/rcf_qualification_test.go"
	queryPath            = "cmd/corvint-docs/testdata/rcf-v1-qualification/queries.json"
	rankingTest          = "TestDCSV1404SearchRankingEvalFixture"
	maxInput             = 8 << 20
)

type qualificationPin struct {
	Commit       string `json:"commit"`
	ReportSHA256 string `json:"report_sha256"`
}

type gate struct {
	root    string
	runTest func(pkg, name string) error
}

func main() {
	candidate := flag.String("candidate", "", "required full candidate commit SHA")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: go run ./tools/rcf-currency --candidate FULL_SHA\nRun from a clean candidate checkout. Changed triggers require CORVINT_RCF_QUALIFY\npointing to the existing live-run config, with a new absolute report path outside\nthe checkout. Example: go run ./tools/rcf-currency --candidate $(git rev-parse HEAD)")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}
	g := gate{root: "."}
	result, err := g.check(*candidate, os.Getenv("CORVINT_RCF_QUALIFY"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL RCF-V1-010:", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func hexID(s string, n int) bool {
	return len(s) == n && strings.Trim(s, "0123456789abcdef") == ""
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = errors.New("input exceeds byte limit")
	}
	return b, err
}

func readFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readBounded(f, maxInput)
}

func (g gate) git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.root
	cmd.Env = append(archivebuild.ReleaseEnvironment(runtime.GOOS, runtime.GOARCH), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_NO_REPLACE_OBJECTS=1")
	return commandOutput(cmd, "git "+strings.Join(args, " "))
}

// checkout reads only committed files, including when the caller has ignored build inputs.
func (g gate) checkout(commit, destination string) error {
	root, err := filepath.Abs(g.root)
	if err != nil {
		return err
	}
	if _, err := g.git("clone", "--quiet", "--shared", "--no-checkout", root, destination); err != nil {
		return err
	}
	_, err = (gate{root: destination}).git("checkout", "--quiet", "--detach", commit)
	return err
}

func commandOutput(cmd *exec.Cmd, label string) ([]byte, error) {
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	b, readErr := readBounded(out, maxInput)
	if readErr != nil {
		_ = cmd.Process.Kill()
	}
	err = cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", label, err, stderr.String())
	}
	return b, nil
}

func (g gate) cleanCandidate(candidate string) error {
	head, err := g.git("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != candidate {
		return errors.New("checkout does not name candidate commit")
	}
	status, err := g.git("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return errors.New("candidate checkout is dirty")
	}
	return nil
}

// Use the production owner matcher, not a second trigger list or glob dialect.
func triggerPatterns(raw []byte) ([]string, error) {
	// Preserve this gate's profile diagnostic; the shared parser below validates
	// the complete declaration, including every member omitted by this preflight.
	var header struct {
		Profile string `json:"profile"`
	}
	if len(raw) > 64<<10 {
		return nil, errors.New("selection exceeds 64 KiB")
	}
	if err := jsonv2.Unmarshal(raw, &header); err != nil {
		return nil, err
	}
	if header.Profile != affected.DeclaredTreeReaderSelectionProfile {
		return nil, errors.New("unsupported selection profile")
	}
	selection, err := affected.ParseTestSelection(raw)
	if err != nil {
		return nil, err
	}
	if selection == nil {
		return nil, errors.New("missing qualification trigger owners")
	}
	var patterns []string
	for _, owner := range selection.Owners {
		if !slices.Contains(owner.Tests, qualificationSource) {
			continue
		}
		patterns = append(patterns, owner.Paths)
	}
	if len(patterns) == 0 {
		return nil, errors.New("missing qualification trigger owners")
	}
	return patterns, nil
}

func (g gate) check(candidate, config string) (string, error) {
	if !hexID(candidate, 40) {
		return "", errors.New("candidate must be a full lowercase commit SHA")
	}
	if err := g.cleanCandidate(candidate); err != nil {
		return "", err
	}
	pinBytes, err := g.git("show", candidate+":"+pinPath)
	if err != nil {
		return "", fmt.Errorf("missing qualification pin: %w", err)
	}
	var pin qualificationPin
	if err := json.Unmarshal(pinBytes, &pin); err != nil {
		return "", err
	}
	if !hexID(pin.Commit, 40) || !hexID(pin.ReportSHA256, 64) {
		return "", errors.New("invalid qualification pin")
	}
	if _, err := g.git("cat-file", "-e", pin.Commit+"^{commit}"); err != nil {
		return "", fmt.Errorf("missing qualified commit: %w", err)
	}
	var patterns []string
	var selections [][]byte
	for _, revision := range []string{pin.Commit, candidate} {
		raw, err := g.git("show", revision+":"+selectionPath)
		if err != nil {
			return "", fmt.Errorf("missing selection at %s: %w", revision, err)
		}
		ps, err := triggerPatterns(raw)
		if err != nil {
			return "", fmt.Errorf("selection at %s: %w", revision, err)
		}
		patterns = append(patterns, ps...)
		selections = append(selections, raw)
	}
	// --no-renames exposes both old and new names, including deletions.
	diff, err := g.git("diff", "--name-only", "-z", "--no-renames", pin.Commit, candidate, "--")
	if err != nil {
		return "", err
	}
	changed := !bytes.Equal(selections[0], selections[1])
	for _, name := range strings.Split(string(diff), "\x00") {
		changed = changed || qualificationOwns(patterns, name)
	}
	rawCatalog, err := g.git("show", candidate+":"+releaseCatalog)
	if err != nil {
		return "", err
	}
	build, err := qualificationBuild(rawCatalog)
	if err != nil {
		return "", fmt.Errorf("%s: %w", releaseCatalog, err)
	}
	scratch, err := os.MkdirTemp("", "rcf-currency-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	checkout := gate{root: filepath.Join(scratch, "candidate")}
	if err := g.checkout(candidate, checkout.root); err != nil {
		return "", err
	}
	if !changed && g.productionCovered(build, patterns, pin.Commit, candidate, scratch, checkout.root) {
		if err := g.cleanCandidate(candidate); err != nil {
			return "", err
		}
		return fmt.Sprintf("PASS candidate=%s qualified=%s report_sha256=%s reason=no-trigger-diff", candidate, pin.Commit, pin.ReportSHA256), nil
	}
	if config == "" {
		return "", errors.New("trigger diff requires CORVINT_RCF_QUALIFY configuration")
	}
	if !filepath.IsAbs(config) {
		return "", errors.New("CORVINT_RCF_QUALIFY must name an absolute configuration path")
	}
	raw, err := readFile(config)
	if err != nil {
		return "", fmt.Errorf("qualification configuration: %w", err)
	}
	var cfg struct {
		Report string `json:"report"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", err
	}
	root, err := filepath.Abs(g.root)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(cfg.Report) {
		return "", errors.New("configuration requires an absolute report path")
	}
	// Resolve the existing parent so a symlink cannot place the report in the checkout.
	parent, err := filepath.EvalSymlinks(filepath.Dir(cfg.Report))
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, filepath.Join(parent, filepath.Base(cfg.Report)))
	if err != nil {
		return "", err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("report must be outside candidate checkout")
	}
	if _, err := os.Lstat(cfg.Report); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stale report: destination must not exist before live run: %s", cfg.Report)
	}
	runTest := g.runTest
	if runTest == nil {
		runTest = func(pkg, name string) error { return checkout.liveTest(pkg, name, config) }
	}
	if err := runTest(build.pkg, qualificationTest); err != nil {
		return "", err
	}
	report, err := readFile(cfg.Report)
	if err != nil {
		return "", fmt.Errorf("missing qualification report: %w", err)
	}
	queries, err := g.git("show", candidate+":"+queryPath)
	if err != nil {
		return "", err
	}
	if err := validateReport(report, queries); err != nil {
		return "", err
	}
	for range 2 {
		if err := runTest("./internal/mcp/corpusbridge", rankingTest); err != nil {
			return "", err
		}
	}
	if err := checkout.cleanCandidate(candidate); err != nil {
		return "", err
	}
	if err := g.cleanCandidate(candidate); err != nil {
		return "", err
	}
	return fmt.Sprintf("PASS candidate=%s report_sha256=%x reason=current-live-qualification ranking_runs=2", candidate, sha256.Sum256(report)), nil
}

func validateReport(raw, queries []byte) error {
	var set struct {
		Repositories []struct {
			Name, Commit, Evidence string
			Queries                []json.RawMessage
		}
	}
	var report struct {
		Schema         string `json:"schema"`
		QuerySetSHA256 string `json:"query_set_sha256"`
		Verdict        string `json:"verdict"`
		Repositories   []struct {
			Name, Commit, Evidence, Verdict string
			Queries, Unverified             int
			StatusBefore                    string `json:"status_before_sha256"`
			StatusAfter                     string `json:"status_after_sha256"`
			Failures                        []json.RawMessage
			Records                         []json.RawMessage
		}
	}
	if err := json.Unmarshal(queries, &set); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("invalid qualification report: %w", err)
	}
	if report.Schema != "corvint-rcf-qualification-report/1" || report.Verdict != "PASS" {
		return errors.New("qualification report is not PASS")
	}
	if report.QuerySetSHA256 != fmt.Sprintf("%x", sha256.Sum256(queries)) {
		return errors.New("stale report: query-set digest mismatch")
	}
	if len(set.Repositories) != 3 || len(report.Repositories) != 3 {
		return errors.New("incomplete qualification report")
	}
	for i, expected := range set.Repositories {
		r := report.Repositories[i]
		if r.Name != expected.Name || r.Commit != expected.Commit || r.Evidence != expected.Evidence || r.Verdict != "PASS" || r.Queries != 25 || len(expected.Queries) != 25 || len(r.Records) != 25 || r.Unverified != 0 || len(r.Failures) != 0 || !hexID(r.StatusBefore, 64) || r.StatusBefore != r.StatusAfter {
			return fmt.Errorf("incomplete or failing qualification repository %s", expected.Name)
		}
	}
	return nil
}

// A zero go-test exit is insufficient: skipped and unselected tests also exit zero.
func namedTestPass(r io.Reader, pkg, name string) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64<<10), maxInput)
	runs, passes, packagePasses := 0, 0, 0
	var failure error
	for s.Scan() {
		var event struct{ Action, Package, Test string }
		if err := json.Unmarshal(s.Bytes(), &event); err != nil {
			failure = fmt.Errorf("invalid go test event: %w", err)
			continue
		}
		if event.Package != "github.com/Beamfall/corvint/"+strings.TrimPrefix(pkg, "./") {
			continue
		}
		if event.Action == "fail" || event.Action == "skip" {
			failure = fmt.Errorf("%s %s: %s", event.Package, event.Test, event.Action)
		}
		if event.Test == name && event.Action == "run" {
			runs++
		}
		if event.Test == name && event.Action == "pass" {
			passes++
		}
		if event.Test == "" && event.Action == "pass" {
			packagePasses++
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if runs != 1 || passes != 1 || packagePasses != 1 {
		return fmt.Errorf("%s: missing named live PASS", name)
	}
	return nil
}

func (g gate) liveTest(pkg, name, config string) error {
	cmd := exec.Command("bash", "script/heavy.sh", "go", "test", archivebuild.ReleaseTrimpath, "-json", "-count=1", "-timeout", "30m", "-run", "^"+name+"$", pkg)
	cmd.Dir = g.root
	// Qualification configuration, private corpora and the report are host-local.
	cmd.Env = append(archivebuild.ReleaseEnvironment(runtime.GOOS, runtime.GOARCH),
		"CORVINT_RCF_QUALIFY="+config, "HEAVY_REMOTE_HOST="+os.Getenv("HEAVY_REMOTE_HOST"), "HEAVY_REMOTE=0",
		"HEAVY_LOCAL_REASON=qualification requires local configuration, private corpora and report")
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	parseErr := namedTestPass(io.TeeReader(out, os.Stdout), pkg, name)
	if parseErr != nil {
		_ = cmd.Process.Kill()
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return parseErr
}
