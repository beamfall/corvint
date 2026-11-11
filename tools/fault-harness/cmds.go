// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
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
