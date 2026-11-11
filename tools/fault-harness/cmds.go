// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// refused prints a refusal and returns exit 1.
func refused(w io.Writer, err error) int {
	if refusalCode(err) != "" {
		fmt.Fprintf(w, "refused: %v\n", err)
		return 1
	}
	fmt.Fprintln(w, err)
	return 2
}

func runCheckCommand(_ context.Context, args []string, w io.Writer) int {
	fs := flags("check", "--preregistration FILE --hash FILE", "--preregistration go.prereg.json --hash go.prereg.json.sha256", w)
	prereg := fs.String("preregistration", "", "pre-registration file (FIH-V1-001)")
	hash := fs.String("hash", "", "committed SHA-256 file of the pre-registration")
	if code := parse(fs, args, prereg); code >= 0 {
		return code
	}
	p, err := loadPreregistration(*prereg, *hash)
	if err != nil {
		return refused(w, err)
	}
	fmt.Fprintf(w, "preregistration %s runner=%s repository=%s@%s overSample=%d\n", p.SHA256, p.Runner, p.Repository.Name, p.Repository.SHA, p.OverSample)
	return 0
}

func runSelectRevertsCommand(_ context.Context, args []string, w io.Writer) int {
	fs := flags("select-reverts", "--preregistration FILE --hash FILE --rule FILE --candidates FILE [--out FILE]",
		"--preregistration go.prereg.json --hash go.prereg.json.sha256 --rule revert-rule.json --candidates go.candidates.json", w)
	prereg := fs.String("preregistration", "", "pre-registration file")
	hash := fs.String("hash", "", "committed SHA-256 file of the pre-registration")
	rule := fs.String("rule", "", "reverted-fix rule file whose SHA-256 the pre-registration holds (A17)")
	cands := fs.String("candidates", "", "observed facts about merged PRs ("+candidatesProfile+")")
	out := fs.String("out", "", "write the selection JSON (default stdout)")
	if code := parse(fs, args, prereg, rule, cands); code >= 0 {
		return code
	}
	p, err := loadPreregistration(*prereg, *hash)
	if err != nil {
		return refused(w, err)
	}
	sel, err := selectFromFiles(p, *rule, *cands)
	if err != nil {
		return refused(w, err)
	}
	if err := report(w, *out, sel); err != nil {
		return refused(w, err)
	}
	return 0
}

func selectFromFiles(p loadedPreregistration, rulePath, candidatesPath string) (revertSelection, error) {
	r, ruleSHA, err := loadRule(rulePath, p.RevertRule.SHA256)
	if err != nil {
		return revertSelection{}, err
	}
	prs, err := loadCandidates(candidatesPath)
	if err != nil {
		return revertSelection{}, err
	}
	return selectReverts(r, ruleSHA, p.Repository.PinDate, prs)
}

func runGenerateGoCommand(ctx context.Context, args []string, w io.Writer) int {
	fs := flags("generate-go", "--preregistration FILE --hash FILE --root DIR --pr N --base SHA --head SHA [--fills-shortfall]",
		"--preregistration go.prereg.json --hash go.prereg.json.sha256 --root ../repo --pr 1234 --base BASE_SHA --head HEAD_SHA", w)
	prereg := fs.String("preregistration", "", "pre-registration file")
	hash := fs.String("hash", "", "committed SHA-256 file of the pre-registration")
	root := fs.String("root", "", "repository holding both commits (read only)")
	pr := fs.String("pr", "", "PR number")
	base := fs.String("base", "", "the PR's base commit")
	head := fs.String("head", "", "the PR's merged commit")
	fills := fs.Bool("fills-shortfall", false, "mark the case as filling the reverted-fix shortfall")
	if code := parse(fs, args, prereg, root, pr, base, head); code >= 0 {
		return code
	}
	p, err := loadPreregistration(*prereg, *hash)
	if err != nil {
		return refused(w, err)
	}
	if p.Language != "go" {
		return refused(w, refuse("runner-adapter-missing", "generate-go cannot generate %s mutants", p.Language))
	}
	n, err := strconv.Atoi(*pr)
	if err != nil || n <= 0 || !hexSHA1.MatchString(*base) || !hexSHA1.MatchString(*head) {
		fs.Usage()
		return 2
	}
	faults, err := generateGoMutants(ctx, *root, *base, *head, n, p.Seeds["mutants"], p.Operators["go"])
	if err != nil {
		return refused(w, err)
	}
	c := harnessCase{ID: fmt.Sprintf("pr%d", n), Kind: "mutants", PR: n, BaseSHA: *base, HeadSHA: *head, FillsShortfall: *fills, Faults: faults}
	if err := report(w, "", c); err != nil {
		return refused(w, err)
	}
	return 0
}

func runSealCommand(_ context.Context, args []string, w io.Writer) int {
	fs := flags("seal", "--preregistration FILE --hash FILE --rule FILE --candidates FILE --set FILE [--out FILE]",
		"--preregistration go.prereg.json --hash go.prereg.json.sha256 --rule revert-rule.json --candidates go.candidates.json --set go.faults.json --out go.seal.json", w)
	prereg := fs.String("preregistration", "", "pre-registration file")
	hash := fs.String("hash", "", "committed SHA-256 file of the pre-registration")
	rule := fs.String("rule", "", "reverted-fix rule file")
	cands := fs.String("candidates", "", "observed facts about merged PRs")
	setPath := fs.String("set", "", "fault set ("+faultSetProfile+")")
	out := fs.String("out", "", "write the seal record (default stdout)")
	if code := parse(fs, args, prereg, rule, cands, setPath); code >= 0 {
		return code
	}
	p, err := loadPreregistration(*prereg, *hash)
	if err != nil {
		return refused(w, err)
	}
	sel, err := selectFromFiles(p, *rule, *cands)
	if err != nil {
		return refused(w, err)
	}
	set, setSHA, err := loadFaultSet(*setPath)
	if err != nil {
		return refused(w, err)
	}
	rec, errs := seal(p, sel, set, setSHA)
	if len(errs) > 0 {
		fmt.Fprint(w, formatRefusals(errs))
		return 1
	}
	if sel.Shortfall > 0 {
		fmt.Fprintf(w, "reverted-fix shortfall: %d qualifying fixes, %d reverts, shortfall %d filled with changed-line mutants\n", sel.Qualifying, len(sel.Reverts), sel.Shortfall)
	}
	if err := report(w, *out, rec); err != nil {
		return refused(w, err)
	}
	return 0
}

func loadFaultSet(path string) (faultSet, string, error) {
	var set faultSet
	body, err := boundedRead(path, 64<<20)
	if err != nil {
		return set, "", err
	}
	if err := decodeStrict(body, &set); err != nil {
		return set, "", refuse("fault-set-invalid", "%v", err)
	}
	return set, sha(body), nil
}

const resultsProfile = "fault-harness-results/1"

// results is what `run` measures and `verdict` reads.
type results struct {
	Profile               string           `json:"profile"`
	Runner                string           `json:"runner"`
	PreregistrationSHA256 string           `json:"preregistrationSha256"`
	FaultSetSHA256        string           `json:"faultSetSha256"`
	ExclusionBudget       int              `json:"exclusionBudget"`
	Baselines             []baselineRecord `json:"baselines"`
	Cases                 []caseResult     `json:"cases"`
}

func runRunCommand(ctx context.Context, args []string, w io.Writer) int {
	fs := flags("run", "--preregistration FILE --hash FILE --set FILE --seal FILE --root DIR --clones DIR [--corvint BIN] [--go-test ARGV] [--unrunnable ID]... [--out FILE]",
		"--preregistration go.prereg.json --hash go.prereg.json.sha256 --set go.faults.json --seal go.seal.json --root ../repo --clones /tmp/fh-clones --out go.results.json", w)
	prereg := fs.String("preregistration", "", "pre-registration file")
	hash := fs.String("hash", "", "committed SHA-256 file of the pre-registration")
	setPath := fs.String("set", "", "sealed fault set")
	sealPath := fs.String("seal", "", "seal record written by `fault-harness seal`")
	root := fs.String("root", "", "pinned source repository (read only)")
	clones := fs.String("clones", "", "existing private clone parent; each case is removed after execution")
	binary := fs.String("corvint", "corvint", "planner binary path or PATH name")
	goTest := fs.String("go-test", "go test", "space-separated argv prefix that runs `go test`")
	timeout := fs.Duration("timeout", 30*time.Minute, "wall bound per adapter command")
	out := fs.String("out", "", "write the results JSON (default stdout)")
	var unrunnable multiFlag
	fs.Var(&unrunnable, "unrunnable", "test ID declared unrunnable in the pinned image (repeatable)")
	if code := parse(fs, args, prereg, setPath, sealPath, root, clones); code >= 0 {
		return code
	}
	// Every refusal below happens before any clone or test run.
	p, err := loadPreregistration(*prereg, *hash)
	if err != nil {
		return refused(w, err)
	}
	set, setSHA, err := loadFaultSet(*setPath)
	if err != nil {
		return refused(w, err)
	}
	sealBody, err := boundedRead(*sealPath, 1<<20)
	if err != nil {
		return refused(w, err)
	}
	var rec sealRecord
	if err := decodeStrict(sealBody, &rec); err != nil {
		return refused(w, refuse("fault-set-invalid", "seal record: %v", err))
	}
	if err := checkSealed(p, set, setSHA, rec); err != nil {
		return refused(w, err)
	}
	adapter, err := adapterFor(p.Runner, adapterOptions{GoTest: strings.Fields(*goTest), Timeout: *timeout})
	if err != nil {
		return refused(w, err)
	}
	cfg, err := resolveRun(*root, *clones, *binary)
	if err != nil {
		fmt.Fprintln(w, err)
		return 2
	}
	cfg.Adapter, cfg.Unrunnable = adapter, unrunnable
	h := newHarness(cfg)
	res := results{Profile: resultsProfile, Runner: p.Runner, PreregistrationSHA256: p.SHA256, FaultSetSHA256: setSHA, ExclusionBudget: p.ExclusionBudget, Cases: []caseResult{}}
	code := 0
	for _, c := range set.Cases {
		r := caseResult{ID: c.ID, Kind: c.Kind, PR: c.PR, HeadSHA: c.HeadSHA, Status: caseError, Error: "not run", Faults: []faultResult{}}
		if ctx.Err() == nil {
			r = h.runCase(ctx, c)
		}
		if r.Status == caseError {
			code = 1
		}
		if *out != "" {
			fmt.Fprintf(w, "%s %s faults=%d\n", r.ID, r.Status, len(r.Faults))
		}
		res.Cases = append(res.Cases, r)
	}
	res.Baselines = h.baselineRecords()
	if err := report(w, *out, res); err != nil {
		fmt.Fprintln(w, err)
		return 2
	}
	return code
}
