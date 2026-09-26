package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const profile = "corvint-use-case-conformance/1"
const legacyProfile = "corvint-use-case-conformance/0"
const evidenceProfile = "corvint-use-case-evidence/0"
const resultProfile = "corvint-use-case-conformance-result/0"
const clausesProfile = "corvint-use-case-clauses/0"
const maxFileBytes = 1048576

var statuses = []string{"specified", "experimental", "verified"}
var claims = []string{"UNPROVEN", "VERIFIED"}
var evidenceClasses = []string{"contract", "implementation", "hostile-tests", "corvint-dogfood", "beamfall-dogfood", "sealed-benchmark"}
var useCaseIDs = []string{"UC-AI-CODING", "UC-AI-DEBUGGING", "UC-ENGINEERING-RESEARCH", "UC-PI-LIFECYCLE", "UC-DEEPSEEK-HARNESS-LIFECYCLE", "UC-HUMAN-DOCUMENTATION", "UC-AUTOMATIC-E2E", "UC-PR-MAINTENANCE-MERGE", "UC-CHANGE-BREAKAGE", "UC-MISSING-TESTS", "UC-MINIMUM-TESTS", "UC-ONBOARDING", "UC-TICKET-ROUTING", "UC-CODE-REVIEW", "UC-CODE-TO-SPEC", "UC-REMOVAL-MIGRATION", "UC-EXPLAIN-SHIPPED", "UC-INCIDENT-ORIENTATION", "UC-LIVE-PROOF-VERIFICATION"}
var workflowUseCaseIDs = append(append([]string{}, useCaseIDs...), "UC-TASK-ORIENTATION", "UC-CHANGE-CONSEQUENCE", "UC-EVIDENCE-CARRYING-COMPLETION")
var useCaseProfiles = map[string][]string{legacyProfile: useCaseIDs, profile: workflowUseCaseIDs}
var hexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var revisionPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
var requirementPattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,63}$`)

type validator struct {
	root           string
	errors         []string
	paths, digests map[string]bool
}

func str(v any) string { s, _ := v.(string); return s }
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
func digest(raw []byte) string                 { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func (v *validator) fail(label, reason string) { v.errors = append(v.errors, label+":"+reason) }
func (v *validator) closed(value any, fields []string, label string) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		v.fail(label, "not-object")
		return nil, false
	}
	valid := true
	for _, k := range fields {
		if _, ok := m[k]; !ok {
			v.fail(label, "missing-field:"+k)
			valid = false
		}
	}
	for k := range m {
		if !contains(fields, k) {
			v.fail(label, "extra-field:"+k)
			valid = false
		}
	}
	return m, valid
}
func (v *validator) read(relative any, label string) []byte {
	s, ok := relative.(string)
	if !ok || s == "" || utf8.RuneCountInString(s) > 512 || strings.ContainsAny(s, "\\\x00") || path.IsAbs(s) {
		v.fail(label, "invalid-path")
		return nil
	}
	for _, p := range strings.Split(s, "/") {
		if p == ".." {
			v.fail(label, "invalid-path")
			return nil
		}
	}
	if path.Clean(s) != s || s == "." {
		v.fail(label, "noncanonical-path")
		return nil
	}
	current := v.root
	for _, p := range strings.Split(s, "/") {
		current = filepath.Join(current, p)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				v.fail(label, "missing-file")
			} else {
				v.fail(label, "path-escape")
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			v.fail(label, "symlink")
			return nil
		}
	}
	f, err := os.Open(current)
	if err != nil {
		v.fail(label, "unreadable-file")
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		v.fail(label, "unreadable-file")
		return nil
	}
	if !info.Mode().IsRegular() {
		v.fail(label, "not-file")
		return nil
	}
	if info.Size() > maxFileBytes {
		v.fail(label, "file-too-large")
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		v.fail(label, "unreadable-file")
		return nil
	}
	if len(raw) > maxFileBytes {
		v.fail(label, "file-too-large")
		return nil
	}
	return raw
}

// Token decoding retains duplicate-key failures at every nesting depth.
func (v *validator) parse(raw []byte, label string) any {
	if !utf8.Valid(raw) {
		v.fail(label, "invalid-json")
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func() (any, error)
	read = func() (any, error) {
		t, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t {
		case json.Delim('{'):
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("key")
				}
				val, e := read()
				if e != nil {
					return nil, e
				}
				if _, ok := m[key]; ok {
					v.fail(label, "duplicate-key:"+key)
				}
				m[key] = val
			}
			_, err = d.Token()
			return m, err
		case json.Delim('['):
			a := []any{}
			for d.More() {
				val, e := read()
				if e != nil {
					return nil, e
				}
				a = append(a, val)
			}
			_, err = d.Token()
			return a, err
		default:
			return t, nil
		}
	}
	value, err := read()
	if err == nil {
		_, err = d.Token()
		if err == io.EOF {
			return value
		}
	}
	v.fail(label, "invalid-json")
	return nil
}
func arrayEqual(value any, want []string) bool {
	a, ok := value.([]any)
	if !ok || len(a) != len(want) {
		return false
	}
	for i, s := range want {
		if a[i] != s {
			return false
		}
	}
	return true
}
func (v *validator) stringList(value any, label string) []string {
	a, ok := value.([]any)
	if !ok || len(a) == 0 || len(a) > 128 {
		v.fail(label, "invalid-string-list")
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, item := range a {
		s, ok := item.(string)
		if !ok || s == "" || utf8.RuneCountInString(s) > 256 || seen[s] {
			v.fail(label, "invalid-string-list")
			return nil
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
func (v *validator) attestation(class string, value any, subjects []string, label string) {
	label += ":attestation"
	if _, ok := value.(map[string]any); !ok {
		v.fail(label, "not-object")
		return
	}
	switch class {
	case "contract", "implementation":
		key := "requirementIds"
		if class == "implementation" {
			key = "entrypoints"
		}
		m, ok := v.closed(value, []string{key}, label)
		if !ok {
			return
		}
		values := v.stringList(m[key], label+":"+key)
		if class == "contract" {
			for _, s := range values {
				if !requirementPattern.MatchString(s) {
					v.fail(label, "invalid-requirement-id")
					break
				}
			}
		}
	case "hostile-tests":
		m, ok := v.closed(value, []string{"cases"}, label)
		if ok && !arrayEqual(m["cases"], []string{"abstention", "hostile", "negative"}) {
			v.fail(label, "incomplete-hostile-cases")
		}
	case "corvint-dogfood", "beamfall-dogfood":
		m, ok := v.closed(value, []string{"outcome", "repository"}, label)
		if !ok {
			return
		}
		if m["repository"] != strings.TrimSuffix(class, "-dogfood") {
			v.fail(label, "repository-mismatch")
		}
		if m["outcome"] != "PASS" {
			v.fail(label, "outcome-not-pass")
		}
	case "sealed-benchmark":
		m, ok := v.closed(value, []string{"corpusSha256", "outcome", "preregistrationSha256", "resultSha256"}, label)
		if !ok {
			return
		}
		ds := []string{str(m["preregistrationSha256"]), str(m["corpusSha256"]), str(m["resultSha256"])}
		if !hexDigest.MatchString(ds[0]) || !hexDigest.MatchString(ds[1]) || !hexDigest.MatchString(ds[2]) {
			v.fail(label, "invalid-benchmark-seal")
		} else if ds[0] == ds[1] || ds[1] == ds[2] || ds[0] == ds[2] {
			v.fail(label, "colliding-benchmark-seal")
		} else {
			for _, s := range ds {
				if !contains(subjects, s) {
					v.fail(label, "unbound-benchmark-seal")
					break
				}
			}
		}
		if m["outcome"] != "PASS" {
			v.fail(label, "outcome-not-pass")
		}
	}
}
func (v *validator) receipt(id, class string, reference any, label string, verified bool) {
	m, ok := v.closed(reference, []string{"path", "sha256"}, label)
	if !ok {
		return
	}
	expected := str(m["sha256"])
	if !hexDigest.MatchString(expected) {
		v.fail(label, "invalid-sha256")
		return
	}
	raw := v.read(m["path"], label+":receipt")
	if raw == nil {
		return
	}
	actual := digest(raw)
	if actual != expected {
		v.fail(label, "receipt-digest-mismatch")
		return
	}
	p := str(m["path"])
	if v.paths[p] || v.digests[actual] {
		v.fail(label, "duplicate-receipt-reuse")
	}
	v.paths[p] = true
	v.digests[actual] = true
	label += ":receipt"
	m, ok = v.closed(v.parse(raw, label), []string{"attestation", "evidenceClass", "repositoryRevision", "result", "spec", "subjects", "useCaseId"}, label)
	if !ok {
		return
	}
	for _, check := range []struct{ key, want, reason string }{{"spec", evidenceProfile, "wrong-spec"}, {"useCaseId", id, "use-case-mismatch"}, {"evidenceClass", class, "evidence-class-mismatch"}, {"result", "PASS", "result-not-pass"}} {
		if m[check.key] != check.want {
			v.fail(label, check.reason)
		}
	}
	if !revisionPattern.MatchString(str(m["repositoryRevision"])) {
		v.fail(label, "invalid-repository-revision")
	}
	subjectDigests := []string{}
	raws := [][]byte{}
	paths := map[string]bool{}
	subjects, ok := m["subjects"].([]any)
	if !ok || len(subjects) == 0 || len(subjects) > 128 {
		v.fail(label, "invalid-subjects")
	} else {
		for i, subject := range subjects {
			sl := fmt.Sprintf("%s:subject:%d", label, i)
			sm, ok := v.closed(subject, []string{"path", "sha256"}, sl)
			if !ok {
				continue
			}
			sd := str(sm["sha256"])
			if !hexDigest.MatchString(sd) {
				v.fail(sl, "invalid-sha256")
				continue
			}
			if sp, ok := sm["path"].(string); ok {
				if paths[sp] {
					v.fail(sl, "duplicate-subject")
				}
				paths[sp] = true
			}
			sr := v.read(sm["path"], sl)
			if sr == nil {
				continue
			}
			if digest(sr) != sd {
				v.fail(sl, "subject-digest-mismatch")
				continue
			}
			subjectDigests = append(subjectDigests, sd)
			raws = append(raws, sr)
		}
	}
	v.attestation(class, m["attestation"], subjectDigests, label)
	if class == "contract" {
		v.clauses(raws, m["attestation"], label)
	}
	if verified && strings.HasSuffix(class, "-dogfood") {
		v.derivedOutcome(raws, label)
	}
}

// derivedOutcome re-derives a verified row's dogfood PASS from its retained report subjects
// instead of trusting the attestation token (UCV0-014): at least one recognized report and no
// recognized report that fails.
func (v *validator) derivedOutcome(raws [][]byte, label string) {
	passes := 0
	for _, raw := range raws {
		outcome := reportOutcome(raw)
		if outcome == "FAIL" {
			v.fail(label, "verified-unsupported-outcome")
			return
		}
		if outcome == "PASS" {
			passes++
		}
	}
	if passes == 0 {
		v.fail(label, "verified-unsupported-outcome")
	}
}

// reportOutcome reads a retained tool packet or corvint-dogfood-change/0 report; any other
// subject is unrecognized ("").
func reportOutcome(raw []byte) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if str(m["tool"]) != "" {
		return verdict(m["ok"] == true && dig(m, "context", "state") == "READY" && dig(m, "context", "abstention", "active") != true)
	}
	if m["profile"] == "corvint-dogfood-change/0" {
		return verdict(m["complete"] == true && dig(m, "dogfoodCheck", "outputsAgree") == true && dig(m, "ocmStatus", "aggregate", "coverage", "unknown") == float64(0))
	}
	return ""
}
func verdict(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}
func dig(value any, keys ...string) any {
	for _, k := range keys {
		m, _ := value.(map[string]any)
		value = m[k]
	}
	return value
}

// clauses checks every clause extract among a contract receipt's subjects against its live spec
// and requires the extracts, when present, to cover exactly the attested requirement IDs (UCV0-016).
func (v *validator) clauses(raws [][]byte, attestation any, label string) {
	covered := []string{}
	extracts := 0
	for i, raw := range raws {
		var probe map[string]any
		if json.Unmarshal(raw, &probe) != nil || probe["profile"] != clausesProfile {
			continue
		}
		extracts++
		covered = append(covered, v.extract(raw, fmt.Sprintf("%s:clauses:%d", label, i))...)
	}
	a, _ := attestation.(map[string]any)
	attested := []string{}
	ids, _ := a["requirementIds"].([]any)
	for _, id := range ids {
		attested = append(attested, str(id))
	}
	sort.Strings(covered)
	sort.Strings(attested)
	if extracts > 0 && !reflect.DeepEqual(covered, attested) {
		v.fail(label, "clause-coverage-mismatch")
	}
}
func (v *validator) extract(raw []byte, label string) []string {
	m, ok := v.closed(v.parse(raw, label), []string{"clauses", "profile", "spec"}, label)
	if !ok {
		return nil
	}
	spec := v.read(m["spec"], label+":spec")
	items, _ := m["clauses"].([]any)
	if spec == nil || len(items) == 0 || len(items) > 128 {
		v.fail(label, "invalid-clauses")
		return nil
	}
	ids := []string{}
	for i, item := range items {
		ids = append(ids, v.clause(spec, item, fmt.Sprintf("%s:clause:%d", label, i)))
	}
	return ids
}
func (v *validator) clause(spec []byte, item any, label string) string {
	m, ok := v.closed(item, []string{"id", "text"}, label)
	if !ok {
		return ""
	}
	id := str(m["id"])
	text, found := clauseText(spec, id)
	if found != 1 {
		v.fail(label, fmt.Sprintf("clause-definitions-%d:%s", found, id))
		return id
	}
	if text != str(m["text"]) {
		v.fail(label, "clause-drift:"+id)
	}
	return id
}

// clauseText returns the single "- `ID`:" bullet in a spec plus its indented continuation lines,
// and how many such bullets exist.
func clauseText(spec []byte, id string) (string, int) {
	lines := strings.Split(string(spec), "\n")
	starts := []int{}
	for i, line := range lines {
		if strings.HasPrefix(line, "- `"+id+"`:") {
			starts = append(starts, i)
		}
	}
	if len(starts) != 1 {
		return "", len(starts)
	}
	end := starts[0] + 1
	for end < len(lines) && strings.HasPrefix(lines[end], " ") {
		end++
	}
	return strings.Join(lines[starts[0]:end], "\n"), 1
}
func validate(root, ledgerPath string) map[string]any {
	v := &validator{paths: map[string]bool{}, digests: map[string]bool{}}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		v.fail("root", "unavailable")
		return v.failure()
	}
	v.root, err = filepath.Abs(resolved)
	if err != nil {
		v.fail("root", "unavailable")
		return v.failure()
	}
	raw := v.read(ledgerPath, "ledger")
	if raw == nil {
		return v.failure()
	}
	ledger, ok := v.closed(v.parse(raw, "ledger"), []string{"claimDefault", "evidenceClasses", "spec", "statuses", "useCases"}, "ledger")
	if !ok {
		return v.failure()
	}
	ids, knownProfile := useCaseProfiles[str(ledger["spec"])]
	if !knownProfile {
		v.fail("ledger", "wrong-spec")
		return v.failure()
	}
	if ledger["claimDefault"] != "UNPROVEN" {
		v.fail("ledger", "wrong-claim-default")
	}
	if !arrayEqual(ledger["statuses"], statuses) {
		v.fail("ledger", "wrong-status-model")
	}
	if !arrayEqual(ledger["evidenceClasses"], evidenceClasses) {
		v.fail("ledger", "wrong-evidence-classes")
	}
	sc := map[string]any{}
	cc := map[string]any{}
	for _, s := range statuses {
		sc[s] = 0
	}
	for _, s := range claims {
		cc[s] = 0
	}
	ec := 0
	seen := map[string]bool{}
	rows, ok := ledger["useCases"].([]any)
	if !ok {
		v.fail("ledger:useCases", "not-array")
	}
	for i, row := range rows {
		label := fmt.Sprintf("use-case:%d", i)
		m, ok := v.closed(row, []string{"claim", "evidence", "id", "job", "status", "title"}, label)
		if !ok {
			continue
		}
		id := str(m["id"])
		if !contains(ids, id) {
			v.fail(label, "unknown-id")
			continue
		}
		if seen[id] {
			v.fail(label, "duplicate-id")
		}
		seen[id] = true
		label = "use-case:" + id
		for _, field := range []string{"job", "title"} {
			s := str(m[field])
			if contextindex.TrimPythonSpace(s) == "" || utf8.RuneCountInString(s) > 512 {
				v.fail(label, "invalid-"+field)
			}
		}
		status, claim := str(m["status"]), str(m["claim"])
		if !contains(statuses, status) {
			v.fail(label, "invalid-status")
		} else {
			sc[status] = sc[status].(int) + 1
		}
		if !contains(claims, claim) {
			v.fail(label, "invalid-claim")
		} else {
			cc[claim] = cc[claim].(int) + 1
		}
		if status == "verified" && claim != "VERIFIED" {
			v.fail(label, "verified-status-requires-verified-claim")
		}
		if (status == "specified" || status == "experimental") && claim != "UNPROVEN" {
			v.fail(label, "unproven-status-forbids-verified-claim")
		}
		evidence, ok := m["evidence"].(map[string]any)
		if !ok {
			v.fail(label+":evidence", "not-object")
			continue
		}
		for class := range evidence {
			if !contains(evidenceClasses, class) {
				v.fail(label+":evidence", "unknown-class:"+class)
			}
		}
		for _, class := range evidenceClasses {
			ref, present := evidence[class]
			if present {
				ec++
				v.receipt(id, class, ref, label+":evidence:"+class, status == "verified")
			} else if status == "verified" {
				v.fail(label, "verified-missing-evidence:"+class)
			} else if status == "experimental" && (class == "contract" || class == "implementation") {
				v.fail(label, "experimental-missing-evidence:"+class)
			}
		}
	}
	for _, id := range ids {
		if !seen[id] {
			v.fail("ledger", "missing-use-case:"+id)
		}
	}
	if len(rows) > len(ids) {
		v.fail("ledger", "unexpected-use-case-count")
	}
	if len(v.errors) > 0 {
		return v.failure()
	}
	return map[string]any{"claimCounts": cc, "evidenceCount": ec, "ledgerSha256": digest(raw), "profile": resultProfile, "statusCounts": sc, "useCaseCount": len(rows), "valid": true}
}
func (v *validator) failure() map[string]any {
	sort.Strings(v.errors)
	out := []any{}
	for i, e := range v.errors {
		if i == 0 || e != v.errors[i-1] {
			out = append(out, e)
		}
	}
	return map[string]any{"errors": out, "profile": resultProfile, "valid": false}
}
func main() {
	root := flag.String("root", ".", "repository root")
	ledger := flag.String("ledger", "conformance/use-cases-v0/ledger.json", "repository-relative ledger")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	result := validate(*root, *ledger)
	raw, err := contextindex.CanonicalJSON(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(raw))
	if !reflect.DeepEqual(result["valid"], true) {
		os.Exit(1)
	}
}
