package doccorpus

import (
	"bytes"
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PlaywrightDiscoverySchema and PlaywrightDiscoveryMode name the live discovery record the
// behavior adapter requires.
const (
	PlaywrightDiscoverySchema = "corvint-playwright-discovery/1"
	PlaywrightDiscoveryMode   = "live-playwright-list"
)

// PlaywrightFileMaxBytes bounds a caller-run Playwright listing or report the producers read.
const PlaywrightFileMaxBytes = typescript.PlaywrightListMaxBytes

// PlaywrightDiscoveryInput is one discovery production: the schema-2 migration record that fixes
// the revision set, the repository-relative Playwright config and the caller-run
// `playwright test --list --reporter=json` report. Receipt is an optional qualified Playwright
// receipt whose test identities the record adopts for exactly matching executions.
type PlaywrightDiscoveryInput struct {
	Root       string
	Migration  []byte
	ConfigPath string
	Listing    []byte
	Receipt    []byte
}

// BuildPlaywrightDiscovery converts a caller-run Playwright listing into the canonical
// corvint-playwright-discovery/1 live-playwright-list record (DCP-V1-046..048). It never runs
// Playwright. Config and test anchors are full Git identities at the migration's source
// revision; the working-tree bytes the listing was taken from must equal them.
func BuildPlaywrightDiscovery(ctx context.Context, input PlaywrightDiscoveryInput) ([]byte, error) {
	var migration BehaviorMigration
	if err := decode(input.Migration, &migration); err != nil || migration.Schema != 2 || !textOK(migration.ContractID) || !validBehaviorRevisions(migration.Revisions) || migration.SourceRevision != migration.Revisions.E2E.Revision || migration.DocumentationRevision == "" {
		return nil, fail("playwright discovery requires the exact schema-2 migration record")
	}
	listed, err := typescript.PlaywrightListedTests(input.Root, input.ConfigPath, input.Listing)
	if err != nil {
		return nil, fail("playwright listing refused: " + err.Error())
	}
	if len(listed) == 0 || len(listed) > MaxRecords {
		return nil, fail("playwright listing has no tests or exceeds the execution bound")
	}
	revision := migration.SourceRevision
	repository, err := contextindex.CorpusRepositoryID(ctx, input.Root, revision)
	if err != nil {
		return nil, err
	}
	if repository != migration.Revisions.E2E.ID {
		return nil, fail("repository root is not the migration's source repository")
	}
	pinned, err := newPlaywrightPinnedSources(ctx, input.Root, repository, revision)
	if err != nil {
		return nil, err
	}
	defer pinned.close()
	config, err := pinned.file(ctx, input.ConfigPath)
	if err != nil {
		return nil, err
	}
	lines := bytes.Count(config.data, []byte{'\n'})
	if len(config.data) > 0 && config.data[len(config.data)-1] != '\n' {
		lines++
	}
	configAnchor, err := pinned.anchor(config, 1, lines, "caller-run playwright --list configuration")
	if err != nil {
		return nil, err
	}
	adopted, err := adoptedPlaywrightIDs(input.Receipt, input.ConfigPath, config, listed, pinned, ctx)
	if err != nil {
		return nil, err
	}
	record := BehaviorDiscovery{Schema: PlaywrightDiscoverySchema, Mode: PlaywrightDiscoveryMode, Revisions: migration.Revisions, Config: configAnchor}
	ids := make([]string, 0, len(listed))
	for index, test := range listed {
		source, err := pinned.file(ctx, test.Path)
		if err != nil {
			return nil, err
		}
		evidence, err := pinned.anchor(source, test.Line, test.Line, "caller-run playwright --list execution")
		if err != nil {
			return nil, fail("playwright listing location is outside its pinned test file: " + test.Path)
		}
		id := "playwright:" + test.ID
		if qualified := adopted[index]; qualified != "" {
			id = qualified
		}
		if !textOK(test.Project) {
			return nil, fail("playwright listing execution lacks a project: " + test.Path)
		}
		ids = append(ids, id)
		record.Executions = append(record.Executions, BehaviorExecution{ID: id, Project: test.Project, Evidence: evidence})
	}
	if !uniqueIdentities(ids) {
		return nil, fail("playwright discovery execution identity is duplicate")
	}
	sort.SliceStable(record.Executions, func(i, j int) bool {
		left, right := record.Executions[i], record.Executions[j]
		if left.Evidence.Path != right.Evidence.Path {
			return left.Evidence.Path < right.Evidence.Path
		}
		if left.Evidence.Start != right.Evidence.Start {
			return left.Evidence.Start < right.Evidence.Start
		}
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		return left.ID < right.ID
	})
	return Encode(record)
}

type playwrightPinnedFile struct {
	path, blob string
	data       []byte
}

type playwrightPinnedSources struct {
	root, repository, revision string
	auth                       *gitauth.Repository
	release                    func()
	files                      map[string]playwrightPinnedFile
}

func newPlaywrightPinnedSources(ctx context.Context, root, repository, revision string) (*playwrightPinnedSources, error) {
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	return &playwrightPinnedSources{root: root, repository: repository, revision: revision, auth: auth, release: auth.BeginObjectSession(), files: map[string]playwrightPinnedFile{}}, nil
}

func (p *playwrightPinnedSources) close() { p.release() }

// file returns the bytes of path at the pinned revision and refuses a working-tree copy that
// differs, because the caller-run listing observed the working tree.
func (p *playwrightPinnedSources) file(ctx context.Context, path string) (playwrightPinnedFile, error) {
	if cached, ok := p.files[path]; ok {
		return cached, nil
	}
	entry, ok, err := p.auth.LookupTreeEntry(ctx, p.revision, path)
	if err != nil {
		return playwrightPinnedFile{}, err
	}
	if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return playwrightPinnedFile{}, fail("playwright file is not a regular Git blob at the source revision: " + path)
	}
	data, over, err := p.auth.BlobBytesWithin(ctx, entry.OID, MaxBytes)
	if err != nil {
		return playwrightPinnedFile{}, err
	}
	if over {
		return playwrightPinnedFile{}, fail("playwright file exceeds the input bound: " + path)
	}
	current, err := affected.ReadSource(p.root, path)
	if err != nil || !bytes.Equal(current, data) {
		return playwrightPinnedFile{}, fail("working-tree bytes differ from the source revision, so the listing is not of the pinned bytes: " + path)
	}
	file := playwrightPinnedFile{path: path, blob: entry.OID, data: data}
	p.files[path] = file
	return file, nil
}

func (p *playwrightPinnedSources) anchor(file playwrightPinnedFile, start, end int, reason string) (Anchor, error) {
	excerpt, err := span(file.data, start, end)
	if err != nil {
		return Anchor{}, err
	}
	return Anchor{Repository: p.repository, Revision: p.revision, Path: file.path, Blob: file.blob, SHA256: Digest(file.data), Start: start, End: end, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "observed", Reason: reason}, nil
}

// adoptedPlaywrightIDs maps listed executions to the qualified receipt's test identities. A
// receipt must be a qualified Playwright document run against the same config and test bytes;
// each listed execution adopts the one receipt test with the same file, line, project and title.
func adoptedPlaywrightIDs(raw []byte, configPath string, config playwrightPinnedFile, listed []typescript.PlaywrightListedTest, pinned *playwrightPinnedSources, ctx context.Context) ([]string, error) {
	adopted := make([]string, len(listed))
	if len(raw) == 0 {
		return adopted, nil
	}
	document, prefix, err := qualifiedPlaywrightReceipt(raw, configPath)
	if err != nil {
		return nil, err
	}
	receipt := document.Playwright
	if receipt.Identity.ConfigDigest != Digest(config.data) {
		return nil, fail("qualified receipt ran a different config than the source revision")
	}
	for index, test := range listed {
		matches := 0
		for _, native := range receipt.Tests {
			if native.Anchor == nil || native.Project == nil || native.Anchor.Line != test.Line || native.Project.Name != test.Project || native.Name != test.Title {
				continue
			}
			if path, ok := strings.CutPrefix(native.Anchor.File, prefix); !ok || path != test.Path {
				continue
			}
			source, err := pinned.file(ctx, test.Path)
			if err != nil {
				return nil, err
			}
			if receipt.Identity.TestFileDigests[native.Anchor.File] != Digest(source.data) {
				return nil, fail("qualified receipt ran different test bytes than the source revision: " + test.Path)
			}
			matches++
			adopted[index] = native.ID
		}
		if matches > 1 {
			return nil, fail("qualified receipt has more than one test for a listed execution: " + test.Path)
		}
	}
	return adopted, nil
}

// qualifiedPlaywrightReceipt decodes a canonical qualified Playwright receipt and returns its
// projection with the absolute run-root prefix that maps its paths onto repository paths through
// configPath.
func qualifiedPlaywrightReceipt(raw []byte, configPath string) (testvaliditydoc.Document, string, error) {
	decoded, err := testvaliditydoc.Decode(raw)
	if err != nil {
		return testvaliditydoc.Document{}, "", fail("receipt is not a canonical qualified provider document")
	}
	document := testvaliditydoc.Project(decoded)
	if document.Playwright == nil {
		return testvaliditydoc.Document{}, "", fail("receipt is not a qualified Playwright receipt")
	}
	prefix, ok := strings.CutSuffix(document.Playwright.Identity.ConfigFile, "/"+configPath)
	if !ok {
		return testvaliditydoc.Document{}, "", fail("qualified receipt config is not " + configPath)
	}
	return document, prefix + "/", nil
}
