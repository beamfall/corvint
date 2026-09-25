package appflows

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
)

// ReviewIdentityNote is stated wherever reviewed links are reported (AFU-V1-008).
const ReviewIdentityNote = "review identity is not verified: a review anchor attests only that a commit which changed the flow intent is an ancestor of the evaluated revision and the link target is unchanged since"

// Evaluated review states; only `reviewed` makes a link's basis `reviewed` (AFU-V1-008, AFU-V1-009).
const (
	ReviewReviewed            = "reviewed"
	ReviewStale               = "stale"
	ReviewNotAncestor         = "anchor-not-ancestor"
	ReviewNotIntentChange     = "anchor-not-intent-change"
	ReviewAnchorUnknown       = "anchor-unknown"
	ReviewLinkNotAtAnchor     = "link-not-at-anchor"
	ReviewEvidenceUnavailable = "evidence-unavailable"
	ReviewTargetUnpinned      = "target-unpinned"
	ReviewUnreviewed          = "unreviewed"
	ReviewInferred            = "inferred"
)

// EvaluatedLink is one forward link evaluated at one revision (AFU-V1-007).
type EvaluatedLink struct {
	Flow              string     `json:"flow"`
	From              string     `json:"from"`
	Basis             string     `json:"basis"`
	ReviewState       string     `json:"review_state"`
	Target            LinkTarget `json:"target"`
	Blob              string     `json:"blob,omitempty"`
	Revision          string     `json:"revision"`
	ReviewedAt        string     `json:"reviewed_at,omitempty"`
	ReviewAttestation string     `json:"review_attestation,omitempty"`
}

// ReviewSummary counts reviewed links over declared links only; inferred links stay separate (AFU-V1-009).
type ReviewSummary struct {
	Revision            string `json:"revision"`
	Reviewed            int    `json:"reviewed"`
	ReviewedDenominator int    `json:"reviewed_denominator"`
	Stale               int    `json:"stale"`
	Inferred            int    `json:"inferred"`
	ReviewAttestation   string `json:"review_attestation"`
	Limitation          string `json:"limitation"`
}

type reviewer struct {
	ctx      context.Context
	root     string
	revision string
	blobs    map[string]string
	links    map[string][]FlowLink
}

// ResolveRevision names the evaluated commit.
func ResolveRevision(ctx context.Context, root, revision string) (string, error) {
	out, err := git(ctx, root, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", errors.New("evaluated revision is not a commit")
	}
	return strings.TrimSpace(string(out)), nil
}

// EvaluateLinks evaluates every forward link at revision. It reads only object IDs, ancestry and
// blob content; Git author, committer and signature fields are never read (AFU-V1-008).
func EvaluateLinks(ctx context.Context, root string, set IntentSet, revision string) ([]EvaluatedLink, error) {
	r := &reviewer{ctx: ctx, root: root, blobs: map[string]string{}, links: map[string][]FlowLink{}}
	var err error
	if r.revision, err = ResolveRevision(ctx, root, revision); err != nil {
		return nil, err
	}
	links := []EvaluatedLink{}
	for _, intent := range set.Flows {
		for _, link := range intent.Links {
			links = append(links, r.evaluate(set.IntentPath(intent.FlowID), intent.FlowID, link))
		}
	}
	return links, nil
}

func (r *reviewer) evaluate(intentPath, flow string, link FlowLink) EvaluatedLink {
	out := EvaluatedLink{Flow: flow, From: link.From, Basis: link.Basis, Target: link.Target, Revision: r.revision, ReviewedAt: link.ReviewedAt}
	if link.Target.Path != "" {
		out.Blob = r.blob(r.revision, link.Target.Path)
	}
	out.ReviewState = r.state(intentPath, link)
	if out.ReviewState == ReviewReviewed {
		out.Basis, out.ReviewAttestation = "reviewed", "self"
	}
	return out
}

func (r *reviewer) state(intentPath string, link FlowLink) string {
	if link.Basis == "inferred" {
		return ReviewInferred
	}
	if link.ReviewedAt == "" {
		return ReviewUnreviewed
	}
	anchor, err := ResolveRevision(r.ctx, r.root, link.ReviewedAt)
	if err != nil {
		return ReviewAnchorUnknown
	}
	if !r.ancestor(anchor) {
		return ReviewNotAncestor
	}
	if !r.changed(anchor, intentPath) {
		return ReviewNotIntentChange
	}
	if !slices.ContainsFunc(r.anchorLinks(anchor, intentPath), func(l FlowLink) bool { return l.From == link.From && l.Target == link.Target && l.Basis == "declared" }) {
		return ReviewLinkNotAtAnchor
	}
	return r.targetState(anchor, link.Target)
}

func (r *reviewer) ancestor(anchor string) bool {
	base, err := git(r.ctx, r.root, "merge-base", anchor, r.revision)
	return err == nil && strings.TrimSpace(string(base)) == anchor
}

// anchorLinks returns the links of the intent file as committed at anchor, so a review anchor covers
// only a link that the reviewed intent content already declared (AFU-V1-008).
func (r *reviewer) anchorLinks(anchor, intentPath string) []FlowLink {
	oid := r.blob(anchor, intentPath)
	if links, ok := r.links[oid]; ok {
		return links
	}
	var intent FlowIntent
	raw, err := git(r.ctx, r.root, "cat-file", "blob", oid)
	if err != nil || Decode(raw, &intent) != nil {
		intent.Links = nil
	}
	r.links[oid] = intent.Links
	return intent.Links
}

// changed reports whether commit changed file against its first parent (or added it as a root commit).
func (r *reviewer) changed(commit, file string) bool {
	after := r.blob(commit, file)
	if after == "" {
		return false
	}
	parent, err := git(r.ctx, r.root, "rev-parse", "--verify", "--quiet", "--end-of-options", commit+"^1")
	if err != nil {
		return true
	}
	return r.blob(strings.TrimSpace(string(parent)), file) != after
}

// targetState compares target content at anchor and at the evaluated revision. Only content identity
// counts, so a target changed and then restored (A to B to A) is unchanged. S1 has no run-evidence
// store, so an evidence target cannot be shown to exist at the evaluated revision and is never reviewed.
func (r *reviewer) targetState(anchor string, t LinkTarget) string {
	if t.Type == "evidence" {
		return ReviewEvidenceUnavailable
	}
	if t.Path == "" {
		return ReviewTargetUnpinned
	}
	before, after := r.blob(anchor, t.Path), r.blob(r.revision, t.Path)
	if before == "" || after == "" {
		return ReviewStale
	}
	if t.StartLine == 0 && before != after {
		return ReviewStale
	}
	if t.StartLine == 0 {
		return ReviewReviewed
	}
	left, okLeft := r.span(before, t.StartLine, t.EndLine)
	right, okRight := r.span(after, t.StartLine, t.EndLine)
	if !okLeft || !okRight || !bytes.Equal(left, right) {
		return ReviewStale
	}
	return ReviewReviewed
}

// blob returns the blob ID of file at commit, or "" when it is absent or not a blob.
func (r *reviewer) blob(commit, file string) string {
	key := commit + "\x00" + file
	if oid, ok := r.blobs[key]; ok {
		return oid
	}
	out, err := git(r.ctx, r.root, "--literal-pathspecs", "ls-tree", "-z", "--full-tree", commit, "--", file)
	oid := ""
	meta, name, _ := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	fields := strings.Fields(meta)
	if err == nil && len(fields) == 3 && fields[1] == "blob" && name == file {
		oid = fields[2]
	}
	r.blobs[key] = oid
	return oid
}

func (r *reviewer) span(blob string, start, end int) ([]byte, bool) {
	content, err := git(r.ctx, r.root, "cat-file", "blob", blob)
	if err != nil {
		return nil, false
	}
	lines := bytes.SplitAfter(content, []byte("\n"))
	if end > len(lines) || (end == len(lines) && len(lines[end-1]) == 0) {
		return nil, false
	}
	return bytes.Join(lines[start-1:end], nil), true
}

// Summarize counts reviewed links; the denominator is declared links only (AFU-V1-008, AFU-V1-009).
func Summarize(revision string, links []EvaluatedLink) ReviewSummary {
	s := ReviewSummary{Revision: revision, ReviewAttestation: "self", Limitation: ReviewIdentityNote}
	for _, l := range links {
		switch l.ReviewState {
		case ReviewInferred:
			s.Inferred++
			continue
		case ReviewReviewed:
			s.Reviewed++
		case ReviewStale:
			s.Stale++
		}
		s.ReviewedDenominator++
	}
	return s
}

// ReverseHit is one flow reached from a source path or test key.
type ReverseHit struct {
	Flow        string `json:"flow"`
	From        string `json:"from"`
	Basis       string `json:"basis"`
	ReviewState string `json:"review_state"`
}

// FlowsForPath derives source path to flows at query time from forward links; nothing is stored (AFU-V1-010).
func FlowsForPath(links []EvaluatedLink, file string) []ReverseHit {
	return reverse(links, func(t LinkTarget) bool { return t.Path == file })
}

// FlowsForTestKey derives test key to flows at query time from forward links (AFU-V1-010).
func FlowsForTestKey(links []EvaluatedLink, key string) []ReverseHit {
	return reverse(links, func(t LinkTarget) bool { return t.Type == "test" && t.TestKey == key })
}

func reverse(links []EvaluatedLink, match func(LinkTarget) bool) []ReverseHit {
	hits := []ReverseHit{}
	for _, l := range links {
		if match(l.Target) {
			hits = append(hits, ReverseHit{Flow: l.Flow, From: l.From, Basis: l.Basis, ReviewState: l.ReviewState})
		}
	}
	slices.SortFunc(hits, func(a, b ReverseHit) int {
		return strings.Compare(a.Flow+"\x00"+a.From+"\x00"+a.Basis, b.Flow+"\x00"+b.From+"\x00"+b.Basis)
	})
	return slices.Compact(hits)
}
