// Package frontiernextrepo independently re-derives the experimental universe
// from immutable Git objects using the existing canonical CEM/OCM verifier.
package frontiernextrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/localauthority"
	"github.com/Beamfall/corvint/internal/lrf"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

type Adapter struct{ Root string }

func (a Adapter) Recompute(ctx context.Context, r frontiernext.Request, s frontiernext.Selection) (frontiernext.Universe, error) {
	return recompute(ctx, a.Root, nil, r, s)
}

type requestAdapter struct{ memo *gitauth.RequestReadMemo }

// NewRequestAdapter shares only immutable primitive successes between the
// canonical verifier and eligibility reader. The protected consumer creates the
// memo after its own view audit; this adapter never admits authority itself.
func NewRequestAdapter(memo *gitauth.RequestReadMemo) frontiernext.Recomputer {
	return requestAdapter{memo: memo}
}

func (a requestAdapter) Recompute(ctx context.Context, r frontiernext.Request, s frontiernext.Selection) (frontiernext.Universe, error) {
	if a.memo == nil {
		return frontiernext.Universe{}, localauthority.ErrInvalid
	}
	return recompute(ctx, "", a.memo, r, s)
}

func recompute(ctx context.Context, root string, memo *gitauth.RequestReadMemo, r frontiernext.Request, s frontiernext.Selection) (frontiernext.Universe, error) {
	out := frontiernext.Universe{Eligible: map[frontiernext.Edge]bool{}, Claims: map[string][]string{}, Open: []frontiernext.Item{}}
	binding := r.Enrollment.Binding
	var original struct {
		Claims []struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			BlobOID  string `json:"blobOid"`
			Selector string `json:"selector"`
		} `json:"claims"`
	}
	var u *lrfrepo.Universe
	var err error
	if memo == nil {
		u, err = lrfrepo.VerifyUniverse(ctx, root, r.CEM, r.OCM, binding.Base, binding.Target)
	} else {
		var universeReader *gitauth.Repository
		universeReader, err = memo.Open(gitrun.NewDefaultBudget())
		if err == nil {
			u, err = lrfrepo.VerifyUniverseWithRepository(ctx, universeReader, r.CEM, r.OCM, binding.Base, binding.Target)
		}
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(r.OCM, &original); err != nil {
		return out, localauthority.ErrInvalid
	}
	var repo *gitauth.Repository
	if memo == nil {
		repo, err = gitauth.Open(root, gitrun.NewDefaultBudget())
	} else {
		repo, err = memo.Open(gitrun.NewDefaultBudget())
	}
	if err != nil {
		return out, err
	}
	tree, err := repo.CommitTree(ctx, binding.Target)
	if err != nil || tree != binding.Tree {
		return out, localauthority.ErrInvalid
	}
	raw, err := repo.CanonicalDiff(ctx, binding.Base, binding.Target)
	if err != nil {
		return out, err
	}
	diff, err := patch.Parse(raw)
	if err != nil {
		return out, err
	}
	baseIntent, err := blob(ctx, repo, binding.Base, u.Intent.Path)
	if err != nil {
		return out, err
	}
	targetIntent, err := blob(ctx, repo, binding.Target, u.Intent.Path)
	if err != nil {
		return out, err
	}
	stableIntent := bytes.Equal(baseIntent, targetIntent)
	obligations := map[string]bool{}
	for _, o := range u.Obligations {
		out.Obligations = append(out.Obligations, o.ID)
		obligations[o.ID] = true
		out.Claims[o.ID] = o.ClaimIDs
		if o.Disposition != "linked" {
			out.Open = append(out.Open, frontiernext.Item{ID: o.ID, Reason: "INTENT_UNKNOWN"})
		}
	}
	lexical, err := lrf.Evaluate(u.LRFRequest)
	if err != nil {
		return out, err
	}
	witnessed := map[string]bool{}
	for _, row := range lexical.Results() {
		if row[0] == "cem-basis" && row[5] == "cem-lexical-v0" {
			witnessed[row[1]] = true
		}
	}
	for _, h := range u.CEM.Hunks {
		if h.Disposition != "mechanical" && (h.Disposition != "supported" || !witnessed[h.ID]) {
			out.Open = append(out.Open, frontiernext.Item{ID: h.ID, Reason: "CHANGE_UNKNOWN"})
		}
	}
	checks := map[string]localauthority.Check{}
	for _, c := range r.Enrollment.Checks {
		checks[c.ID] = c
	}
	for _, edge := range s.Edges {
		if !obligations[edge.ObligationID] {
			return out, localauthority.ErrInvalid
		}
		claimLinked := false
		for _, id := range out.Claims[edge.ObligationID] {
			if id == edge.ClaimID {
				claimLinked = true
			}
		}
		if !claimLinked {
			return out, localauthority.ErrInvalid
		}
		check, ok := checks[edge.CheckID]
		if !ok || !stableIntent || edge.Subject == u.Intent.Path {
			continue
		}
		statement := []byte(nil)
		hunkIDs := map[string]bool{}
		for _, o := range u.LRFRequest.Obligations {
			if o.ID == edge.ObligationID {
				statement = o.Statement
				for _, id := range o.HunkIDs {
					hunkIDs[id] = true
				}
			}
		}
		if !bytes.Contains(statement, []byte("`"+edge.Function+"`")) {
			continue
		}
		claimMatches := false
		for _, claim := range original.Claims {
			if claim.ID == edge.ClaimID && claim.Path == check.DriverPath && claim.Selector == check.ClaimSelector && strings.HasPrefix(claim.Selector, "test:"+check.DriverUnit+"/case:") {
				entry, exists, err := repo.LookupTreeEntry(ctx, binding.Target, check.DriverPath)
				claimMatches = err == nil && exists && entry.OID == claim.BlobOID
			}
		}
		if !claimMatches {
			continue
		}
		if !driverStable(ctx, repo, binding.Base, binding.Target, check) {
			continue
		}
		if !completePackage(ctx, repo, binding.Target, edge.Subject) {
			continue
		}
		source, err := blob(ctx, repo, binding.Target, edge.Subject)
		if err != nil {
			continue
		}
		start, end, ok := functionSpan(edge.Subject, source, edge.Function)
		if !ok {
			continue
		}
		out.Eligible[edge] = materialIntersection(diff, edge.Subject, start, end, hunkIDs)
	}
	out.Identity, err = localauthority.Digest(struct {
		Profile     string                 `json:"profile"`
		Binding     localauthority.Binding `json:"binding"`
		Selection   frontiernext.Selection `json:"selection"`
		PatchSHA256 string                 `json:"patchSHA256"`
	}{frontiernext.Profile, binding, s, u.PatchSHA256})
	return out, err
}
func blob(ctx context.Context, repo *gitauth.Repository, revision, p string) ([]byte, error) {
	if p == "" || path.Clean(p) != p || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") {
		return nil, localauthority.ErrInvalid
	}
	entry, exists, err := repo.LookupTreeEntry(ctx, revision, p)
	if err != nil {
		return nil, err
	}
	if !exists || entry.Mode != "100644" {
		return nil, localauthority.ErrInvalid
	}
	return repo.BlobBytes(ctx, entry.OID)
}
func driverStable(ctx context.Context, repo *gitauth.Repository, base, target string, c localauthority.Check) bool {
	before, err := blob(ctx, repo, base, c.DriverPath)
	if err != nil {
		return false
	}
	after, err := blob(ctx, repo, target, c.DriverPath)
	if err != nil {
		return false
	}
	if !bytes.Equal(before, after) || localauthority.BytesDigest(after) != c.DriverSHA256 {
		return false
	}
	_, _, ok := functionSpan(c.DriverPath, after, c.DriverUnit)
	return ok
}

// Only unique top-level Go functions are supported; methods and ambiguous
// definitions remain OPEN. Parsing is bounded and does not typecheck or run code.
func functionSpan(name string, source []byte, symbol string) (int, int, bool) {
	if !strings.HasSuffix(name, ".go") || len(source) > 1<<20 || !token.IsIdentifier(symbol) {
		return 0, 0, false
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, name, source, 0)
	if err != nil {
		return 0, 0, false
	}
	start, end, count := 0, 0, 0
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if ok && f.Name.Name == symbol {
			count++
			if f.Recv == nil && f.Body != nil {
				start = fs.Position(f.Pos()).Line
				end = fs.Position(f.End()).Line
			}
		}
	}
	return start, end, count == 1 && start > 0
}
func materialIntersection(p *patch.Patch, name string, start, end int, ids map[string]bool) bool {
	for _, h := range p.Hunks {
		if h.NewPath != nil && *h.NewPath == name && ids[h.ID] && materialHunk(h, start, end) {
			return true
		}
	}
	return false
}
func materialHunk(h *patch.Hunk, start, end int) bool {
	removed := map[string]bool{}
	for _, body := range h.Body {
		if body.Prefix == '-' {
			removed[compact(body.Payload)] = true
		}
	}
	line := int(h.NewRange.Start)
	for _, body := range h.Body {
		if body.Prefix == '-' {
			continue
		}
		normalized := compact(body.Payload)
		if body.Prefix == '+' && line >= start && line <= end && normalized != "" && !removed[normalized] {
			return true
		}
		line++
	}
	return false
}
func compact(raw []byte) string { return strings.Join(strings.Fields(string(raw)), "") }

// The selected execution recipe compiles precisely codec.go. An undeclared
// sibling production input cannot be hidden by the one-file capsule.
func completePackage(ctx context.Context, repo *gitauth.Repository, target, subject string) bool {
	if subject != "internal/wp3codec/codec.go" {
		return false
	}
	paths, err := repo.TreePaths(ctx, target, path.Dir(subject))
	if err != nil {
		return false
	}
	for _, p := range paths {
		if path.Dir(p) != path.Dir(subject) {
			return false
		}
		if p != subject && !strings.HasSuffix(p, "_test.go") {
			return false
		}
		entry, exists, err := repo.LookupTreeEntry(ctx, target, p)
		if err != nil || !exists || entry.Mode != "100644" {
			return false
		}
	}
	return true
}
