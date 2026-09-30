package postmergeconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func digest(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func StableKey(b Binding) string { return "pm-" + digest(b) }
func validURL(s string, p Policy) bool {
	if len(s) > 2048 || strings.ContainsAny(s, "\r\n\t `[]()\\<>") {
		return false
	}
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && slices.Contains(p.URLOrigins, u.Scheme+"://"+u.Host)
}
func Build(ctx context.Context, root string, f Fixture, p Policy, in Input) (Plan, error) {
	if in.Binding != p.Expected {
		return Plan{}, fmt.Errorf("input-binding-mismatch")
	}
	read, err := Read(ctx, root, f, p, in.SourceItem)
	if err != nil {
		return Plan{}, err
	}
	for _, n := range []int{in.Counts.Docs, in.Counts.Tests, in.Counts.Corpus} {
		if n < 0 || n > 1000000 {
			return Plan{}, fmt.Errorf("count-invalid")
		}
	}
	if len(in.Findings) > p.MaxFindings || len(in.Drafts) > 64 {
		return Plan{}, fmt.Errorf("batch-cap-exceeded")
	}
	// Normalize ordered request output while rejecting duplicate logical operations.
	in.Findings = slices.Clone(in.Findings)
	in.Drafts = slices.Clone(in.Drafts)
	slices.SortFunc(in.Findings, func(a, b Finding) int { return strings.Compare(digest(a), digest(b)) })
	slices.SortFunc(in.Drafts, func(a, b Draft) int { return strings.Compare(a.Kind, b.Kind) })
	key := StableKey(in.Binding)
	parent := read.Hierarchy[len(read.Hierarchy)-1].ID
	requests := []Request{}
	seen := map[string]bool{}
	add := func(r Request) error {
		if seen[r.Key] {
			return fmt.Errorf("duplicate-operation")
		}
		seen[r.Key] = true
		requests = append(requests, r)
		return nil
	}
	baseRequest := func(op, suffix string) Request {
		return Request{Profile: Profile, Key: key + "-" + suffix, Operation: op, Source: in.SourceItem, Parent: parent}
	}
	// Renderers have fixed vocabulary; there is no title/body/template slot.
	if in.Counts != (Counts{}) || len(in.Findings) > 0 || len(in.Drafts) > 0 {
		r := baseRequest("upsert-followup", "followup")
		r.Route = "ordinary"
		r.Body = fmt.Sprintf("Post-merge follow-up\nChange: %s\nMerge: %s\nSource: %s\nParent: %s\nDocs: %d\nTests: %d\nCorpus: %d\nFindings: %d\nDrafts: %d\n", in.Binding.Change, in.Binding.Merge, in.SourceItem, parent, in.Counts.Docs, in.Counts.Tests, in.Counts.Corpus, len(in.Findings), len(in.Drafts))
		_ = add(r)
	}
	for _, finding := range in.Findings {
		if !slices.Contains(p.AllowedClasses, finding.Class) || !slices.Contains([]string{"ordinary", "security"}, finding.Classification) {
			return Plan{}, fmt.Errorf("finding-class-unadmitted")
		}
		if !lineExists(ctx, root, in.Binding.Merge, finding.Path, finding.Line) {
			return Plan{}, fmt.Errorf("finding-anchor-unavailable")
		}
		r := baseRequest("upsert-finding", "finding-"+digest(struct {
			Class, Path string
			Line        int
		}{finding.Class, finding.Path, finding.Line}))
		r.Route = "ordinary"
		if finding.Classification == "security" || finding.Class == "security" {
			r.Route = "restricted"
		}
		r.Body = fmt.Sprintf("Post-merge finding\nChange: %s\nMerge: %s\nClass: %s\nPath: %s\nLine: %d\nTriage: %s\n", in.Binding.Change, in.Binding.Merge, finding.Class, finding.Path, finding.Line, r.Route)
		if err := add(r); err != nil {
			return Plan{}, err
		}
	}
	for _, draft := range in.Drafts {
		if !slices.Contains([]string{"docs", "tests", "corpus"}, draft.Kind) || !oidPattern.MatchString(draft.Revision) || !validURL(draft.URL, p) {
			return Plan{}, fmt.Errorf("draft-invalid")
		}
		out, e := gitBytes(ctx, root, 256, "rev-parse", "--verify", draft.Revision+"^{commit}")
		if e != nil || strings.TrimSpace(string(out)) != draft.Revision {
			return Plan{}, fmt.Errorf("draft-revision-unavailable")
		}
		r := baseRequest("upsert-draft-change", draft.Kind)
		r.Draft = true
		r.Branch = "corvint/" + key + "/" + draft.Kind
		r.Body = fmt.Sprintf("Post-merge draft\nChange: %s\nMerge: %s\nKind: %s\nRevision: %s\nEvidence: %s\nDraft: true\n", in.Binding.Change, in.Binding.Merge, draft.Kind, draft.Revision, draft.URL)
		if err := add(r); err != nil {
			return Plan{}, err
		}
	}
	slices.SortFunc(requests, func(a, b Request) int { return strings.Compare(a.Key, b.Key) })
	plan := Plan{Profile: Profile, Input: in, Requests: requests}
	plan.Digest = digest(plan)
	return plan, nil
}

// ValidatePlan rederives every byte from current pinned inputs before effects.
// A copied digest or a caller-edited generated body never authorizes a request.
func ValidatePlan(ctx context.Context, root string, f Fixture, p Policy, plan Plan) error {
	actual, err := Build(ctx, root, f, p, plan.Input)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(plan)
	if string(a) != string(b) {
		return fmt.Errorf("plan-mismatch")
	}
	return nil
}
