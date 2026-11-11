// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	preregProfile     = "fault-harness-preregistration/1"
	ruleProfile       = "fault-harness-revert-rule/1"
	candidatesProfile = "fault-harness-revert-candidates/1"
	// minimumN is the N below which a runner's verdict is INVALID (FIH-V1-014).
	minimumN = 100
)

var (
	hexSHA1   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// preregistration is the file of FIH-V1-001, frozen and hashed before any
// execution.
type preregistration struct {
	Profile  string `json:"profile"`
	Runner   string `json:"runner"`
	Language string `json:"language"`
	// Repository is the pinned repository; PinDate is its commit date, the end
	// of the reverted-fix frame.
	Repository struct {
		Name    string `json:"name"`
		SHA     string `json:"sha"`
		PinDate string `json:"pinDate"`
	} `json:"repository"`
	// SamplingFrame states which PRs are sampled for changed-line mutants.
	SamplingFrame string `json:"samplingFrame"`
	// RevertRule pins the reverted-fix rule file of FIH-V1-002 (A17); it is
	// the revert selection query.
	RevertRule struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"revertRule"`
	// Operators is the operator list per language.
	Operators map[string][]string `json:"operators"`
	// Seeds: "mutants" seeds the per-PR mutant pick.
	Seeds map[string]int64 `json:"seeds"`
	// OverSample is the number of code PRs sealed so N can reach 100.
	OverSample int `json:"overSample"`
	// ExclusionBudget is the number of cases that may be excluded with
	// pin-unavailable before the verdict is INVALID.
	ExclusionBudget int      `json:"exclusionBudget"`
	ExclusionRules  []string `json:"exclusionRules"`
	// FlowManifestSHA256 is the frozen flow manifest of a Playwright set
	// (FIH-V1-002); it is required for the playwright runner only.
	FlowManifestSHA256 string `json:"flowManifestSha256,omitempty"`
	// HeldOut marks a drawn held-out repository (FIH-V1-012); DrawRecord pins
	// the committed held-out draw record (FIH-V1-001).
	HeldOut    bool `json:"heldOut,omitempty"`
	DrawRecord *struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"drawRecord,omitempty"`
}

// loadedPreregistration is a pre-registration whose hash matched.
type loadedPreregistration struct {
	preregistration
	SHA256 string
}

// readHashFile reads a committed hash file: a SHA-256 hex digest, optionally
// followed by whitespace and a file name, as sha256sum writes it.
func readHashFile(path string) (string, error) {
	body, err := boundedRead(path, 4096)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 || !hexSHA256.MatchString(fields[0]) {
		return "", fmt.Errorf("%s holds no SHA-256 digest", path)
	}
	return fields[0], nil
}

// loadPreregistration refuses to go on when the file's SHA-256 differs from
// the committed hash (FIH-V1-001), then validates the content.
func loadPreregistration(path, hashPath string) (loadedPreregistration, error) {
	var lp loadedPreregistration
	body, err := boundedRead(path, 1<<20)
	if err != nil {
		return lp, err
	}
	if hashPath == "" {
		return lp, refuse("preregistration-unhashed", "no committed hash file names %s", path)
	}
	want, err := readHashFile(hashPath)
	if err != nil {
		return lp, refuse("preregistration-unhashed", "%v", err)
	}
	lp.SHA256 = sha(body)
	if lp.SHA256 != want {
		return lp, refuse("preregistration-changed", "%s has SHA-256 %s, the committed hash is %s", path, lp.SHA256, want)
	}
	if err := decodeStrict(body, &lp.preregistration); err != nil {
		return lp, refuse("preregistration-invalid", "%v", err)
	}
	return lp, lp.validate()
}

func (p preregistration) validate() error {
	bad := func(format string, args ...any) error { return refuse("preregistration-invalid", format, args...) }
	if p.Profile != preregProfile {
		return bad("profile %q, want %s", p.Profile, preregProfile)
	}
	if !slices.Contains(runners, p.Runner) {
		return bad("runner %q is not one of %v", p.Runner, runners)
	}
	if p.Repository.Name == "" || !hexSHA1.MatchString(p.Repository.SHA) {
		return bad("repository needs a name and a full commit SHA")
	}
	if _, err := time.Parse(time.DateOnly, p.Repository.PinDate); err != nil {
		return bad("repository pinDate: %v", err)
	}
	if p.SamplingFrame == "" || p.Language == "" {
		return bad("sampling frame and language are required")
	}
	if p.RevertRule.Path == "" || !hexSHA256.MatchString(p.RevertRule.SHA256) {
		return bad("revertRule needs a path and a SHA-256")
	}
	ops := p.Operators[p.Language]
	if len(ops) == 0 {
		return bad("no operator list for language %q", p.Language)
	}
	for lang, list := range p.Operators {
		seen := map[string]bool{}
		for _, op := range list {
			if !slices.Contains(operatorFamilies, op) || seen[op] {
				return bad("%s: unknown or repeated operator %q (families: %v)", lang, op, operatorFamilies)
			}
			seen[op] = true
		}
	}
	if _, ok := p.Seeds["mutants"]; !ok {
		return bad("seeds need a \"mutants\" seed")
	}
	if p.OverSample < minimumN {
		return bad("overSample %d cannot reach N = %d", p.OverSample, minimumN)
	}
	if p.ExclusionBudget < 0 || p.ExclusionRules == nil {
		return bad("exclusion budget and rules are required")
	}
	if p.Runner == "playwright" && !hexSHA256.MatchString(p.FlowManifestSHA256) {
		return bad("a playwright set needs the frozen flow manifest SHA-256 (FIH-V1-002)")
	}
	if p.HeldOut {
		return p.checkDraw()
	}
	return nil
}

// checkDraw refuses a held-out pre-registration while no committed held-out
// draw record exists. The draw is seeded by the first candidate tag, so until
// then every held-out request is refused (FIH-V1-001, FIH-V1-012).
func (p preregistration) checkDraw() error {
	if p.DrawRecord == nil || p.DrawRecord.Path == "" {
		return refuse("held-out-not-drawn", "no committed held-out draw record (FIH-V1-001)")
	}
	body, err := boundedRead(p.DrawRecord.Path, 1<<20)
	if err != nil {
		return refuse("held-out-not-drawn", "draw record: %v", err)
	}
	if sha(body) != p.DrawRecord.SHA256 {
		return refuse("held-out-not-drawn", "draw record %s does not match its pinned SHA-256", p.DrawRecord.Path)
	}
	return nil
}

// revertRule is the one deterministic reverted-fix rule file (A17). Its
// numbers are the decided ones; another value is refused, not obeyed.
type revertRule struct {
	Profile string `json:"profile"`
	// Statement is the rule in words, hashed with its numbers.
	Statement           string   `json:"statement"`
	FrameMonths         int      `json:"frameMonths"`         // 12
	ExtendedFrameMonths int      `json:"extendedFrameMonths"` // 24
	Take                int      `json:"take"`                // 30
	MaxPerSourceFile    int      `json:"maxPerSourceFile"`    // 2
	ConfirmRuns         int      `json:"confirmRuns"`         // 3
	FixTitlePrefixes    []string `json:"fixTitlePrefixes"`
	BugLabels           []string `json:"bugLabels"`
}

// loadRule reads the rule file and refuses when its SHA-256 is not the one the
// pre-registration holds.
func loadRule(path, want string) (revertRule, string, error) {
	var r revertRule
	body, err := boundedRead(path, 1<<20)
	if err != nil {
		return r, "", err
	}
	got := sha(body)
	if got != want {
		return r, got, refuse("revert-rule-changed", "%s has SHA-256 %s, the pre-registration holds %s", path, got, want)
	}
	if err := decodeStrict(body, &r); err != nil {
		return r, got, refuse("revert-rule-invalid", "%v", err)
	}
	if r.Profile != ruleProfile || r.FrameMonths != 12 || r.ExtendedFrameMonths != 24 || r.Take != 30 || r.MaxPerSourceFile != 2 || r.ConfirmRuns != confirmRuns || len(r.FixTitlePrefixes) == 0 || r.BugLabels == nil || r.Statement == "" {
		return r, got, refuse("revert-rule-invalid", "the rule must be %s with the A17 numbers 12, 24, 30, 2 and 3 and name its fix signals", ruleProfile)
	}
	return r, got, nil
}

// candidatePR holds the observed facts the rule reads about one merged PR.
type candidatePR struct {
	Number         int      `json:"number"`
	MergedAt       string   `json:"mergedAt"` // YYYY-MM-DD
	Title          string   `json:"title"`
	Labels         []string `json:"labels"`
	ClosesBug      bool     `json:"closesBug"`
	SourceFiles    []string `json:"sourceFiles"`
	TestFiles      []string `json:"testFiles"`
	RevertApplies  bool     `json:"revertApplies"`
	RevertBuilds   bool     `json:"revertBuilds"`
	RevertFailRuns int      `json:"revertFailRuns"` // full-suite runs of the revert with a failing test
}

type candidates struct {
	Profile string        `json:"profile"`
	PRs     []candidatePR `json:"prs"`
}

// revertSelection is the rule's result, reported with any shortfall.
type revertSelection struct {
	RuleSHA256  string `json:"ruleSha256"`
	FrameMonths int    `json:"frameMonths"`
	Qualifying  int    `json:"qualifying"`
	Reverts     []int  `json:"reverts"`
	// Shortfall is the number of reverts below 30, filled with changed-line
	// mutant PRs (FIH-V1-002).
	Shortfall int `json:"shortfall"`
}

func (r revertRule) isFix(c candidatePR) bool {
	title := strings.ToLower(strings.TrimSpace(c.Title))
	for _, p := range r.FixTitlePrefixes {
		p = strings.ToLower(p)
		if rest, ok := strings.CutPrefix(title, p); ok && (rest == "" || !isLetter(rest[0])) {
			return true
		}
	}
	for _, l := range c.Labels {
		if slices.ContainsFunc(r.BugLabels, func(b string) bool { return strings.EqualFold(b, l) }) {
			return true
		}
	}
	return c.ClosesBug
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

func (r revertRule) qualifies(c candidatePR, from, to time.Time) bool {
	merged, err := time.Parse(time.DateOnly, c.MergedAt)
	if err != nil || !merged.After(from) || merged.After(to) {
		return false
	}
	return r.isFix(c) && len(c.SourceFiles) > 0 && len(c.TestFiles) > 0 && c.RevertApplies && c.RevertBuilds && c.RevertFailRuns >= r.ConfirmRuns
}

// selectReverts applies the rule: the qualifying PRs of the 12 months before
// the pin (24 when fewer than 30 qualify), ordered by SHA-256 of (rule-file
// hash, PR number), the first 30 with at most two per source file.
func selectReverts(r revertRule, ruleSHA string, pinDate string, prs []candidatePR) (revertSelection, error) {
	pin, err := time.Parse(time.DateOnly, pinDate)
	if err != nil {
		return revertSelection{}, err
	}
	seen := map[int]bool{}
	for _, c := range prs {
		if c.Number <= 0 || seen[c.Number] {
			return revertSelection{}, fmt.Errorf("candidate PR number %d is invalid or repeated", c.Number)
		}
		seen[c.Number] = true
	}
	frame := func(months int) []candidatePR {
		var q []candidatePR
		for _, c := range prs {
			if r.qualifies(c, pin.AddDate(0, -months, 0), pin) {
				q = append(q, c)
			}
		}
		return q
	}
	sel := revertSelection{RuleSHA256: ruleSHA, FrameMonths: r.FrameMonths, Reverts: []int{}}
	q := frame(r.FrameMonths)
	if len(q) < r.Take {
		sel.FrameMonths = r.ExtendedFrameMonths
		q = frame(r.ExtendedFrameMonths)
	}
	sel.Qualifying = len(q)
	key := func(n int) string {
		h := sha256.Sum256([]byte(ruleSHA + "\n" + strconv.Itoa(n)))
		return hex.EncodeToString(h[:])
	}
	slices.SortFunc(q, func(a, b candidatePR) int { return strings.Compare(key(a.Number), key(b.Number)) })
	perFile := map[string]int{}
	for _, c := range q {
		if len(sel.Reverts) == r.Take {
			break
		}
		if slices.ContainsFunc(c.SourceFiles, func(f string) bool { return perFile[f] >= r.MaxPerSourceFile }) {
			continue
		}
		for _, f := range c.SourceFiles {
			perFile[f]++
		}
		sel.Reverts = append(sel.Reverts, c.Number)
	}
	sel.Shortfall = r.Take - len(sel.Reverts)
	return sel, nil
}

func loadCandidates(path string) ([]candidatePR, error) {
	body, err := boundedRead(path, 16<<20)
	if err != nil {
		return nil, err
	}
	var c candidates
	if err := decodeStrict(body, &c); err != nil {
		return nil, err
	}
	if c.Profile != candidatesProfile {
		return nil, fmt.Errorf("candidates profile %q, want %s", c.Profile, candidatesProfile)
	}
	return c.PRs, nil
}
