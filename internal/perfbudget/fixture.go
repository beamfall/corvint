// Package perfbudget generates the realistic-size read-budget fixture and measures the read
// budgets over it (performance-profiling-and-read-budget-v0.md, PRB-V0-020..036). The fixture
// is generated at run time, deterministically, and never committed.
package perfbudget

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// Scale sizes one fixture. Full is the realistic size GitHub #1602 asks for; Smoke is the same
// shape small enough for a unit test.
type Scale struct {
	Name string `json:"name"`
	// PrimaryPages are Markdown inputs the native provider derives records from.
	PrimaryPages int `json:"primary_pages"`
	// AdditionalRepositories each hold AdditionalPages inputs an adapter provider anchors.
	AdditionalRepositories int `json:"additional_repositories"`
	AdditionalPages        int `json:"additional_pages"`
	// ParagraphsPerPage sets the native records per primary page.
	ParagraphsPerPage int `json:"paragraphs_per_page"`
	// ClaimsPerPage is the adapter claims per additional page, beside one subject.
	ClaimsPerPage int `json:"claims_per_page"`
	// ChangedPages are primary pages edited between the previous and current artifacts.
	ChangedPages int `json:"changed_pages"`
	// RubyFiles is the size of the Ruby tree affected --path plans over.
	RubyFiles int `json:"ruby_files"`
	// GoPackages and TSModules size the Go and TypeScript code repository the index, context,
	// lifecycle-hook and Go/TS affected scenarios read (PRB-V0-058); LongLineBytes is the length
	// of its adversarial single-line Go and TypeScript declarations.
	GoPackages    int `json:"go_packages"`
	TSModules     int `json:"ts_modules"`
	LongLineBytes int `json:"long_line_bytes"`
}

var (
	// Full: 7,000 + 3 x 1,000 = 10,000 inputs and about 96,000 corpus records, under the
	// 100,000-record artifact bound (doccorpus.MaxCorpusRecords).
	Full  = Scale{Name: "full", PrimaryPages: 7000, AdditionalRepositories: 3, AdditionalPages: 1000, ParagraphsPerPage: 6, ClaimsPerPage: 10, ChangedPages: 350, RubyFiles: 20000, GoPackages: 1500, TSModules: 3000, LongLineBytes: 1 << 20}
	Smoke = Scale{Name: "smoke", PrimaryPages: 40, AdditionalRepositories: 3, AdditionalPages: 10, ParagraphsPerPage: 6, ClaimsPerPage: 10, ChangedPages: 4, RubyFiles: 60, GoPackages: 12, TSModules: 16, LongLineBytes: 16 << 10}
)

// FixtureGenerator versions the generator; a reused fixture from another version is refused.
const FixtureGenerator = "corvint-perf-budget-fixture/2"

// Fixture is a generated fixture. Paths are absolute; artifacts are relative to Primary.
type Fixture struct {
	Generator  string   `json:"generator"`
	Scale      Scale    `json:"scale"`
	Primary    string   `json:"-"`
	Additional []string `json:"-"`
	Ruby       string   `json:"-"`
	Code       string   `json:"-"`
	// PreviousRevision and Revision are the primary docs commits the two artifacts pin.
	PreviousRevision string `json:"previous_revision"`
	Revision         string `json:"revision"`
	Inputs           int    `json:"inputs"`
	Records          int    `json:"records"`
	ArtifactBytes    int    `json:"artifact_bytes"`
	// Previous and Current are the corpus artifacts, built from PreviousManifest and Manifest.
	Previous         string `json:"previous"`
	PreviousManifest string `json:"previous_manifest"`
	Current          string `json:"current"`
	Manifest         string `json:"manifest"`
	// BuilderSHA256 identifies the corvint binary that built the artifacts; a different binary
	// rebuilds them from the manifests before measuring.
	BuilderSHA256 string `json:"builder_sha256"`
	// SearchQuery, GetID, RubyPath name the reads every scenario issues.
	SearchQuery string `json:"search_query"`
	GetID       string `json:"get_id"`
	RubyPath    string `json:"ruby_path"`
	// GoPath and TSPath are the code repository paths the Go and TypeScript affected scenarios
	// plan over; ContextTask is the context query and hook prompt over that repository.
	GoPath      string `json:"go_path"`
	TSPath      string `json:"ts_path"`
	ContextTask string `json:"context_task"`
}

const (
	fixtureTimestamp = "2026-10-23T00:00:00Z"
	fixtureGitDate   = "2026-10-23T00:00:00Z"
)

var fixtureWords = strings.Fields("request response handler widget order invoice account payment customer session token cache queue worker schedule report export import validate render template signal metric trace span event stream batch record shard index query result filter route policy grant audit ledger journal")

func word(i int) string { return fixtureWords[i%len(fixtureWords)] }

// Generate writes a fixture of scale s under dir, which must be empty or absent. Identical
// scales produce byte-identical repositories, commits and artifacts.
func Generate(ctx context.Context, dir string, s Scale, build Builder) (*Fixture, error) {
	if s.PrimaryPages < 1 || s.AdditionalRepositories < 1 || s.AdditionalRepositories > 7 || s.AdditionalPages < 1 || s.ChangedPages > s.PrimaryPages || s.RubyFiles < 2 || s.GoPackages < 4 || s.TSModules < 4 || s.LongLineBytes < 1024 {
		return nil, fmt.Errorf("perf-budget fixture: invalid scale %+v", s)
	}
	// Generation proceeds only into a directory proved absent or empty; any other inspection
	// failure (an unreadable directory may hold paths generation would overwrite) refuses.
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("perf-budget fixture: cannot prove %s empty: %w", dir, err)
	case len(entries) > 0:
		return nil, fmt.Errorf("perf-budget fixture: %s is not empty", dir)
	}
	f := &Fixture{Generator: FixtureGenerator, Scale: s, Primary: filepath.Join(dir, "app"), Ruby: filepath.Join(dir, "ruby-app"), Previous: "previous.json", PreviousManifest: "previous-manifest.json", Current: "corpus.json", Manifest: "manifest.json", SearchQuery: "invoice ledger", GetID: "adapter-lib1:lib1-0", RubyPath: "app/models/model_00001.rb", Code: filepath.Join(dir, "code-app"), GoPath: codeGoPath, TSPath: codeTSPath, ContextTask: codeContextTask}
	// Primary pages, first revision.
	for n := range s.PrimaryPages {
		if err := writeFile(filepath.Join(f.Primary, "docs", fmt.Sprintf("area%02d", n%50), fmt.Sprintf("page%05d.md", n)), primaryPage(n, s.ParagraphsPerPage, false)); err != nil {
			return nil, err
		}
	}
	if err := writeFile(filepath.Join(f.Primary, ".gitignore"), []byte("/previous.json\n/previous-manifest.json\n/corpus.json\n/manifest.json\n/.corvint/\n")); err != nil {
		return nil, err
	}
	if err := commitAll(ctx, f.Primary, "docs"); err != nil {
		return nil, err
	}
	docsRevision, err := gitOutput(ctx, f.Primary, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	// Additional repositories and the adapter provider records that anchor their pages.
	type extra struct {
		name, dir, id string
		record        doccorpus.ProviderRecord
		inputs        []doccorpus.Input
	}
	extras := []extra{}
	for r := 1; r <= s.AdditionalRepositories; r++ {
		e := extra{name: fmt.Sprintf("lib%d", r), dir: filepath.Join(dir, fmt.Sprintf("lib%d", r))}
		for n := range s.AdditionalPages {
			if err := writeFile(filepath.Join(e.dir, "guide", fmt.Sprintf("p%05d.md", n)), additionalPage(e.name, n, s.ClaimsPerPage)); err != nil {
				return nil, err
			}
		}
		if err := commitAll(ctx, e.dir, e.name); err != nil {
			return nil, err
		}
		if e.id, err = gitOutput(ctx, e.dir, "rev-parse", "HEAD"); err != nil {
			return nil, err
		}
		f.Additional = append(f.Additional, e.dir)
		extras = append(extras, e)
	}
	for i := range extras {
		e := &extras[i]
		e.record = doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "adapter-" + e.name, Version: "1", Subjects: []doccorpus.Subject{}, Claims: []doccorpus.Claim{}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{},
			Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "explicit provider subjects"}, {Name: "claims", State: "present", Reason: "explicit provider claims"}}}
		for n := range s.AdditionalPages {
			path := fmt.Sprintf("guide/p%05d.md", n)
			raw := additionalPage(e.name, n, s.ClaimsPerPage)
			blob, digest := blobID(raw), doccorpus.Digest(raw)
			e.inputs = append(e.inputs, doccorpus.Input{Path: path, Revision: e.id, Blob: blob, SHA256: digest, Provider: e.record.ID, Purpose: "evidence"})
			lines := strings.SplitAfter(string(raw), "\n")
			evidence := func(line int) doccorpus.Evidence {
				return doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Limitations: []string{"provider declaration; structural anchor does not prove behavior"},
					Anchors: []doccorpus.Anchor{{Repository: e.id, Revision: e.id, Path: path, Blob: blob, SHA256: digest, Start: line, End: line, SpanSHA256: doccorpus.Digest([]byte(lines[line-1])), Authority: "external-provider", Kind: "declared", Reason: "generated perf-budget fixture declaration"}}}
			}
			subject := fmt.Sprintf("%s:%s-%d", e.record.ID, e.name, n)
			e.record.Subjects = append(e.record.Subjects, doccorpus.Subject{ID: subject, Kind: "capability", Name: fmt.Sprintf("%s %s %s %d", e.name, word(n), word(n/7), n), Provider: e.record.ID, Evidence: evidence(1)})
			for c := range s.ClaimsPerPage {
				e.record.Claims = append(e.record.Claims, doccorpus.Claim{ID: fmt.Sprintf("%s-claim-%02d", subject, c), Subject: subject, Text: strings.TrimSpace(strings.TrimPrefix(lines[2+c], "- ")), Provider: e.record.ID, Evidence: evidence(3 + c)})
			}
		}
	}
	// publish commits the provider records for one docs revision: a record's source is the
	// manifest repository identity, which names that docs commit.
	publish := func(docs string) (string, error) {
		m, err := doccorpus.InventoryScopes(ctx, f.Primary, docs, []string{"docs"}, doccorpus.ManifestSchemaV2, fixtureTimestamp)
		if err != nil {
			return "", fmt.Errorf("perf-budget fixture: inventory at %s: %w", docs, err)
		}
		for i := range extras {
			extras[i].record.Source = m.Repository
			data, err := doccorpus.Encode(extras[i].record)
			if err != nil {
				return "", fmt.Errorf("perf-budget fixture: provider %s: %w", extras[i].record.ID, err)
			}
			if err := writeFile(filepath.Join(f.Primary, "evidence", extras[i].name+".json"), data); err != nil {
				return "", err
			}
		}
		if err := commitAll(ctx, f.Primary, "providers for "+docs); err != nil {
			return "", err
		}
		return gitOutput(ctx, f.Primary, "rev-parse", "HEAD")
	}
	manifestAt := func(docs, revision string) (doccorpus.Manifest, error) {
		m, err := doccorpus.InventoryScopes(ctx, f.Primary, docs, []string{"docs"}, doccorpus.ManifestSchemaV2, fixtureTimestamp)
		if err != nil {
			return m, err
		}
		additional := []doccorpus.AdditionalRepository{}
		for _, e := range extras {
			additional = append(additional, doccorpus.AdditionalRepository{ID: e.id, Revision: e.id, Root: "../" + e.name})
			m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "guide", Revision: e.id})
			m.Inputs = append(m.Inputs, e.inputs...)
			record := "evidence/" + e.name + ".json"
			blob, err := gitOutput(ctx, f.Primary, "rev-parse", revision+":"+record)
			if err != nil {
				return m, err
			}
			data, err := os.ReadFile(filepath.Join(f.Primary, record))
			if err != nil {
				return m, err
			}
			m.Providers = append(m.Providers, doccorpus.Provider{ID: e.record.ID, Kind: "records", Version: "1", Revision: revision, Record: record})
			m.Scopes = append(m.Scopes, doccorpus.Scope{Path: record, Revision: revision})
			m.Inputs = append(m.Inputs, doccorpus.Input{Path: record, Revision: revision, Blob: blob, SHA256: doccorpus.Digest(data), Provider: e.record.ID, Purpose: "provider"})
		}
		slices.SortFunc(additional, func(a, b doccorpus.AdditionalRepository) int { return strings.Compare(a.ID, b.ID) })
		m.AdditionalRepositories = additional
		return m, nil
	}
	f.PreviousRevision = docsRevision
	providers, err := publish(docsRevision)
	if err != nil {
		return nil, err
	}
	if _, err := buildArtifact(ctx, f.Primary, manifestAt, build, docsRevision, providers, f.Previous, f.PreviousManifest); err != nil {
		return nil, err
	}
	// Second revision: ChangedPages primary pages change; the worklist sees them.
	for n := range s.ChangedPages {
		page := n * (s.PrimaryPages / max(s.ChangedPages, 1))
		if err := writeFile(filepath.Join(f.Primary, "docs", fmt.Sprintf("area%02d", page%50), fmt.Sprintf("page%05d.md", page)), primaryPage(page, s.ParagraphsPerPage, true)); err != nil {
			return nil, err
		}
	}
	if err := commitAll(ctx, f.Primary, "edit"); err != nil {
		return nil, err
	}
	if f.Revision, err = gitOutput(ctx, f.Primary, "rev-parse", "HEAD"); err != nil {
		return nil, err
	}
	if providers, err = publish(f.Revision); err != nil {
		return nil, err
	}
	records, err := buildArtifact(ctx, f.Primary, manifestAt, build, f.Revision, providers, f.Current, f.Manifest)
	if err != nil {
		return nil, err
	}
	f.Inputs = s.PrimaryPages + s.AdditionalRepositories*s.AdditionalPages
	f.Records = records
	if info, err := os.Stat(filepath.Join(f.Primary, f.Current)); err == nil {
		f.ArtifactBytes = int(info.Size())
	}
	if err := generateRuby(ctx, f.Ruby, s.RubyFiles); err != nil {
		return nil, err
	}
	if err := generateCode(ctx, f.Code, s); err != nil {
		return nil, err
	}
	return f, nil
}

// Builder turns the manifest written at root/manifestPath (m in memory) into artifact bytes.
// The measured corvint binary must build the artifacts it reads: a read rederives the artifact
// and refuses one whose builder identity differs.
type Builder func(ctx context.Context, root string, m doccorpus.Manifest, manifestPath string) ([]byte, error)

// CLIBuilder builds with `corvint docs corpus build`.
func CLIBuilder(corvint string) Builder {
	return func(ctx context.Context, root string, _ doccorpus.Manifest, manifestPath string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, corvint, "--root", root, "docs", "corpus", "build", "--manifest", manifestPath)
		cmd.Env = measuredEnv()
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := outputContained(cmd)
		if err != nil {
			return nil, fmt.Errorf("corvint docs corpus build: %v: %.400s", err, stderr.String())
		}
		return out, nil
	}
}

// LibraryBuilder builds in process; its artifacts carry this binary's builder identity.
func LibraryBuilder(ctx context.Context, root string, m doccorpus.Manifest, _ string) ([]byte, error) {
	a, err := doccorpus.Build(ctx, root, m)
	if err != nil {
		return nil, err
	}
	return doccorpus.Encode(a)
}

// buildArtifact writes the manifest, builds the artifact from it and returns its record count.
func buildArtifact(ctx context.Context, root string, manifestAt func(string, string) (doccorpus.Manifest, error), build Builder, docs, revision, artifact, manifest string) (int, error) {
	m, err := manifestAt(docs, revision)
	if err != nil {
		return 0, fmt.Errorf("perf-budget fixture: manifest at %s: %w", docs, err)
	}
	data, err := doccorpus.Encode(m)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(root, manifest), data, 0o600); err != nil {
		return 0, err
	}
	return rebuild(ctx, root, m, build, manifest, artifact)
}

func rebuild(ctx context.Context, root string, m doccorpus.Manifest, build Builder, manifest, artifact string) (int, error) {
	data, err := build(ctx, root, m, manifest)
	if err != nil {
		return 0, fmt.Errorf("perf-budget fixture: build %s: %w", artifact, err)
	}
	var counted struct {
		Subjects, Claims, Relations, Journeys, Observations []json.RawMessage
	}
	if err := json.Unmarshal(data, &counted); err != nil {
		return 0, fmt.Errorf("perf-budget fixture: artifact %s: %w", artifact, err)
	}
	if err := os.WriteFile(filepath.Join(root, artifact), data, 0o600); err != nil {
		return 0, err
	}
	return len(counted.Subjects) + len(counted.Claims) + len(counted.Relations) + len(counted.Journeys) + len(counted.Observations), nil
}

func primaryPage(n, paragraphs int, edited bool) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s page %d\n", strings.ToUpper(word(n)[:1])+word(n)[1:], word(n/3), n)
	for p := range paragraphs {
		b.WriteString("\n")
		for w := range 18 {
			if w > 0 {
				b.WriteString(" ")
			}
			b.WriteString(word(n*7 + p*11 + w*5))
		}
		fmt.Fprintf(&b, " item%d.%d.\n", n, p)
	}
	if edited {
		fmt.Fprintf(&b, "\nRevised %s guidance for page %d.\n", word(n+1), n)
	}
	return []byte(b.String())
}

func additionalPage(name string, n, claims int) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s %d\n\n", name, word(n), n)
	for c := range claims {
		fmt.Fprintf(&b, "- The %s %s %s step %d.%d.\n", name, word(n+c), word(n*3+c*5), n, c)
	}
	return []byte(b.String())
}

// generateRuby writes a Rails-shaped Ruby tree of about files files: models, a service per
// model that requires it, and a spec per model.
func generateRuby(ctx context.Context, root string, files int) error {
	models := max(files/3, 1)
	if err := writeFile(filepath.Join(root, "Gemfile"), []byte("source \"https://rubygems.org\"\ngem \"rails\"\ngem \"rspec-rails\"\n")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "app", "models", "application_record.rb"), []byte("class ApplicationRecord\nend\n")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "spec", "spec_helper.rb"), []byte("require \"rspec\"\n")); err != nil {
		return err
	}
	for n := 1; n <= models; n++ {
		model := fmt.Sprintf("model_%05d", n)
		class := fmt.Sprintf("Model%05d", n)
		body := fmt.Sprintf("require_relative \"application_record\"\n\nclass %s < ApplicationRecord\n  def %s\n    \"%s\"\n  end\nend\n", class, word(n), word(n/3))
		if n > 1 {
			body = fmt.Sprintf("require_relative \"model_%05d\"\n", n-1) + body
		}
		if err := writeFile(filepath.Join(root, "app", "models", model+".rb"), []byte(body)); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(root, "app", "services", model+"_service.rb"), fmt.Appendf(nil, "require_relative \"../models/%s\"\n\nclass %sService\n  def call\n    %s.new\n  end\nend\n", model, class, class)); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(root, "spec", "models", model+"_spec.rb"), fmt.Appendf(nil, "require \"spec_helper\"\nrequire_relative \"../../app/models/%s\"\n\nRSpec.describe %s do\n  it \"answers\" do\n    expect(%s.new.%s).to eq(\"%s\")\n  end\nend\n", model, class, class, word(n), word(n/3))); err != nil {
			return err
		}
	}
	return commitAll(ctx, root, "ruby")
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// gitCommand runs Git without user or system configuration and with fixed identities and dates,
// so commit IDs depend only on content.
func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	base := []string{"-C", dir, "-c", "user.name=perf-budget", "-c", "user.email=perf-budget@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}
	c := exec.CommandContext(ctx, "git", append(base, args...)...)
	env := []string{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	c.Env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_DATE="+fixtureGitDate, "GIT_COMMITTER_DATE="+fixtureGitDate)
	return c
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := outputContained(gitCommand(ctx, dir, args...))
	if err != nil {
		return "", fmt.Errorf("perf-budget fixture: git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func commitAll(ctx context.Context, dir, message string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if out, err := combinedContained(gitCommand(ctx, dir, "init", "-q")); err != nil {
			return fmt.Errorf("perf-budget fixture: git init %s: %v %s", dir, err, out)
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		if out, err := combinedContained(gitCommand(ctx, dir, args...)); err != nil {
			return fmt.Errorf("perf-budget fixture: git %s: %v %s", strings.Join(args, " "), err, out)
		}
	}
	return nil
}

func blobID(raw []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(append(fmt.Appendf(nil, "blob %d\x00", len(raw)), raw...)))
}
