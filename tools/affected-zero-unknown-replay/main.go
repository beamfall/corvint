// SPDX-License-Identifier: Apache-2.0

// The replay reads exact historical trees with a caller-selected implementation.
// It never installs declarations from a different revision into those trees.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type unknown struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}
type selection struct {
	UnitID string   `json:"unitId"`
	Tests  []string `json:"tests"`
}
type plan struct {
	Raw                []byte      `json:"-"`
	Selected           []selection `json:"selected"`
	Excluded           []selection `json:"excluded"`
	Unknown            []unknown   `json:"unknown"`
	GoAdditiveExcluded []selection `json:"goAdditiveExcluded"`
}
type envelope struct {
	OK   bool `json:"ok"`
	Plan plan `json:"plan"`
}
type observation struct {
	Unit    string `json:"unit"`
	Test    string `json:"test"`
	Outcome string `json:"outcome"`
}
type suite struct {
	Revision  string        `json:"revision"`
	Inventory []string      `json:"inventory"`
	Complete  bool          `json:"complete"`
	Results   []observation `json:"results"`
}
type mutant struct {
	ID       string        `json:"id"`
	Side     string        `json:"side"`
	Path     string        `json:"path"`
	Line     int           `json:"line"`
	KilledBy []observation `json:"killedBy"`
}
type safetyReceipt struct {
	Merge            string   `json:"merge"`
	Parent           string   `json:"parent"`
	ParentSuite      suite    `json:"parentSuite"`
	MergeSuite       suite    `json:"mergeSuite"`
	MutationComplete bool     `json:"mutationComplete"`
	Mutants          []mutant `json:"mutants"`
}
type safetyResult struct {
	Status string   `json:"status"`
	Digest string   `json:"digest,omitempty"`
	Misses []string `json:"misses"`
	Detail string   `json:"detail,omitempty"`
}
type row struct {
	Merge             string         `json:"merge"`
	Parent            string         `json:"parent"`
	Tree              string         `json:"tree"`
	Unknown           []unknown      `json:"unknown"`
	UnknownByReason   map[string]int `json:"unknownByReason"`
	Selected          int            `json:"selected"`
	Tests             int            `json:"tests"`
	WallMillis        int64          `json:"wallMillis"`
	Safety            safetyResult   `json:"safety"`
	BaselineUnknown   []unknown      `json:"baselineUnknown,omitempty"`
	BaselineSelected  *int           `json:"baselineSelected,omitempty"`
	RemovedUnknown    []unknown      `json:"removedUnknown,omitempty"`
	AddedSelections   []string       `json:"addedSelections,omitempty"`
	DroppedSelections []string       `json:"droppedSelections,omitempty"`
}
type report struct {
	Profile              string `json:"profile"`
	ImplementationSHA256 string `json:"implementationSha256"`
	BaselineSHA256       string `json:"baselineSha256,omitempty"`
	FrozenListSHA256     string `json:"frozenListSha256"`
	Through              string `json:"through"`
	Accepted             bool   `json:"accepted"`
	Merges               []row  `json:"merges"`
}

func digest(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func git(root string, args ...string) (string, error) {
	argv := append([]string{"-c", "core.hooksPath=/dev/null", "-C", root}, args...)
	out, err := exec.Command("git", argv...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
func units(p plan) []string {
	set := map[string]bool{}
	for _, list := range [][]selection{p.Selected, p.Excluded, p.GoAdditiveExcluded} {
		for _, s := range list {
			set[s.UnitID] = true
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
func selected(p plan) map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Selected {
		out[s.UnitID] = true
	}
	return out
}
func readPlan(binary, root, base string) (plan, int64, error) {
	start := time.Now()
	cmd := exec.Command(binary, "--root", root, "affected", "--no-cache", "--full", "--base", base)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return plan{}, elapsed, fmt.Errorf("affected %s: %w: %s", base, err, stderr.String())
	}
	var result envelope
	if err := json.Unmarshal(raw, &result); err != nil {
		return plan{}, elapsed, err
	}
	if !result.OK || result.Plan.Selected == nil || result.Plan.Unknown == nil || result.Plan.Excluded == nil {
		return plan{}, elapsed, errors.New("incomplete affected plan")
	}
	result.Plan.Raw = raw
	return result.Plan, elapsed, nil
}

// diffHeaderPath decodes Git's C-quoted paths. Unquoted spaces belong to
// the path; a literal tab separates the path from an optional header suffix.
func diffHeaderPath(header, prefix string) string {
	name, _, _ := strings.Cut(header, "\t")
	if strings.HasPrefix(name, `"`) {
		decoded, err := strconv.Unquote(name)
		if err != nil {
			return ""
		}
		name = decoded
	}
	if !strings.HasPrefix(name, prefix) {
		return "" // Includes /dev/null for added and deleted files.
	}
	return strings.TrimPrefix(name, prefix)
}

func changedLines(diff string) map[string]map[int]bool {
	lines := map[string]map[int]bool{}
	oldFile, newFile := "", ""
	headers := false
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			oldFile, newFile = "", ""
			headers = true
			continue
		}
		if headers && strings.HasPrefix(line, "--- ") {
			oldFile = diffHeaderPath(strings.TrimPrefix(line, "--- "), "a/")
			continue
		}
		if headers && strings.HasPrefix(line, "+++ ") {
			newFile = diffHeaderPath(strings.TrimPrefix(line, "+++ "), "b/")
			continue
		}
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		headers = false
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		for index, item := range []struct {
			side, file string
			field      string
		}{{"parent", oldFile, fields[1]}, {"merge", newFile, fields[2]}} {
			if item.file == "" {
				continue
			}
			var start, count int
			h := strings.TrimPrefix(item.field, []string{"-", "+"}[index])
			if strings.Contains(h, ",") {
				fmt.Sscanf(h, "%d,%d", &start, &count)
			} else {
				fmt.Sscanf(h, "%d", &start)
				count = 1
			}
			key := item.side + ":" + item.file
			if lines[key] == nil {
				lines[key] = map[int]bool{}
			}
			for n := start; n < start+count; n++ {
				lines[key][n] = true
			}
		}
	}
	return lines
}

func checkSuite(s suite, rev string, inventory []string) error {
	if s.Revision != rev || !s.Complete || !slices.Equal(s.Inventory, inventory) {
		return errors.New("full-suite revision, completeness or inventory mismatch")
	}
	covered := map[string]bool{}
	seen := map[string]bool{}
	for _, r := range s.Results {
		key := r.Unit + "\x00" + r.Test
		if r.Test == "" || !slices.Contains(inventory, r.Unit) || seen[key] || (r.Outcome != "pass" && r.Outcome != "fail" && r.Outcome != "skip") {
			return errors.New("invalid or duplicate full-suite result")
		}
		covered[r.Unit] = true
		seen[key] = true
	}
	for _, id := range inventory {
		if !covered[id] {
			return fmt.Errorf("full-suite result absent for %s", id)
		}
	}
	return nil
}
func safety(body []byte, merge, parent string, before, after plan, lines map[string]map[int]bool) safetyResult {
	result := safetyResult{Status: "NOT_PRODUCED", Misses: []string{}}
	fail := func(err error) safetyResult { result.Detail = err.Error(); return result }
	var receipt safetyReceipt
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return fail(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fail(errors.New("trailing safety receipt data"))
	}
	result.Digest = digest(body)
	if receipt.Merge != merge || receipt.Parent != parent {
		return fail(errors.New("safety receipt revision mismatch"))
	}
	if err := checkSuite(receipt.ParentSuite, parent, units(before)); err != nil {
		return fail(err)
	}
	if err := checkSuite(receipt.MergeSuite, merge, units(after)); err != nil {
		return fail(err)
	}
	if !receipt.MutationComplete || len(receipt.Mutants) == 0 {
		return fail(errors.New("complete changed-line mutation receipt absent"))
	}
	old := map[string]string{}
	for _, r := range receipt.ParentSuite.Results {
		old[r.Unit+"\x00"+r.Test] = r.Outcome
	}
	current := map[string]bool{}
	for _, r := range receipt.MergeSuite.Results {
		current[r.Unit+"\x00"+r.Test] = true
	}
	for key := range old {
		if !current[key] {
			return fail(errors.New("parent test outcome absent from merge full suite"))
		}
	}
	chosen := selected(after)
	// A newly discovered test also differs from the parent's absent outcome.
	for _, r := range receipt.MergeSuite.Results {
		if old[r.Unit+"\x00"+r.Test] != r.Outcome && !chosen[r.Unit] {
			result.Misses = append(result.Misses, "outcome:"+r.Unit+":"+r.Test)
		}
	}
	coordinates := map[string]bool{}
	for _, m := range receipt.Mutants {
		coordinate := fmt.Sprintf("%s:%s:%d", m.Side, m.Path, m.Line)
		if m.ID == "" || (m.Side != "parent" && m.Side != "merge") || !lines[m.Side+":"+m.Path][m.Line] || coordinates[m.ID] {
			return fail(fmt.Errorf("mutant is duplicate or outside changed lines: %s", coordinate))
		}
		coordinates[m.ID] = true
		for _, killer := range m.KilledBy {
			knownKiller := slices.ContainsFunc(receipt.MergeSuite.Results, func(r observation) bool { return r.Unit == killer.Unit && r.Test == killer.Test })
			if killer.Test == "" || !knownKiller || !slices.Contains(units(after), killer.Unit) {
				return fail(errors.New("unknown mutant killer"))
			}
			if !chosen[killer.Unit] {
				result.Misses = append(result.Misses, "mutant:"+coordinate+":"+killer.Unit+":"+killer.Test)
			}
		}
	}
	sort.Strings(result.Misses)
	result.Misses = slices.Compact(result.Misses)
	result.Status = "OBSERVED_NO_MISS"
	if len(result.Misses) > 0 {
		result.Status = "UNSAFE_DROP"
	}
	return result
}
func compare(r *row, before, after plan) {
	n := len(before.Selected)
	r.BaselineSelected = &n
	r.BaselineUnknown = before.Unknown
	for _, u := range before.Unknown {
		if !slices.Contains(after.Unknown, u) {
			r.RemovedUnknown = append(r.RemovedUnknown, u)
		}
	}
	a, b := selected(before), selected(after)
	for id := range b {
		if !a[id] {
			r.AddedSelections = append(r.AddedSelections, id)
		}
	}
	for id := range a {
		if !b[id] {
			r.DroppedSelections = append(r.DroppedSelections, id)
		}
	}
	sort.Strings(r.AddedSelections)
	sort.Strings(r.DroppedSelections)
}

var frozenMergePrefix = []string{
	"8e2c13aa926fcaf1731fae71b27c41c41c8f09af",
	"39c03198086681fb24f509fbd00b34696d3268c4",
	"d436dff76de1b8e75c427069ae7f80003ae99bc2",
	"6c33df33bc5670131482f5263b410cb980580657",
	"ea770fa17992b970aeda570ed74686a16dbc441e",
	"24d8c04f9f5ffaaed86759951e520cb775dc2eff",
	"f13f52b1607222033b089144a744fedae0d35239",
	"9fdc2f02efc11ce9ed582d8529b63fe0478fcad2",
}

func validFrozenPrefix(refs []string) bool {
	return len(refs) >= len(frozenMergePrefix) && slices.Equal(refs[:len(frozenMergePrefix)], frozenMergePrefix)
}

func replayMerges(root string, refs []string, through string) ([]string, string, error) {
	seen := map[string]bool{}
	for _, ref := range refs {
		if seen[ref] {
			return nil, "", fmt.Errorf("duplicate frozen merge %s", ref)
		}
		seen[ref] = true
	}
	tip, err := git(root, "rev-parse", through+"^{commit}")
	if err != nil {
		return nil, "", err
	}
	if _, err := git(root, "merge-base", "--is-ancestor", refs[0], tip); err != nil {
		return nil, "", errors.New("through revision is not a descendant of the frozen anchor")
	}
	later, err := git(root, "rev-list", "--first-parent", "--merges", "--reverse", refs[0]+".."+tip)
	if err != nil {
		return nil, "", err
	}
	merged := slices.Clone(refs)
	for _, ref := range strings.Fields(later) {
		if !seen[ref] {
			merged = append(merged, ref)
			seen[ref] = true
		}
	}
	return merged, tip, nil
}

func run(root, binary, baseline, list, through, out, safetyDir string) error {
	absolute, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	binary = absolute
	implementation, err := os.ReadFile(binary)
	if err != nil {
		return err
	}
	frozen, err := os.ReadFile(list)
	if err != nil {
		return err
	}
	refs := strings.Fields(string(frozen))
	if !validFrozenPrefix(refs) {
		return errors.New("frozen merge list must start with the contractual eight full SHAs")
	}
	refs, tip, err := replayMerges(root, refs, through)
	if err != nil {
		return err
	}
	// Keep every clone and raw artifact outside the repository.
	rootAbs, _ := filepath.Abs(root)
	outAbs, _ := filepath.Abs(out)
	if outAbs == rootAbs || strings.HasPrefix(outAbs, rootAbs+string(os.PathSeparator)) {
		return errors.New("replay output must be outside repository")
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		return err
	}
	resolvedOut, err := filepath.EvalSymlinks(outAbs)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return err
	}
	if resolvedOut == resolvedRoot || strings.HasPrefix(resolvedOut, resolvedRoot+string(os.PathSeparator)) {
		return errors.New("replay output resolves inside repository")
	}
	scratch, err := os.MkdirTemp(out, "snapshots-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	if _, err := git(root, "clone", "--quiet", "--shared", "--no-checkout", rootAbs, scratch); err != nil {
		return err
	}
	output := report{Profile: "affected-zero-unknown-replay/0", ImplementationSHA256: digest(implementation), FrozenListSHA256: digest(frozen), Through: tip, Merges: []row{}}
	if baseline != "" {
		baseline, err = filepath.Abs(baseline)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(baseline)
		if err != nil {
			return err
		}
		output.BaselineSHA256 = digest(body)
	}
	return replayRevisions(root, scratch, binary, baseline, out, safetyDir, refs, output)
}

// replayRevisions keeps intermediate and failed reports unaccepted. Only a
// completed replay of every pinned revision may promote the aggregate result.
func replayRevisions(root, scratch, binary, baseline, out, safetyDir string, refs []string, output report) (replayErr error) {
	writeReport := func() error {
		encoded, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, "report.json"), append(encoded, '\n'), 0600)
	}
	output.Accepted = false
	if err := writeReport(); err != nil {
		return err
	}
	var pending *row
	defer func() {
		if replayErr != nil && pending != nil {
			pending.Safety.Detail = "replay incomplete: " + replayErr.Error()
			output.Merges = append(output.Merges, *pending)
			// Preserve the original replay error even if the report cannot be
			// updated; the previously written report is already unaccepted.
			_ = writeReport()
		}
	}()
	qualified := len(refs) > 0
	seen := map[string]bool{}
	for _, ref := range refs {
		pending = &row{Merge: ref, Unknown: []unknown{}, UnknownByReason: map[string]int{}, Safety: safetyResult{Status: "NOT_PRODUCED", Misses: []string{}}}
		if len(ref) != 40 {
			return fmt.Errorf("frozen merge must be full SHA: %s", ref)
		}
		if seen[ref] {
			return fmt.Errorf("duplicate frozen merge %s", ref)
		}
		seen[ref] = true
		parents, err := git(root, "rev-list", "--parents", "-n", "1", ref)
		if err != nil {
			return err
		}
		fields := strings.Fields(parents)
		if len(fields) < 3 || fields[0] != ref {
			return fmt.Errorf("not a pinned merge: %s", ref)
		}
		parent := fields[1]
		pending.Parent = parent
		if _, err := git(scratch, "checkout", "--quiet", "--detach", "--force", ref); err != nil {
			return err
		}
		p, elapsed, err := readPlan(binary, scratch, parent)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, ref+".candidate.json"), p.Raw, 0600); err != nil {
			return err
		}
		tree, err := git(root, "rev-parse", ref+"^{tree}")
		if err != nil {
			return err
		}
		r := row{Merge: ref, Parent: parent, Tree: tree, Unknown: p.Unknown, UnknownByReason: map[string]int{}, Selected: len(p.Selected), WallMillis: elapsed, Safety: safetyResult{Status: "NOT_PRODUCED", Detail: "full-suite and changed-line mutation receipts unavailable", Misses: []string{}}}
		for _, u := range p.Unknown {
			r.UnknownByReason[u.Reason+":"+u.Detail]++
		}
		for _, s := range p.Selected {
			r.Tests += len(s.Tests)
		}
		pending = &r
		if baseline != "" {
			b, _, err := readPlan(baseline, scratch, parent)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(out, ref+".baseline.json"), b.Raw, 0600); err != nil {
				return err
			}
			compare(&r, b, p)
		}
		if safetyDir != "" {
			body, err := os.ReadFile(filepath.Join(safetyDir, ref+".json"))
			if err != nil {
				return err
			}
			diff, err := git(root, "diff", "--no-ext-diff", "--no-renames", "--unified=0", parent, ref)
			if err != nil {
				return err
			}
			if _, err := git(scratch, "checkout", "--quiet", "--detach", "--force", parent); err != nil {
				return err
			}
			// The inventory depends on the tree, not the dirty range. Planning the
			// parent against itself enumerates selected plus excluded tests.
			b, _, err := readPlan(binary, scratch, parent)
			if err != nil {
				return err
			}
			r.Safety = safety(body, ref, parent, b, p, changedLines(diff))
		}
		if len(r.Unknown) > 0 || r.Safety.Status != "OBSERVED_NO_MISS" {
			qualified = false
		}
		output.Merges = append(output.Merges, r)
		pending = nil
		if err := writeReport(); err != nil {
			return err
		}
		fmt.Printf("merge=%s unknown=%d selected=%d tests=%d wallMs=%d safety=%s\n", ref, len(r.Unknown), r.Selected, r.Tests, r.WallMillis, r.Safety.Status)
	}
	output.Accepted = qualified
	if err := writeReport(); err != nil {
		return err
	}
	if !output.Accepted {
		return errors.New("AZU-V0-009 NOT_ACCEPTED: unknowns or missing/failed safety evidence; see report.json")
	}
	return nil
}
func main() {
	root := flag.String("root", ".", "repository root")
	binary := flag.String("corvint", "", "absolute candidate binary built from the tested implementation (required)")
	baseline := flag.String("baseline", "", "optional frozen baseline binary for unknown/selection deltas")
	list := flag.String("merges", "", "frozen full-SHA merge list (default script/affected-zero-unknown-merges.txt)")
	through := flag.String("through", "origin/main", "include all later first-parent merges through this revision")
	out := flag.String("out", "", "private evidence directory outside repository (required)")
	receipts := flag.String("safety", "", "external full-suite/mutation receipt directory; absent stays NOT_PRODUCED")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: script/affected-zero-unknown-replay.sh --corvint /abs/corvint --out /abs/evidence [--baseline /abs/base] [--safety /abs/receipts]\nReplays each exact frozen merge tree and subsequent first-parent merges. No safety receipt means NOT_ACCEPTED.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *binary == "" || *out == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *list == "" {
		*list = filepath.Join(*root, "script/affected-zero-unknown-merges.txt")
	}
	if err := run(*root, *binary, *baseline, *list, *through, *out, *receipts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
