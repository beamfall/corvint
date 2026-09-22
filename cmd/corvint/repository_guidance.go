package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/genesis"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

type guidanceInvocation struct {
	Root, Command, Base string
	MaxRefs             int
	removeScratch       func(string) error
}
type guidanceEvidence struct {
	Path  string `json:"path"`
	Blob  string `json:"blob"`
	Start int    `json:"startLine"`
	End   int    `json:"endLine"`
	Rule  string `json:"rule"`
}
type guidanceFeature struct {
	Label     string             `json:"label"`
	Authority string             `json:"authority"`
	Evidence  []guidanceEvidence `json:"evidence"`
}
type guidanceCall struct {
	Argv      []string `json:"argv"`
	Inert     bool     `json:"inert"`
	Authority string   `json:"authority"`
}
type guidanceSkip struct {
	Ref    string `json:"ref"`
	Tip    string `json:"tip"`
	Reason string `json:"reason"`
}
type guidanceOverlap struct {
	Ref   string   `json:"ref"`
	Tip   string   `json:"tip"`
	Base  string   `json:"mergeBase"`
	State string   `json:"state"`
	Paths []string `json:"paths"`
}
type guidanceReview struct {
	CurrentRef      string            `json:"currentRef"`
	Base            string            `json:"base"`
	Affected        affectedReceipt   `json:"affected"`
	ChangedFeatures []guidanceFeature `json:"changedFeatures"`
	LocalTips       map[string]string `json:"localTips"`
	Skipped         []guidanceSkip    `json:"skipped"`
	Overlaps        []guidanceOverlap `json:"overlaps"`
}
type guidanceReceipt struct {
	Profile      string             `json:"profile"`
	Tool         string             `json:"tool"`
	Experimental bool               `json:"experimental"`
	Authority    string             `json:"authority"`
	Mutates      bool               `json:"mutates"`
	Revision     string             `json:"revision"`
	Tree         string             `json:"tree"`
	Features     []guidanceFeature  `json:"features"`
	Languages    []string           `json:"languages,omitzero"`
	Entrypoints  []guidanceEvidence `json:"entrypoints,omitzero"`
	Manifests    []guidanceEvidence `json:"manifests,omitzero"`
	Tests        []guidanceEvidence `json:"tests,omitzero"`
	Index        map[string]string  `json:"index"`
	Unknown      []string           `json:"unknown"`
	Omissions    map[string]int     `json:"omissions"`
	NextCalls    []guidanceCall     `json:"nextCalls"`
	Review       *guidanceReview    `json:"review,omitempty"`
}

func parseGuidanceInvocation(args []string) (guidanceInvocation, bool, error) {
	out := guidanceInvocation{Root: ".", MaxRefs: 32}
	if _, help, _ := parseHelpInvocation(args); help {
		return out, false, nil
	}
	i := 0
	for i < len(args) && (args[i] == "--root" || strings.HasPrefix(args[i], "--root=")) {
		if args[i] == "--root" {
			if !rootPreambleValue(args, i+1) {
				return out, false, nil
			}
			out.Root = args[i+1]
			i += 2
		} else {
			out.Root = strings.TrimPrefix(args[i], "--root=")
			i++
		}
	}
	if i >= len(args) {
		return out, false, nil
	}
	out.Command = args[i]
	if out.Command != "features" && out.Command != "overview" && out.Command != "review" {
		return out, false, nil
	}
	seen := map[string]bool{}
	for i++; i < len(args); i++ {
		key, value, equals := strings.Cut(args[i], "=")
		if out.Command != "review" || (key != "--base" && key != "--max-refs") || seen[key] {
			return out, true, argumentError("unrecognized guidance argument")
		}
		seen[key] = true
		if !equals {
			i++
			if i >= len(args) {
				return out, true, argumentError("guidance option requires a value")
			}
			value = args[i]
		}
		if key == "--base" {
			out.Base = value
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 32 {
			return out, true, argumentError("--max-refs must be 1..32")
		}
		out.MaxRefs = n
	}
	if out.Command == "review" && !validGitObjectID(out.Base) {
		return out, true, argumentError("review --base requires a full commit id")
	}
	resolved, err := resolveExplicitRoot(out.Root)
	out.Root = resolved
	return out, true, err
}

func guidanceError(err error) error {
	return &gokernel.Error{Code: "unsupported-repository-guidance", Message: err.Error()}
}

func runGuidance(ctx context.Context, in guidanceInvocation, stdout, stderr io.Writer) int {
	raw, err := compileGuidance(ctx, in, nil)
	if err != nil {
		emitError(stderr, guidanceError(err))
		return 2
	}
	if _, err = stdout.Write(append(raw, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write guidance receipt"})
		return 2
	}
	return 0
}

// beforeCheck is a test seam for deterministic source/ref drift, never a CLI option.
func compileGuidance(ctx context.Context, in guidanceInvocation, beforeCheck func()) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	snapshot, err := genesis.OpenGuidance(ctx, in.Root)
	if err != nil {
		return nil, err
	}
	receipt := discoverGuidance(snapshot, in.Command)
	refs := ""
	if in.Command == "review" {
		refs, err = snapshot.RefTips(ctx)
		if err != nil {
			return nil, err
		}
		receipt.Review, err = reviewGuidance(ctx, snapshot, in, refs, &receipt)
		if err != nil {
			return nil, err
		}
	}
	raw, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		return nil, err
	}
	if len(raw) > 1<<20 {
		return nil, fmt.Errorf("guidance output exceeds 1 MiB")
	}
	if beforeCheck != nil {
		beforeCheck()
	}
	if err = snapshot.Check(ctx); err != nil {
		return nil, err
	}
	if in.Command == "review" {
		after, e := snapshot.RefTips(ctx)
		if e != nil {
			return nil, e
		}
		if after != refs {
			return nil, fmt.Errorf("local ref drift")
		}
		current, e := snapshot.CurrentRef(ctx)
		if e != nil {
			return nil, e
		}
		if current != receipt.Review.CurrentRef {
			return nil, fmt.Errorf("current ref drift")
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

var guidanceMarkers = regexp.MustCompile(`(?i)\b(feature|scenario)\s*:\s*([^\r\n]{1,120})`)
var guidanceRegistrations = regexp.MustCompile(`\b(HandleFunc|Handle|Get|Post|Put|Delete|Patch|get|post|put|delete|patch|route|AddTool|addTool|registerTool|register_tool|tool)\s*\(\s*["']([^"'\r\n]{1,120})["']`)
var guidanceLanguages = map[string]string{".go": "Go", ".js": "JavaScript", ".jsx": "JavaScript", ".ts": "TypeScript", ".tsx": "TypeScript", ".py": "Python", ".rb": "Ruby", ".rs": "Rust", ".java": "Java", ".kt": "Kotlin", ".swift": "Swift", ".cs": "C#", ".c": "C", ".cpp": "C++"}
var guidanceManifests = map[string]bool{"package.json": true, "go.mod": true, "Cargo.toml": true, "pyproject.toml": true, "Package.swift": true, "AndroidManifest.xml": true, "Info.plist": true, "Gemfile": true, "pom.xml": true}

func discoverGuidance(s *genesis.GuidanceSnapshot, command string) guidanceReceipt {
	out := guidanceReceipt{Profile: "repository-guidance/0", Tool: command, Experimental: true, Authority: "inferred", Revision: s.Revision, Tree: s.Tree,
		Features: []guidanceFeature{}, Languages: []string{}, Entrypoints: []guidanceEvidence{}, Manifests: []guidanceEvidence{}, Tests: []guidanceEvidence{},
		Index:     map[string]string{"state": "UNKNOWN", "reason": "not-read; index-derived fields omitted"},
		Unknown:   []string{"heuristics are not exhaustive; dynamic registrations and unsupported syntax/languages remain unknown", "advisory only; CEM, OCM, frontier and mandatory test obligations remain open"},
		Omissions: map[string]int{"inventory": s.Omitted}, NextCalls: guidanceNextCalls(command)}
	candidates := map[string]*guidanceFeature{}
	languages := map[string]bool{}
	for _, r := range s.Records {
		if r.Path == nil {
			out.Omissions["unread-sources"]++
			continue
		}
		p := *r.Path
		if language := guidanceLanguages[path.Ext(p)]; language != "" {
			languages[language] = true
		}
		if r.Classification != "INCLUDED" {
			out.Omissions["unread-sources"]++
			continue
		}
		body := string(s.Blobs[r.OID])
		e := guidanceEvidence{Path: p, Blob: r.OID, Start: 1, End: 1}
		if strings.HasPrefix(p, "cmd/") && path.Base(p) == "main.go" {
			e.Rule = "cmd-entrypoint"
			appendGuidanceEvidence(&out.Entrypoints, e, &out)
			addGuidanceFeature(candidates, path.Dir(p), e, &out)
		}
		if guidanceManifests[path.Base(p)] {
			e.Rule = "package-manifest"
			appendGuidanceEvidence(&out.Manifests, e, &out)
			addGuidanceFeature(candidates, p, e, &out)
			if path.Base(p) == "package.json" {
				discoverPackage(body, e, candidates, &out)
			}
		}
		if strings.HasSuffix(p, "_test.go") || strings.Contains(p, "/e2e/") || strings.Contains(p, ".spec.") || strings.Contains(p, ".test.") || strings.HasPrefix(path.Base(p), "test_") {
			e.Rule = "test-convention"
			appendGuidanceEvidence(&out.Tests, e, &out)
		}
		for line, text := range strings.Split(body, "\n") {
			for _, rule := range []struct {
				id      string
				pattern *regexp.Regexp
			}{{"literal-marker", guidanceMarkers}, {"literal-registration", guidanceRegistrations}} {
				matches := rule.pattern.FindAllStringSubmatch(text, -1)
				if len(matches) > 16 {
					out.Omissions["line-matches"] += len(matches) - 16
					matches = matches[:16]
				}
				for _, match := range matches {
					e.Start = line + 1
					e.End = line + 1
					e.Rule = rule.id
					addGuidanceFeature(candidates, strings.TrimSpace(match[2]), e, &out)
				}
			}
		}
	}
	for language := range languages {
		out.Languages = append(out.Languages, language)
	}
	sort.Strings(out.Languages)
	labels := make([]string, 0, len(candidates))
	for label := range candidates {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		out.Features = append(out.Features, *candidates[label])
	}
	if out.Omissions["inventory"] > 0 || out.Omissions["unread-sources"] > 0 {
		out.Unknown = append(out.Unknown, "inventory or source bytes omitted; absence is not negative evidence")
	}
	if command == "features" {
		out.Languages = nil
		out.Entrypoints = nil
		out.Manifests = nil
		out.Tests = nil
		out.Unknown = append(out.Unknown, "overview-only languages, entrypoints, manifests and tests fields omitted; use overview")
	}
	return out
}

func appendGuidanceEvidence(target *[]guidanceEvidence, e guidanceEvidence, out *guidanceReceipt) {
	if len(*target) >= 256 {
		out.Omissions["overview-evidence"]++
		return
	}
	*target = append(*target, e)
}

func addGuidanceFeature(candidates map[string]*guidanceFeature, label string, e guidanceEvidence, out *guidanceReceipt) {
	if label == "" {
		return
	}
	if len(label) > 120 {
		out.Omissions["long-labels"]++
		return
	}
	candidate := candidates[label]
	if candidate == nil {
		if len(candidates) >= 256 {
			out.Omissions["features"]++
			return
		}
		candidate = &guidanceFeature{Label: label, Authority: "inferred", Evidence: []guidanceEvidence{}}
		candidates[label] = candidate
	}
	for _, existing := range candidate.Evidence {
		if existing == e {
			return
		}
	}
	if len(candidate.Evidence) >= 16 {
		out.Omissions["feature-evidence"]++
		return
	}
	candidate.Evidence = append(candidate.Evidence, e)
}

func discoverPackage(body string, e guidanceEvidence, candidates map[string]*guidanceFeature, out *guidanceReceipt) {
	e.Start = 1
	e.End = strings.Count(body, "\n") + 1
	var manifest map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &manifest) != nil {
		out.Omissions["manifest-parse"]++
		return
	}
	for _, field := range []string{"bin", "scripts"} {
		raw, exists := manifest[field]
		if !exists {
			continue
		}
		var values map[string]json.RawMessage
		if json.Unmarshal(raw, &values) != nil {
			var value string
			if field == "bin" && json.Unmarshal(raw, &value) == nil {
				e.Rule = "manifest-bin"
				addGuidanceFeature(candidates, e.Path+":bin", e, out)
				appendGuidanceEvidence(&out.Entrypoints, e, out)
				continue
			}
			out.Omissions["manifest-parse"]++
			continue
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			e.Rule = "manifest-" + field
			// The complete JSON blob is the parser's evidence, including multiline objects.
			e.Start = 1
			e.End = strings.Count(body, "\n") + 1
			addGuidanceFeature(candidates, e.Path+":"+field+":"+key, e, out)
			if field == "bin" {
				appendGuidanceEvidence(&out.Entrypoints, e, out)
			}
			if field == "scripts" && strings.HasPrefix(key, "test") {
				appendGuidanceEvidence(&out.Tests, e, out)
			}
		}
	}
}

func guidanceNextCalls(command string) []guidanceCall {
	table := map[string][][]string{
		"features": {{"corvint", "overview"}, {"corvint", "help", "feature"}},
		"overview": {{"corvint", "features"}, {"corvint", "help", "review"}, {"corvint", "help", "affected"}},
		"review":   {{"corvint", "help", "cem"}, {"corvint", "help", "ocm"}, {"corvint", "help", "frontier"}},
	}
	out := []guidanceCall{}
	for _, argv := range table[command] {
		out = append(out, guidanceCall{Argv: argv, Inert: true, Authority: "none"})
	}
	return out
}

func reviewGuidance(ctx context.Context, s *genesis.GuidanceSnapshot, in guidanceInvocation, refs string, out *guidanceReceipt) (result *guidanceReview, resultErr error) {
	out.Unknown = append(out.Unknown, "changedFeatures is target-tree inferred; deleted/base-only candidates are not discovered")
	ancestor, err := s.MergeBase(ctx, in.Base, s.Revision)
	if err != nil {
		return nil, err
	}
	if ancestor != in.Base {
		return nil, fmt.Errorf("base must be an ancestor of target")
	}
	paths, err := s.Paths(ctx, in.Base, s.Revision)
	if err != nil {
		return nil, err
	}
	review := &guidanceReview{Base: in.Base, ChangedFeatures: []guidanceFeature{}, LocalTips: map[string]string{}, Skipped: []guidanceSkip{}, Overlaps: []guidanceOverlap{}}
	changed := map[string]bool{}
	for _, p := range paths {
		changed[p] = true
	}
	for _, feature := range out.Features {
		for _, e := range feature.Evidence {
			if changed[e.Path] {
				review.ChangedFeatures = append(review.ChangedFeatures, feature)
				break
			}
		}
	}
	scratch, err := s.Materialize()
	if err != nil {
		return nil, err
	}
	remove := in.removeScratch
	if remove == nil {
		remove = os.RemoveAll
	}
	defer func() {
		resultErr = cleanupGuidanceScratch(scratch, resultErr, remove)
		if resultErr != nil {
			result = nil
		}
	}()
	graph, err := affected.Build(scratch, affectedLanguages()...)
	if err != nil {
		return nil, err
	}
	plan := affected.Select(graph, affected.NormalizePaths(paths))
	provider := providerGoProjection(graph, plan)
	advice := compileAffectedAdvice(scratch, plan, provider)
	if out.Omissions["inventory"] > 0 || out.Omissions["unread-sources"] > 0 {
		advice.Unknown = append(advice.Unknown, "immutable source inventory incomplete")
	}
	review.Affected = affectedReceipt{Advice: advice, Mutates: false, OK: true, Plan: plan, Profile: affectedProfile, Provider: affectedProvider{Go: provider}, Range: affectedRange{Base: in.Base, Paths: paths}, Revision: s.Revision, Tool: "affected"}
	current, err := s.CurrentRef(ctx)
	if err != nil {
		return nil, err
	}
	review.CurrentRef = current
	names := []string{}
	for _, line := range strings.Split(strings.TrimSpace(refs), "\n") {
		if line == "" {
			continue
		}
		name, tip, ok := strings.Cut(line, " ")
		if !ok || !strings.HasPrefix(name, "refs/heads/") || !validGitObjectID(tip) {
			return nil, fmt.Errorf("malformed ref map")
		}
		review.LocalTips[name] = tip
		names = append(names, name)
	}
	admitted := []string{}
	processed := 0
	for _, name := range names {
		tip := review.LocalTips[name]
		if name == current {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "current-ref"})
			continue
		}
		if processed >= in.MaxRefs {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "ref-budget"})
			out.Omissions["refs"]++
			continue
		}
		processed++
		base, e := s.MergeBase(ctx, tip, s.Revision)
		if e != nil {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "UNKNOWN ancestry unavailable"})
			continue
		}
		reason := ""
		if base == tip {
			reason = "ancestor-of-target"
		}
		if base == s.Revision {
			reason = "descendant-of-target"
		}
		if reason != "" {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, reason})
			continue
		}
		admitted = append(admitted, name)
	}
	overlapBudget := 64
	for i, name := range admitted {
		tip := review.LocalTips[name]
		stacked := false
		for j, other := range admitted {
			if i == j {
				continue
			}
			otherTip := review.LocalTips[other]
			base, e := s.MergeBase(ctx, tip, otherTip)
			if e != nil {
				out.Omissions["ancestry-comparisons"]++
				continue
			}
			if base == tip && (tip != otherTip || i > j) {
				stacked = true
				break
			}
		}
		if stacked {
			review.Skipped = append(review.Skipped, guidanceSkip{name, tip, "ancestry-stacked"})
			continue
		}
		base, e := s.MergeBase(ctx, in.Base, tip)
		if e != nil {
			review.Overlaps = append(review.Overlaps, guidanceOverlap{Ref: name, Tip: tip, State: "UNKNOWN", Paths: []string{}})
			continue
		}
		branchPaths, e := s.Paths(ctx, base, tip)
		row := guidanceOverlap{Ref: name, Tip: tip, Base: base, State: "DISJOINT", Paths: []string{}}
		if e != nil || len(branchPaths) > 256 {
			row.State = "UNKNOWN"
			out.Omissions["branch-paths"]++
			review.Overlaps = append(review.Overlaps, row)
			continue
		}
		for _, p := range branchPaths {
			if changed[p] {
				row.Paths = append(row.Paths, p)
			}
		}
		if len(row.Paths) > 0 {
			row.State = "OVERLAP"
		}
		if len(row.Paths) > overlapBudget {
			out.Omissions["overlap-paths"] += len(row.Paths) - overlapBudget
			row.Paths = row.Paths[:overlapBudget]
			row.State = "OVERLAP_INCOMPLETE"
		}
		overlapBudget -= len(row.Paths)
		review.Overlaps = append(review.Overlaps, row)
	}
	return review, nil
}

func cleanupGuidanceScratch(root string, original error, remove func(string) error) error {
	if err := remove(root); err != nil {
		return errors.Join(original, fmt.Errorf("scratch cleanup failed for %q: %w", root, err))
	}
	return original
}
