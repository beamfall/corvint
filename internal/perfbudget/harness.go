package perfbudget

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// ReportSchema names the harness report.
const ReportSchema = "corvint-perf-budget-report/0"

// Options configures one harness run.
type Options struct {
	Budgets   *Budgets
	Corvint   string // corvint binary
	CorpusMCP string // corvint-corpus-mcp binary
	// FixtureDir holds a fixture: reused when it carries fixture.json, generated into otherwise.
	FixtureDir string
	Scale      Scale
	// Revision is the Corvint commit under measurement, recorded as given.
	Revision string
	// Progress, when set, receives one line per phase.
	Progress func(string)
	// LoadAverage observes the host load around the run; nil reads the host's own.
	LoadAverage LoadFunc
	// Selected, when non-empty, names the only ScenarioIDs the run measures and judges
	// (PRB-V0-970).
	Selected []string
}

// Report is one harness run.
type Report struct {
	Schema    string             `json:"schema"`
	Revision  string             `json:"revision"`
	StartedAt string             `json:"started_at"`
	Fixture   *Fixture           `json:"fixture"`
	Host      Host               `json:"host"`
	Scenarios []ScenarioMeasured `json:"scenarios"`
	// Selected lists the scenarios a filtered run measured; it is absent for a whole run
	// (PRB-V0-970).
	Selected []string `json:"selected,omitempty"`
	Checks   []Check  `json:"checks"`
	// Pending lists every pending target, so a reader sees the goals not yet enforced.
	Pending []Check `json:"pending"`
	// Advisory lists every advisory target, measured and never enforced (PRB-V0-056).
	Advisory []Check `json:"advisory"`
	// Verdict is PASS, FAIL or NOT_JUDGED (PRB-V0-038); OK is true only for PASS.
	Verdict         string `json:"verdict"`
	NotJudgedReason string `json:"not_judged_reason,omitempty"`
	OK              bool   `json:"ok"`
}

const fixtureRecord = "fixture.json"

// LoadOrGenerate reuses the fixture in dir when its record matches scale s, rebuilding its two
// artifacts when builderSHA256 names a different builder, or generates one.
func LoadOrGenerate(ctx context.Context, dir string, s Scale, build Builder, builderSHA256 string) (*Fixture, error) {
	if data, err := os.ReadFile(filepath.Join(dir, fixtureRecord)); err == nil {
		var f Fixture
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("perf-budget fixture record: %w", err)
		}
		if f.Generator != FixtureGenerator {
			return nil, fmt.Errorf("perf-budget fixture in %s was made by generator %q, not %q; remove it", dir, f.Generator, FixtureGenerator)
		}
		if f.Scale != s {
			return nil, fmt.Errorf("perf-budget fixture in %s has scale %q, want %q; remove it or choose another directory", dir, f.Scale.Name, s.Name)
		}
		f.locate(dir)
		if f.BuilderSHA256 == builderSHA256 {
			return &f, nil
		}
		for _, pair := range [][2]string{{f.PreviousManifest, f.Previous}, {f.Manifest, f.Current}} {
			raw, err := os.ReadFile(filepath.Join(f.Primary, pair[0]))
			if err != nil {
				return nil, err
			}
			var m doccorpus.Manifest
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, fmt.Errorf("perf-budget fixture manifest %s: %w", pair[0], err)
			}
			records, err := rebuild(ctx, f.Primary, m, build, pair[0], pair[1])
			if err != nil {
				return nil, err
			}
			if pair[1] == f.Current {
				f.Records = records
			}
		}
		f.BuilderSHA256 = builderSHA256
		return &f, writeFixtureRecord(dir, &f)
	}
	f, err := Generate(ctx, dir, s, build)
	if err != nil {
		return nil, err
	}
	f.BuilderSHA256 = builderSHA256
	return f, writeFixtureRecord(dir, f)
}

func writeFixtureRecord(dir string, f *Fixture) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fixtureRecord), append(data, '\n'), 0o600)
}

// FileSHA256 is the hex SHA-256 of one file, the builder identity of a corvint binary.
func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (f *Fixture) locate(dir string) {
	f.Primary, f.Ruby, f.Code, f.Additional = filepath.Join(dir, "app"), filepath.Join(dir, "ruby-app"), filepath.Join(dir, "code-app"), nil
	for r := 1; r <= f.Scale.AdditionalRepositories; r++ {
		f.Additional = append(f.Additional, filepath.Join(dir, fmt.Sprintf("lib%d", r)))
	}
}

// Run measures every scenario over the fixture and evaluates the budgets.
func Run(ctx context.Context, o Options) (*Report, error) {
	if err := o.Budgets.Validate(); err != nil {
		return nil, err
	}
	progress := o.Progress
	if progress == nil {
		progress = func(string) {}
	}
	r := &Report{Schema: ReportSchema, Revision: o.Revision, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	progress("fixture: " + o.Scale.Name + " in " + o.FixtureDir)
	builder, err := FileSHA256(o.Corvint)
	if err != nil {
		return nil, err
	}
	fixture, err := LoadOrGenerate(ctx, o.FixtureDir, o.Scale, CLIBuilder(o.Corvint), builder)
	if err != nil {
		return nil, err
	}
	r.Fixture = fixture
	progress(fmt.Sprintf("fixture: %d inputs, %d records, %d additional repositories", fixture.Inputs, fixture.Records, len(fixture.Additional)))
	load := o.LoadAverage
	if load == nil {
		load = loadAverage
	}
	r.Host = observeHost(ctx, load)
	samples := map[string]int{}
	for _, s := range o.Budgets.Scenarios {
		samples[s.ID] = s.Samples
	}
	search := []string{o.Corvint, "--root", fixture.Primary, "docs", "corpus", "search", "--artifact", fixture.Current, "--query", fixture.SearchQuery}
	selected := func(id string) bool { return len(o.Selected) == 0 || slices.Contains(o.Selected, id) }
	r.Selected = slices.Clone(o.Selected)
	// context-query and lifecycle-hook measure reads over an existing index snapshot, which the
	// index scenarios leave behind in a whole run; a filtered run that skips them builds it once,
	// unmeasured, first (PRB-V0-970).
	indexed := false
	for _, id := range ScenarioIDs {
		if !selected(id) && (id != "mcp-first-call" || !selected("mcp-warm-call")) {
			continue
		}
		if (id == "context-query" || id == "lifecycle-hook") && !indexed {
			progress("setup: index snapshot for " + id)
			if _, err := runOnce(ctx, fixture.Code, []string{o.Corvint, "--root", fixture.Code, "index", "--if-stale"}, nil); err != nil {
				return nil, fmt.Errorf("index setup for %s: %w", id, err)
			}
			indexed = true
		}
		if id == "index-cold-build" || id == "index-warm-build" {
			indexed = true
		}
		progress("scenario: " + id)
		var m ScenarioMeasured
		var err error
		switch id {
		case "cli-cold-read":
			m, err = measureCLI(ctx, id, fixture.Primary, search, samples[id], 0, func() error { return os.RemoveAll(filepath.Join(fixture.Primary, ".corvint")) })
		case "cli-warm-read":
			m, err = measureCLI(ctx, id, fixture.Primary, search, samples[id], 1, nil)
		case "mcp-first-call":
			first, warm, err := measureMCP(ctx, o.CorpusMCP, fixture, samples["mcp-first-call"], samples["mcp-warm-call"])
			if err != nil {
				return nil, err
			}
			for _, m := range []ScenarioMeasured{first, warm} {
				if selected(m.ID) {
					r.Scenarios = append(r.Scenarios, m)
				}
			}
			continue
		case "mcp-warm-call":
			continue
		case "full-delta":
			m, err = measureCLI(ctx, id, fixture.Primary, []string{o.Corvint, "--root", fixture.Primary, "docs", "corpus", "delta", "--previous", fixture.Previous, "--artifact", fixture.Current, "--limit", "256"}, samples[id], 0, nil)
		case "affected-path":
			m, err = measureCLI(ctx, id, fixture.Ruby, []string{o.Corvint, "--root", fixture.Ruby, "affected", "--path", fixture.RubyPath}, samples[id], 1, nil)
		case "unchanged-delta":
			// The current artifact against itself: one verified read plus the stream (PRB-V0-050).
			m, err = measureCLI(ctx, id, fixture.Primary, []string{o.Corvint, "--root", fixture.Primary, "docs", "corpus", "delta", "--previous", fixture.Current, "--artifact", fixture.Current, "--format", "jsonl"}, samples[id], 0, nil)
		case "index-cold-build":
			// Every sample builds the snapshot of the code repository from nothing (PRB-V0-057).
			m, err = measureCLI(ctx, id, fixture.Code, []string{o.Corvint, "--root", fixture.Code, "index"}, samples[id], 0, func() error { return removeIndexState(fixture.Code) })
		case "index-warm-build":
			// A matching snapshot exists: the freshness check every hook and packet verb pays.
			m, err = measureCLI(ctx, id, fixture.Code, []string{o.Corvint, "--root", fixture.Code, "index", "--if-stale"}, samples[id], 1, nil)
		case "context-query":
			m, err = measureCLI(ctx, id, fixture.Code, []string{o.Corvint, "--root", fixture.Code, "context", "--task", fixture.ContextTask, "--subject", fixture.GoPath, "--lsp", "off"}, samples[id], 1, nil)
		case "lifecycle-hook":
			m, err = measureHook(ctx, id, o.Corvint, fixture, samples[id])
		case "affected-go":
			m, err = measureCLI(ctx, id, fixture.Code, []string{o.Corvint, "--root", fixture.Code, "affected", "--path", fixture.GoPath}, samples[id], 1, nil)
		case "affected-ts":
			m, err = measureCLI(ctx, id, fixture.Code, []string{o.Corvint, "--root", fixture.Code, "affected", "--path", fixture.TSPath}, samples[id], 1, nil)
		}
		if err != nil {
			return nil, err
		}
		r.Scenarios = append(r.Scenarios, m)
	}
	r.Host.finish(ctx, load)
	judged := o.Budgets
	if len(o.Selected) != 0 {
		filtered := *o.Budgets
		filtered.Scenarios = nil
		for _, b := range o.Budgets.Scenarios {
			if selected(b.ID) {
				filtered.Scenarios = append(filtered.Scenarios, b)
			}
		}
		judged = &filtered
	}
	r.Checks, r.Verdict, r.NotJudgedReason = Judge(judged, r.Host, r.Scenarios)
	r.OK = r.Verdict == RunPass
	r.Pending, r.Advisory = unenforced(r.Checks)
	return r, nil
}

// unenforced lists the pending and the advisory checks, each non-nil (PRB-V0-056).
func unenforced(checks []Check) (pending, advisory []Check) {
	pending, advisory = []Check{}, []Check{}
	for _, c := range checks {
		switch c.Limit.Status {
		case StatusPending:
			pending = append(pending, c)
		case StatusAdvisory:
			advisory = append(advisory, c)
		}
	}
	return pending, advisory
}

// measureCLI runs warmups unmeasured, then samples measured runs; before runs ahead of each. A
// run that executed and refused is the scenario's error; an executable that could not start is
// returned as the harness's *StartError.
func measureCLI(ctx context.Context, id, dir string, argv []string, samples, warmups int, before func() error) (ScenarioMeasured, error) {
	return measureCLIInput(ctx, id, dir, argv, samples, warmups, before, nil)
}

// measureHook times the Claude Code UserPromptSubmit lifecycle hook over the code repository: the
// host's event payload on stdin, the project directory in the host's variable, and the hook's
// own adapter argv (PRB-V0-057). One unmeasured warmup lets the hook find the index snapshot.
func measureHook(ctx context.Context, id, corvint string, f *Fixture, samples int) (ScenarioMeasured, error) {
	payload, err := json.Marshal(map[string]any{"session_id": "corvint-perf-budget", "cwd": f.Code, "hook_event_name": "UserPromptSubmit", "prompt": f.ContextTask})
	if err != nil {
		return ScenarioMeasured{}, err
	}
	in := &input{stdin: payload, env: []string{"CLAUDE_PROJECT_DIR=" + f.Code}}
	return measureCLIInput(ctx, id, f.Code, []string{corvint, "adapter", "claude-code", "user-prompt"}, samples, 1, nil, in)
}

// removeIndexState removes the index snapshots of root: the shared store under the Git common
// directory and the worktree's own legacy store.
func removeIndexState(root string) error {
	for _, dir := range []string{filepath.Join(root, ".git", "corvint"), filepath.Join(root, ".corvint")} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}

// measureCLIInput is measureCLI with in, when set, supplied to every run.
func measureCLIInput(ctx context.Context, id, dir string, argv []string, samples, warmups int, before func() error, in *input) (ScenarioMeasured, error) {
	m := ScenarioMeasured{ID: id, Command: displayCommand(argv), Samples: []Sample{}}
	for i := range warmups + samples {
		if before != nil {
			if err := before(); err != nil {
				m.Error = err.Error()
				return m, nil
			}
		}
		sample, err := runOnce(ctx, dir, argv, in)
		var notStarted *StartError
		if errors.As(err, &notStarted) {
			return m, err
		}
		if err != nil {
			m.Error = err.Error()
			return m, nil
		}
		if i >= warmups {
			m.Samples = append(m.Samples, sample)
		}
	}
	m.summarize()
	return m, nil
}

// measureMCP starts firstSamples corpus MCP servers. Each first call is timed from process start
// to its response; warmSamples further calls on the first server, alternating search and get,
// are timed per call. Each scenario's peak resident set is its servers' maximum. A server that
// started and refused fails both scenarios; one that could not start is returned as the
// harness's *StartError (PRB-V0-029).
func measureMCP(ctx context.Context, binary string, f *Fixture, firstSamples, warmSamples int) (ScenarioMeasured, ScenarioMeasured, error) {
	argv := []string{binary, "--root", f.Primary, "--artifact", f.Current, "--protocol-version", mcpProtocolVersion}
	first := ScenarioMeasured{ID: "mcp-first-call", Command: displayCommand(argv), Samples: []Sample{}}
	warm := ScenarioMeasured{ID: "mcp-warm-call", Command: displayCommand(argv), Samples: []Sample{}}
	failBoth := func(err error) (ScenarioMeasured, ScenarioMeasured, error) {
		first.Error, warm.Error = err.Error(), err.Error()
		return first, warm, nil
	}
	for i := range firstSamples {
		session, start, err := startMCP(ctx, f.Primary, argv)
		var notStarted *StartError
		if errors.As(err, &notStarted) {
			return first, warm, err
		}
		if err != nil {
			return failBoth(err)
		}
		if _, err := session.call("corvint.docs_search", map[string]any{"query": f.SearchQuery}); err != nil {
			return failBoth(session.fail(err))
		}
		firstWall := time.Since(start)
		warmWalls := []time.Duration{}
		if i == 0 {
			for c := range warmSamples {
				tool, arguments := "corvint.docs_search", map[string]any{"query": f.SearchQuery}
				if c%2 == 1 {
					tool, arguments = "corvint.docs_get", map[string]any{"id": f.GetID}
				}
				wall, err := session.call(tool, arguments)
				if err != nil {
					warm.Error = session.fail(err).Error()
					first.Error = warm.Error
					return first, warm, nil
				}
				warmWalls = append(warmWalls, wall)
			}
		}
		rss, err := session.close()
		if err != nil {
			return failBoth(err)
		}
		first.Samples = append(first.Samples, Sample{WallMS: ms(firstWall), PeakRSS: rss})
		for _, wall := range warmWalls {
			warm.Samples = append(warm.Samples, Sample{WallMS: ms(wall), PeakRSS: rss})
		}
	}
	first.summarize()
	warm.summarize()
	return first, warm, nil
}

// displayCommand names the binary by its base name so reports do not carry host paths.
func displayCommand(argv []string) []string {
	out := append([]string{filepath.Base(argv[0])}, argv[1:]...)
	for i := range out {
		if filepath.IsAbs(out[i]) {
			out[i] = "<fixture>/" + filepath.Base(out[i])
		}
	}
	return out
}
