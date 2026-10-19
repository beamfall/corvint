package testsource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Seed validation, sibling collision and hook blast radius (TSA-V0-240..246; V1-1935 / GitHub
// #1196). The server validation a seed must pass is read from a project-declared, committed seed
// rule file; a seed the rules cannot decide abstains.

// SeedRulesPath is the committed seed rule file `corvint tests validity` reads at the revision.
const SeedRulesPath = ".corvint/seed-rules.json"

// SeedRulesProfile is the closed profile of the seed rule file.
const SeedRulesProfile = "corvint-seed-rules/0"

const (
	maxSeedRulesBytes     = 64 << 10
	maxSeedRuleResources  = 64
	maxSeedRecurrenceDays = 366 * 10
)

// SeedRules is the project's declared server validation for seeded resources (TSA-V0-240).
type SeedRules struct {
	Profile   string         `json:"profile"`
	Resources []SeedResource `json:"resources"`
}

// SeedResource names how a resource is seeded and the validation the server applies to it. Calls
// are function or method names whose last argument is the record object; Paths are request paths a
// `X.post|put(path, { data })` call seeds. Tenant scopes every rule; Unique lists composite keys;
// Range is a start/end pair with inclusive or exclusive bounds; Date is a calendar date that
// Weekly (a boolean field) repeats every seven days until Until (a date field, optional).
type SeedResource struct {
	Name   string     `json:"name"`
	Calls  []string   `json:"calls,omitempty"`
	Paths  []string   `json:"paths,omitempty"`
	Tenant string     `json:"tenant,omitempty"`
	Unique [][]string `json:"unique,omitempty"`
	Range  *SeedRange `json:"range,omitempty"`
	Date   string     `json:"date,omitempty"`
	Weekly string     `json:"weekly,omitempty"`
	Until  string     `json:"until,omitempty"`
}

// SeedRange is a seeded range's fields and whether touching bounds overlap.
type SeedRange struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Bounds string `json:"bounds"`
}

// DecodeSeedRules decodes and validates a seed rule file (TSA-V0-240).
func DecodeSeedRules(data []byte) (SeedRules, error) {
	var rules SeedRules
	if len(data) > maxSeedRulesBytes {
		return rules, fmt.Errorf("over %d bytes", maxSeedRulesBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rules); err != nil {
		return SeedRules{}, err
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return SeedRules{}, fmt.Errorf("trailing data after the rule object")
	}
	if rules.Profile != SeedRulesProfile {
		return SeedRules{}, fmt.Errorf("profile %q is not %s", rules.Profile, SeedRulesProfile)
	}
	if len(rules.Resources) == 0 || len(rules.Resources) > maxSeedRuleResources {
		return SeedRules{}, fmt.Errorf("resources must hold 1..%d entries", maxSeedRuleResources)
	}
	for _, res := range rules.Resources {
		switch {
		case res.Name == "" || len(res.Calls)+len(res.Paths) == 0:
			return SeedRules{}, fmt.Errorf("resource %q names no call or path", res.Name)
		case res.Range != nil && (res.Range.Start == "" || res.Range.End == "" || res.Range.Bounds != "inclusive" && res.Range.Bounds != "exclusive"):
			return SeedRules{}, fmt.Errorf("resource %q range needs start, end and inclusive or exclusive bounds", res.Name)
		case (res.Weekly != "" || res.Until != "") && res.Date == "":
			return SeedRules{}, fmt.Errorf("resource %q recurrence needs a date field", res.Name)
		case len(res.Unique) == 0 && res.Range == nil && res.Date == "":
			return SeedRules{}, fmt.Errorf("resource %q declares no rule", res.Name)
		}
		for _, key := range res.Unique {
			if len(key) == 0 || slices.Contains(key, "") {
				return SeedRules{}, fmt.Errorf("resource %q has an empty unique key", res.Name)
			}
		}
	}
	return rules, nil
}

// seedValue is one literal field value. Two values are equal only when their kinds match; a number's
// text is its canonical form, so 1 and 1.0 are one value.
type seedValue struct {
	kind   string // "number", "string" or "bool"
	text   string
	number float64
	isNum  bool
}

// seed is one seed call the rules matched.
type seed struct {
	res        *SeedResource
	at, line   int
	owner      int // index into r.decls of the test, or -1 for a hook
	hook       int // index into hooks, or -1
	fields     map[string]seedValue
	unresolved string
}

// seedHook is one beforeAll/beforeEach hook body.
type seedHook struct {
	at, line int
	body     [2]int
	scope    int // innermost describe index, or -1 for file level
}

// SeedFindings returns the TSA-V0-241..246 findings of one JS/TS test file under rules.
func SeedFindings(name, text string, rules SeedRules) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	if !IsTestFile(name) || ValidityFindings(name, text).Uncertain {
		return out
	}
	r := &jsReader{s: parseSource(name, text), name: name}
	r.read()
	hooks := r.seedHooks()
	seeds := []seed{}
	for i, h := range hooks {
		seeds = append(seeds, r.seedsIn(h.body, rules, -1, i)...)
	}
	for i, d := range r.decls {
		if d.kind == "test" && d.hasBody {
			seeds = append(seeds, r.seedsIn(d.body, rules, i, -1)...)
		}
	}
	slices.SortStableFunc(seeds, func(a, b seed) int { return a.at - b.at })
	testOf := func(sd seed) string {
		if sd.owner >= 0 {
			return r.at(r.decls[sd.owner].line)
		}
		return r.at(hooks[sd.hook].line)
	}
	for _, sd := range seeds {
		if sd.unresolved != "" {
			out = append(out, testvalidity.SourceFinding{Rule: testvalidity.RuleSeedValidationUnresolved, Severity: testvalidity.SeverityAdvisory,
				Test: testOf(sd), Location: r.at(sd.line), Cause: r.at(sd.line),
				Message: "abstained: the " + sd.res.Name + " seed cannot be checked against the declared validation (" + sd.unresolved + ")"})
			continue
		}
		if why := seedSelfInvalid(sd); why != "" {
			out = append(out, testvalidity.SourceFinding{Rule: testvalidity.RuleSeedValidationRejected, Severity: testvalidity.SeverityAdvisory,
				Test: testOf(sd), Location: r.at(sd.line), Cause: r.at(sd.line),
				Message: "the server's " + sd.res.Name + " validation rejects this seed: " + why})
		}
	}
	// Seeds that run in one setup sequence: a hook alone, and each test with the hooks around it.
	inScope := func(h seedHook, testAt int) bool {
		return h.scope < 0 || within(testAt, r.decls[h.scope].body)
	}
	reported := map[[2]int]bool{}
	check := func(group []seed, rule string) {
		for j := range group {
			for i := 0; i < j; i++ {
				a, b := group[i], group[j]
				if a.at > b.at {
					a, b = b, a
				}
				key := [2]int{a.at, b.at}
				if reported[key] || rule == testvalidity.RuleSeedSiblingCollision && a.owner == b.owner {
					continue
				}
				why := seedConflict(a, b, rule == testvalidity.RuleSeedSiblingCollision)
				if why == "" {
					continue
				}
				reported[key] = true
				message := "the server's " + a.res.Name + " validation rejects this seed after the one at " + r.at(a.line) + ": " + why
				if rule == testvalidity.RuleSeedSiblingCollision {
					message = "this seed collides with the sibling seed at " + r.at(a.line) + " (" + testOf(a) + ") in the same tenant: " + why
				}
				out = append(out, testvalidity.SourceFinding{Rule: rule, Severity: testvalidity.SeverityAdvisory,
					Test: testOf(b), Location: r.at(b.line), Cause: r.at(a.line), Message: message})
			}
		}
	}
	resolved := func(filter func(seed) bool) []seed {
		group := []seed{}
		for _, sd := range seeds {
			if sd.unresolved == "" && filter(sd) {
				group = append(group, sd)
			}
		}
		return group
	}
	for i := range hooks {
		check(resolved(func(sd seed) bool { return sd.hook == i }), testvalidity.RuleSeedValidationRejected)
	}
	for t, d := range r.decls {
		if d.kind != "test" {
			continue
		}
		check(resolved(func(sd seed) bool {
			return sd.owner == t || sd.hook >= 0 && inScope(hooks[sd.hook], d.at)
		}), testvalidity.RuleSeedValidationRejected)
	}
	check(resolved(func(sd seed) bool { return sd.owner >= 0 }), testvalidity.RuleSeedSiblingCollision)
	// Blast radius: every test a seeding hook's failure would skip (TSA-V0-245).
	for i, h := range hooks {
		if !slices.ContainsFunc(seeds, func(sd seed) bool { return sd.hook == i }) {
			continue
		}
		skipped := []string{}
		for _, d := range r.decls {
			if d.kind == "test" && inScope(h, d.at) {
				skipped = append(skipped, r.at(d.line))
			}
		}
		if len(skipped) == 0 {
			continue
		}
		out = append(out, testvalidity.SourceFinding{Rule: testvalidity.RuleSeedHookBlastRadius, Severity: testvalidity.SeverityAdvisory,
			Test: r.at(h.line), Location: r.at(h.line), Cause: skipped[0],
			Message: fmt.Sprintf("a failing seed in this %s hook skips %d test(s): %s", strings.Fields(r.s.tokens[h.at].Text)[0], len(skipped), strings.Join(skipped, ", "))})
	}
	return out
}

// seedHooks finds every `beforeAll`/`beforeEach` (bare or `test.`) hook with a literal body.
func (r *jsReader) seedHooks() []seedHook {
	s := r.s
	out := []seedHook{}
	for k := range s.tokens {
		if !s.ident(k, "beforeAll") && !s.ident(k, "beforeEach") {
			continue
		}
		if s.memberAccess(k) && !s.ident(k-2, "test") || !s.punct(k+1, "(") || s.match[k+1] < 0 || r.inExampleBody(k, -1) {
			continue
		}
		args := s.splitArgs(k+1, s.match[k+1])
		for a := len(args) - 1; a >= 0; a-- {
			if body, _, ok := s.functionBody(args[a][0]); ok && body[1] <= args[a][1] {
				scope := -1
				if chain := r.ancestors(k); len(chain) > 0 {
					scope = chain[0]
				}
				out = append(out, seedHook{at: k, line: s.tokens[k].Line, body: body, scope: scope})
				break
			}
		}
	}
	return out
}

// seedsIn returns the seed calls in body that a resource matches.
func (r *jsReader) seedsIn(body [2]int, rules SeedRules, owner, hook int) []seed {
	s := r.s
	out := []seed{}
	for k := body[0]; k < body[1]; k++ {
		if !s.ident(k, "") || !s.punct(k+1, "(") || s.match[k+1] < 0 {
			continue
		}
		args := s.splitArgs(k+1, s.match[k+1])
		name := s.tokens[k].Text
		for i := range rules.Resources {
			res := &rules.Resources[i]
			object := -1
			switch {
			case slices.Contains(res.Calls, name):
				if len(args) > 0 {
					object = args[len(args)-1][0]
				}
			case s.memberAccess(k) && (name == "post" || name == "put") && len(args) > 0 && s.loneString(args[0]) &&
				slices.Contains(res.Paths, strings.SplitN(s.tokens[args[0][0]].Text, "?", 2)[0]):
				if len(args) > 1 && s.punct(args[1][0], "{") {
					object, _ = r.objectMember(args[1][0], "data")
				}
			default:
				continue
			}
			sd := seed{res: res, at: k, line: s.tokens[k].Line, owner: owner, hook: hook}
			if object < 0 || !s.punct(object, "{") || s.match[object] < 0 {
				sd.unresolved = "the seeded record is not an object literal"
			} else {
				sd.fields, sd.unresolved = r.seedFields(object, res)
			}
			out = append(out, sd)
			break
		}
	}
	return out
}

// objectMember returns the value token index of key in the object literal at open, or -1. A later
// key replaces an earlier one, as at run time. plain is false when any member is a spread, a
// computed or inexact key, a method or a shorthand, since such a member may set key.
func (r *jsReader) objectMember(open int, key string) (at int, plain bool) {
	s := r.s
	at = -1
	for _, el := range s.splitArgs(open, s.match[open]) {
		if el[1] <= el[0] {
			continue
		}
		t := s.tokens[el[0]]
		if el[1]-el[0] < 3 || !s.punct(el[0]+1, ":") || t.Kind != appmap.JSIdent && (t.Kind != appmap.JSString || !t.Literal) {
			return -1, false
		}
		if t.Text == key {
			at = el[0] + 2
		}
	}
	return at, true
}

// seedFields reads every field a rule names as a literal; the first one that is not abstains.
func (r *jsReader) seedFields(open int, res *SeedResource) (map[string]seedValue, string) {
	s := r.s
	needed := []string{}
	if res.Tenant != "" {
		needed = append(needed, res.Tenant)
	}
	for _, key := range res.Unique {
		needed = append(needed, key...)
	}
	if res.Range != nil {
		needed = append(needed, res.Range.Start, res.Range.End)
	}
	if res.Date != "" {
		needed = append(needed, res.Date)
	}
	optional := map[string]bool{res.Weekly: true, res.Until: true}
	if res.Weekly != "" {
		needed = append(needed, res.Weekly)
	}
	if res.Until != "" {
		needed = append(needed, res.Until)
	}
	fields := map[string]seedValue{}
	for _, field := range needed {
		v, plain := r.objectMember(open, field)
		if !plain {
			return nil, "the record has a spread, computed or shorthand member"
		}
		if v < 0 {
			if optional[field] {
				continue
			}
			return nil, "field " + field + " is not set literally"
		}
		value, ok := r.literalValue(v, s.match[open])
		if !ok {
			return nil, "field " + field + " is not a literal"
		}
		fields[field] = value
	}
	return fields, ""
}

// literalValue reads a lone string, number or boolean value ending at the next top-level comma.
func (r *jsReader) literalValue(v, close int) (seedValue, bool) {
	s := r.s
	neg := s.punct(v, "-")
	if neg {
		v++
	}
	if v+1 < close && !s.punct(v+1, ",") || v >= close {
		return seedValue{}, false
	}
	t := s.tokens[v]
	switch {
	case t.Kind == appmap.JSNumber:
		n, err := strconv.ParseFloat(strings.ReplaceAll(t.Text, "_", ""), 64)
		if err != nil {
			return seedValue{}, false
		}
		if neg {
			n = -n
		}
		return seedValue{kind: "number", text: strconv.FormatFloat(n, 'g', -1, 64), number: n, isNum: true}, true
	case neg:
		return seedValue{}, false
	case (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal:
		return seedValue{kind: "string", text: t.Text}, true
	case t.Kind == appmap.JSIdent && (t.Text == "true" || t.Text == "false"):
		return seedValue{kind: "bool", text: t.Text}, true
	}
	return seedValue{}, false
}

// compareSeed orders two values: both numbers, both `YYYY-MM-DD` dates, or both RFC 3339
// timestamps compared as instants. Anything else, a date against a timestamp included, abstains.
func compareSeed(a, b seedValue) (int, bool) {
	if a.isNum && b.isNum {
		switch {
		case a.number < b.number:
			return -1, true
		case a.number > b.number:
			return 1, true
		}
		return 0, true
	}
	if a.kind != "string" || b.kind != "string" || !isoLike(a.text) || !isoLike(b.text) {
		return 0, false
	}
	if len(a.text) == 10 && len(b.text) == 10 {
		return strings.Compare(a.text, b.text), true
	}
	x, errA := time.Parse(time.RFC3339Nano, a.text)
	y, errB := time.Parse(time.RFC3339Nano, b.text)
	if errA != nil || errB != nil {
		return 0, false
	}
	return x.Compare(y), true
}

// isoLike reports a `YYYY-MM-DD` date, optionally followed by a time.
func isoLike(v string) bool {
	if len(v) < 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", v[:10])
	return err == nil
}

// seedSelfInvalid reports a range the server rejects on its own.
func seedSelfInvalid(sd seed) string {
	rg := sd.res.Range
	if rg == nil {
		return ""
	}
	c, ok := compareSeed(sd.fields[rg.Start], sd.fields[rg.End])
	switch {
	case !ok:
		return ""
	case c > 0:
		return rg.Start + " is after " + rg.End
	case c == 0 && rg.Bounds == "exclusive":
		return rg.Start + " equals " + rg.End + " under exclusive bounds, so the range is empty"
	}
	return ""
}

// seedConflict reports why two resolved seeds of one resource and tenant conflict, or "".
func seedConflict(a, b seed, recurrence bool) string {
	if a.res != b.res {
		return ""
	}
	res := a.res
	if res.Tenant != "" && a.fields[res.Tenant] != b.fields[res.Tenant] {
		return ""
	}
	for _, key := range res.Unique {
		same := true
		parts := []string{}
		for _, field := range key {
			same = same && a.fields[field] == b.fields[field]
			parts = append(parts, field+"="+a.fields[field].text)
		}
		if same {
			return "unique key " + strings.Join(parts, ",") + " is seeded twice"
		}
	}
	if rg := res.Range; rg != nil {
		c1, ok1 := compareSeed(a.fields[rg.Start], b.fields[rg.End])
		c2, ok2 := compareSeed(b.fields[rg.Start], a.fields[rg.End])
		if ok1 && ok2 {
			overlap := c1 < 0 && c2 < 0
			if rg.Bounds == "inclusive" {
				overlap = c1 <= 0 && c2 <= 0
			}
			if overlap {
				why := fmt.Sprintf("ranges [%s, %s] and [%s, %s] overlap", a.fields[rg.Start].text, a.fields[rg.End].text, b.fields[rg.Start].text, b.fields[rg.End].text)
				if c1 == 0 || c2 == 0 {
					why += " at a shared bound under inclusive bounds"
				}
				return why
			}
		}
	}
	if res.Date != "" && recurrence {
		if day, ok := seedDateCollision(a, b); ok {
			return "both seed " + res.Date + " " + day
		}
	}
	return ""
}

// seedDateCollision reports a calendar day both seeds occupy, expanding weekly recurrences.
func seedDateCollision(a, b seed) (string, bool) {
	res := a.res
	day := func(sd seed, field string) (time.Time, bool) {
		v, ok := sd.fields[field]
		if !ok || v.kind != "string" || !isoLike(v.text) {
			return time.Time{}, false
		}
		t, _ := time.Parse("2006-01-02", v.text[:10])
		return t, true
	}
	occurrences := func(sd seed) ([]time.Time, bool) {
		start, ok := day(sd, res.Date)
		if !ok {
			return nil, false
		}
		if res.Weekly == "" || sd.fields[res.Weekly] != (seedValue{kind: "bool", text: "true"}) {
			return []time.Time{start}, true
		}
		until, bounded := day(sd, res.Until)
		if !bounded {
			until = start.AddDate(0, 0, maxSeedRecurrenceDays)
		}
		out := []time.Time{}
		for d := start; !d.After(until); d = d.AddDate(0, 0, 7) {
			out = append(out, d)
		}
		return out, true
	}
	as, okA := occurrences(a)
	bs, okB := occurrences(b)
	if !okA || !okB {
		return "", false
	}
	for _, x := range as {
		for _, y := range bs {
			if x.Equal(y) {
				return x.Format("2006-01-02"), true
			}
		}
	}
	return "", false
}
