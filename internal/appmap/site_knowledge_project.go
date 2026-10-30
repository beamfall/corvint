package appmap

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// SKLimits is printed in every site-knowledge report (SKH-V0-002, SKH-V0-005).
const SKLimits = "static and lexical: page objects and test bodies are read from committed source at the map revision; " +
	"no code runs, no browser starts, no network is used; a construct the reader does not support is an unknown or an " +
	"opaque step, never a guess; the authority of every record is learned"

const maxSKSources = 16

// readSiteKnowledge reads the declared source directories at the map's revision of each root they
// name: --root, or an aliased root the map pinned (SKH-V0-189).
func readSiteKnowledge(ctx context.Context, m *Map, sources []string, o Options) (*SKModel, int, int, error) {
	in, entries, roots, skipped, err := readSKSources(ctx, m, sources, o, func(p string) bool {
		ext := strings.ToLower(path.Ext(p))
		return (webSuffix[ext] || ext == ".rb" || skUnsupported[ext] != "") && !strings.HasSuffix(p, ".d.ts")
	})
	if err != nil {
		return nil, 0, 0, err
	}
	read := map[string]bool{}
	for _, sr := range roots {
		read[sr.alias] = true
	}
	joins := map[string]TestFile{}
	for _, f := range m.Files {
		if !read[f.Anchor.Repo] {
			continue
		}
		if prev, ok := joins[f.Path]; ok && prev.Anchor.Repo != f.Anchor.Repo {
			return nil, 0, 0, &gokernel.Error{Code: "appmap-invalid-query", Message: fmt.Sprintf("map test file %s is joined in two roots (%s and %s)", f.Path, rootName(prev.Anchor.Repo), rootName(f.Anchor.Repo))}
		}
		joins[f.Path] = f
	}
	k := extractSiteKnowledge(in, joins, newScreenIndex(m.HashPrefix, m.BasePath, m.Screens))
	k.blobs = map[string]string{}
	for _, e := range entries {
		k.blobs[e.path] = e.oid
	}
	k.roots = map[string]*sourceRoot{}
	for _, sr := range roots {
		k.roots[sr.alias] = sr
	}
	return k, len(in), skipped, nil
}

// readSKSources reads the committed files under the declared sources that keep selects, within
// the site-knowledge file and byte bounds. It returns the text files read, every tree entry under
// the sources, the roots read and the count skipped as oversized or binary.
func readSKSources(ctx context.Context, m *Map, sources []string, o Options, keep func(string) bool) ([]skSource, []blobEntry, []*sourceRoot, int, error) {
	query := func(msg string) error { return &gokernel.Error{Code: "appmap-invalid-query", Message: msg} }
	if len(sources) == 0 || len(sources) > maxSKSources {
		return nil, nil, nil, 0, query(fmt.Sprintf("site knowledge requires 1 to %d --source directories", maxSKSources))
	}
	roots, err := openSources(ctx, m, sources, o)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	entries, err := sourceTree(roots, maxTestFiles)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	picked, skipped, total := []blobEntry{}, 0, 0
	for _, e := range entries {
		if !keep(e.path) {
			continue
		}
		if e.size > maxTestFileBytes {
			skipped++
			continue
		}
		picked = append(picked, e)
		total += e.size
	}
	if len(picked) > maxSKFiles {
		return nil, nil, nil, 0, bound(fmt.Sprintf("declared sources hold more than %d candidate files", maxSKFiles))
	}
	if total > maxTestBytes {
		return nil, nil, nil, 0, bound(fmt.Sprintf("declared source files exceed %d bytes", maxTestBytes))
	}
	in := []skSource{}
	for _, sr := range roots {
		mine := []blobEntry{}
		for _, e := range picked {
			if e.repo == sr.alias {
				mine = append(mine, e)
			}
		}
		data, err := sr.r.blobs(mine, maxTestFileBytes, maxTestBytes)
		if err != nil {
			return nil, nil, nil, 0, err
		}
		for _, e := range mine {
			if b := data[e.oid]; bytes.IndexByte(b, 0) < 0 {
				in = append(in, skSource{entry: e, data: b})
			} else {
				skipped++
			}
		}
	}
	return in, entries, roots, skipped, nil
}

func skHead(schema string, m *Map, sources []string, budget int, o Options, read, skipped int) []field {
	return []field{{"schema", schema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"authority", skAuthority}, {"limits", SKLimits}, {"sources", sources},
		{"files_read", read}, {"files_skipped", skipped}}
}

// PageObjects reports the page-object model of the declared sources (SKH-V0-010..014).
func PageObjects(ctx context.Context, m *Map, sources []string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	k, read, skipped, err := readSiteKnowledge(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	forms, methods, joined, effects := map[string]int{}, 0, 0, map[string]int{}
	joinReasons, effectReasons := map[string]int{}, map[string]int{}
	items := make([]any, len(k.PageObjects))
	for i, po := range k.PageObjects {
		forms[po.Form]++
		methods += len(po.Methods)
		if po.Join == StatusResolved {
			joined++
		} else {
			joinReasons[po.JoinReason]++
		}
		for _, mt := range po.Methods {
			effects[mt.Effect]++
			if mt.EffectReason != "" {
				effectReasons[mt.EffectReason]++
			}
		}
		items[i] = po
	}
	unknowns := make([]any, len(k.Unknowns))
	for i, u := range k.Unknowns {
		unknowns[i] = u
	}
	head := append(skHead(PageObjectsSchema, m, sources, budget, o, read, skipped),
		field{"page_objects_total", len(k.PageObjects)}, field{"forms", forms}, field{"methods", methods},
		field{"method_effects", effects}, field{"effect_reasons", effectReasons}, field{"screens_joined", joined},
		field{"join_reasons", joinReasons}, field{"unknowns_total", len(k.Unknowns)})
	return render(head, []section{{"page_objects", items}, {"unknowns", unknowns}}, budget)
}

// Setup reports the stage-3 setup records of the declared sources and links each test to the
// records that arrange it (SKH-V0-030..034, SKH-V0-100..105).
func Setup(ctx context.Context, m *Map, sources []string, o Options) ([]byte, error) {
	return SetupQuery(ctx, m, sources, nil, o)
}

// Setup query bounds (SKH-V0-103).
const (
	MaxSetupConditions = 16
	maxConditionBytes  = 256
)

// SetupQuery is Setup that also answers "what can produce condition C" for each condition, and
// links every flow precondition of the map to the records that can produce it (SKH-V0-031).
func SetupQuery(ctx context.Context, m *Map, sources, conditions []string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	if err := checkConditions(conditions); err != nil {
		return nil, err
	}
	k, read, skipped, err := readSiteKnowledge(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	records, uses, notRead, err := k.allSetup(repo{ctx: ctx, root: o.Root}, m, o.Repos)
	if err != nil {
		return nil, err
	}
	kinds, unknown, mocked := map[string]int{}, 0, 0
	items := make([]any, len(records))
	for i, s := range records {
		kinds[s.Kind]++
		if s.Status != StatusResolved {
			unknown++
		}
		if s.Mocked {
			mocked++
		}
		items[i] = s
	}
	linked, unresolved := 0, 0
	tests := make([]any, len(uses))
	for i, u := range uses {
		if len(u.Uses) > 0 {
			linked++
		}
		if len(u.Unresolved) > 0 {
			unresolved++
		}
		tests[i] = u
	}
	pre, preLinked := []any{}, 0
	for _, f := range m.Flows {
		for _, text := range f.Preconditions {
			c := conditionQuery(text, records)
			c.Flow = f.FlowID
			if c.Status == ConditionCandidates {
				preLinked++
			}
			pre = append(pre, c)
		}
	}
	asked := make([]any, len(conditions))
	for i, c := range conditions {
		asked[i] = conditionQuery(c, records)
	}
	head := append(skHead(SetupSchema, m, sources, budget, o, read, skipped),
		field{"records", len(records)}, field{"record_kinds", kinds}, field{"records_unknown", unknown},
		field{"records_mocked", mocked}, field{"tests_total", len(uses)}, field{"tests_linked", linked},
		field{"tests_unresolved_fixtures", unresolved}, field{"preconditions_total", len(pre)},
		field{"preconditions_linked", preLinked}, field{"not_read", notRead})
	return render(head, []section{{"conditions", asked}, {"preconditions", pre}, {"setup", items}, {"tests", tests}}, budget)
}

// checkConditions bounds the --condition values of a setup or how query (SKH-V0-103).
func checkConditions(conditions []string) error {
	if len(conditions) > MaxSetupConditions {
		return &gokernel.Error{Code: "appmap-invalid-query", Message: fmt.Sprintf("setup accepts at most %d --condition values", MaxSetupConditions)}
	}
	for _, c := range conditions {
		if strings.TrimSpace(c) == "" || len(c) > maxConditionBytes || strings.ContainsAny(c, "\x00\n") {
			return &gokernel.Error{Code: "appmap-invalid-query", Message: fmt.Sprintf("--condition must be 1 to %d bytes of one line", maxConditionBytes)}
		}
		if secretscreen.MatchString(c) {
			return &gokernel.Error{Code: "appmap-invalid-query", Message: "--condition holds secret-shaped text"}
		}
	}
	return nil
}

// allSetup is every stage-3 setup record, Playwright and Rails, with the tests each arranges.
func (k *SKModel) allSetup(r repo, m *Map, repos map[string]string) ([]SKSetup, []SKSetupUse, []SetupNotRead, error) {
	records, uses := k.extractSetup(k.blobs)
	rails, notRead, err := readRailsSetup(r, m, repos)
	if err != nil {
		return nil, nil, nil, err
	}
	records = append(records, rails...)
	return records, append(uses, k.linkRuby(records)...), notRead, nil
}

// Steps reports the test bodies of the declared sources in the closed step IR, with
// deduplicated reusable records (SKH-V0-020..023).
func Steps(ctx context.Context, m *Map, sources []string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	k, read, skipped, err := readSiteKnowledge(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	kinds, complete, reasons := map[string]int{}, 0, map[string]int{}
	tests := make([]any, len(k.Tests))
	for i, t := range k.Tests {
		if t.Complete {
			complete++
		}
		for _, s := range t.Steps {
			kinds[s.Kind]++
			if s.Kind == StepOpaque {
				reasons[s.Reason]++
			}
		}
		tests[i] = t
	}
	records := make([]any, len(k.Records))
	reused := 0
	for i, r := range k.Records {
		if r.Usage > 1 {
			reused++
		}
		records[i] = r
	}
	unknowns := make([]any, len(k.Unknowns))
	for i, u := range k.Unknowns {
		unknowns[i] = u
	}
	head := append(skHead(StepsSchema, m, sources, budget, o, read, skipped),
		field{"tests_total", len(k.Tests)}, field{"tests_complete", complete}, field{"step_kinds", kinds},
		field{"opaque_reasons", reasons}, field{"records_total", len(k.Records)}, field{"records_reused", reused},
		field{"unknowns_total", len(k.Unknowns)})
	return render(head, []section{{"records", records}, {"tests", tests}, {"unknowns", unknowns}}, budget)
}
