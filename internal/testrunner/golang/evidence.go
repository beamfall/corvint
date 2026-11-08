// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// Source is a regular immutable tree blob supplied by the core, not a worktree
// read. Symlinks and missing bytes cannot be certified by this native adapter.
type Source struct {
	OID   string
	Mode  string
	Bytes []byte
}

func (s Source) valid(name string) bool {
	if !localPath(name, false) || len(s.Bytes) > MaxArtifactBytes || (s.Mode != "100644" && s.Mode != "100755") {
		return false
	}
	raw := append([]byte(fmt.Sprintf("blob %d\x00", len(s.Bytes))), s.Bytes...)
	if len(s.OID) == 40 {
		sum := sha1.Sum(raw)
		return hex.EncodeToString(sum[:]) == s.OID
	}
	if len(s.OID) == 64 {
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:]) == s.OID
	}
	return false
}

type goPackage struct {
	Dir, ImportPath, Name, ForTest                                                                                                     string
	Standard, Incomplete                                                                                                               bool
	Error                                                                                                                              json.RawMessage
	DepsErrors                                                                                                                         []json.RawMessage
	Imports, Deps                                                                                                                      []string
	GoFiles, CgoFiles, TestGoFiles, XTestGoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles []string
	EmbedFiles, TestEmbedFiles, XTestEmbedFiles                                                                                        []string
}

func decodeDependencies(raw string) ([]goPackage, error) {
	if raw == "" || len(raw) > MaxArtifactBytes {
		return nil, fmt.Errorf("go-dependencies-missing-or-capped")
	}
	d := json.NewDecoder(strings.NewReader(raw))
	var packages []goPackage
	seen := map[string]bool{}
	for {
		var pkg goPackage
		var frame json.RawMessage
		err := d.Decode(&frame)
		if err == io.EOF {
			break
		}
		if err == nil {
			err = jsonv2.Unmarshal(frame, &pkg)
		}
		if err != nil {
			return nil, fmt.Errorf("go-dependencies-malformed")
		}
		if pkg.ImportPath == "" || seen[pkg.ImportPath] || pkg.Incomplete || len(pkg.DepsErrors) > 0 || len(pkg.Error) > 0 && string(pkg.Error) != "null" {
			return nil, fmt.Errorf("go-dependencies-incomplete")
		}
		if !filepath.IsAbs(pkg.Dir) && pkg.ImportPath != "unsafe" {
			return nil, fmt.Errorf("go-dependencies-directory-unknown")
		}
		seen[pkg.ImportPath] = true
		packages = append(packages, pkg)
	}
	for _, pkg := range packages {
		for _, dep := range append(append([]string{}, pkg.Imports...), pkg.Deps...) {
			if dep != "C" && !seen[dep] {
				return nil, fmt.Errorf("go-dependencies-truncated")
			}
		}
	}
	return packages, nil
}

func (p *Plugin) loaded() testevidence.Component[testevidence.Blob] {
	fail := func(reason string) testevidence.Component[testevidence.Blob] {
		return testevidence.Component[testevidence.Blob]{State: testevidence.Incomplete, Reason: reason}
	}
	packages, err := decodeDependencies(p.capture.Dependencies)
	if err != nil {
		return fail(err.Error())
	}
	blobs := map[string]testevidence.Blob{}
	for _, pkg := range packages {
		// go list reports a generated test-main package whose source lives in the
		// Go build cache. Its imports remain checked by decodeDependencies and all
		// repository files of those packages are included below.
		if pkg.ImportPath == p.config.ImportPath+".test" && pkg.Name == "main" {
			generated := len(pkg.GoFiles) > 0
			for _, file := range pkg.GoFiles {
				rel, e := filepath.Rel(p.repositoryRoot, file)
				if !filepath.IsAbs(file) || e != nil || localPath(filepath.ToSlash(rel), true) {
					generated = false
				}
			}
			if generated {
				continue
			}
		}
		rel, err := filepath.Rel(p.repositoryRoot, pkg.Dir)
		inside := err == nil && localPath(filepath.ToSlash(rel), true)
		if !inside {
			// Standard packages and generated test-main files are outside the tree.
			// Other outside dependencies are not silently mistaken for repository files.
			if pkg.Standard || pkg.ImportPath == p.config.ImportPath+".test" {
				continue
			}
			return fail("go-loaded-outside-repository")
		}
		groups := [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.TestGoFiles, pkg.XTestGoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.SysoFiles, pkg.EmbedFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles}
		for _, files := range groups {
			for _, file := range files {
				if !localPath(file, false) {
					return fail("go-loaded-path-unsupported")
				}
				name := path.Join(filepath.ToSlash(rel), file)
				source, ok := p.capture.Sources[name]
				if !ok || !source.valid(name) {
					return fail("go-loaded-source-unbound")
				}
				blobs[name] = p.blob(name, source)
			}
		}
	}
	names := make([]string, 0, len(blobs))
	for name := range blobs {
		names = append(names, name)
	}
	sort.Strings(names)
	result := testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{}}
	for _, name := range names {
		result.Items = append(result.Items, blobs[name])
	}
	return result
}
func (p *Plugin) blob(name string, source Source) testevidence.Blob {
	return testevidence.Blob{Path: testevidence.Path{Repository: p.config.Repository, Lexical: name, Resolved: name}, OID: source.OID}
}

func (p *Plugin) native(ctx context.Context) ([]testevidence.Record, error) {
	inv, err := p.Inventory(ctx, registry.InventoryRequest{Key: p.key, Output: p.capture.List})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, test := range inv.Tests {
		known[test.ID] = true
	}
	oids := map[string]string{}
	for name, source := range p.capture.Sources {
		if !source.valid(name) {
			return nil, fmt.Errorf("go-evidence-source-unbound")
		}
		// linecov consumes module-relative paths. Keep unrelated repository
		// sources out of that temporary map, then restore the prefix below.
		if p.modulePrefix == "." {
			oids[name] = source.OID
		} else if relative, ok := strings.CutPrefix(name, p.modulePrefix+"/"); ok {
			oids[relative] = source.OID
		}
	}
	for _, coverage := range p.capture.Coverage {
		if !known[coverage.Package+"/"+coverage.Name] {
			return nil, fmt.Errorf("go-evidence-test-not-in-inventory")
		}
		if !strings.HasPrefix(string(coverage.Profile), "mode: set\n") && !strings.HasPrefix(string(coverage.Profile), "mode: count\n") && !strings.HasPrefix(string(coverage.Profile), "mode: atomic\n") {
			return nil, fmt.Errorf("go-coverage-mode-missing-or-unknown")
		}
	}
	byTest := map[string]testevidence.Record{}
	for _, coverage := range p.capture.Coverage {
		id := coverage.Package + "/" + coverage.Name
		if _, exists := byTest[id]; exists {
			return nil, fmt.Errorf("go-coverage-test-duplicate")
		}
		// Build each isolated profile independently: another test's instrumented
		// file set must not manufacture unexecuted coverage for this test.
		ix, err := linecov.Build(p.config.Module, p.config.Checkout, p.config.RunnerVersion, oids, []linecov.Record{coverage})
		if err != nil {
			return nil, err
		}
		records, err := ix.EvidenceRecords(p.config.Repository)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			for i := range record.Executed.Items {
				blob := &record.Executed.Items[i].Blob
				blob.Path.Lexical = path.Join(p.modulePrefix, blob.Path.Lexical)
				blob.Path.Resolved = path.Join(p.modulePrefix, blob.Path.Resolved)
			}
			byTest[record.Test] = record
		}
	}

	loaded := p.loaded()
	out := make([]testevidence.Record, 0, len(inv.Tests))
	for _, test := range inv.Tests {
		record, ok := byTest[test.ID]
		if !ok {
			record = testevidence.New(p.config.Repository, test.ID, p.config.Checkout)
		}
		record.Loaded = loaded
		record.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present, Items: []testevidence.RegionsFile{}}
		for _, file := range record.Executed.Items {
			source, ok := p.capture.Sources[file.Blob.Path.Lexical]
			if !ok {
				record.Regions = testevidence.Missing[testevidence.RegionsFile]("go-region-source-unbound")
				break
			}
			bodies, err := linecov.GoBodies(source.Bytes)
			if err != nil {
				record.Regions = testevidence.Missing[testevidence.RegionsFile]("go-regions-unparseable")
				break
			}
			record.Regions.Items = append(record.Regions.Items, testevidence.RegionsFile{Blob: file.Blob, Bodies: bodies})
		}
		if record.Executed.State != testevidence.Present {
			record.Regions = testevidence.Missing[testevidence.RegionsFile]("go-executed-incomplete")
		}
		if err := record.ValidateComponents(); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}

// Assemble joins the native evidence with core-owned trace components using the
// T1 record API. Missing traces remain absent/incomplete; this never upgrades
// them, attributes a shared tree to one test, admits provenance or stores data.
func (p *Plugin) Assemble(ctx context.Context, header testevidence.Record, trace testevidence.Trace) (testevidence.Record, error) {
	if header.Repository != p.config.Repository || header.Commit != p.key.Checkout || header.Runner.Plugin != p.key.Runner || header.Runner.ConfigurationDigest != p.key.ConfigDigest || header.Runner.ExecutableDigest != p.config.ExecutableDigest || header.Runner.RunnerVersion != p.config.RunnerVersion || header.Execution.OS != p.key.OS || header.Execution.Architecture != p.key.Arch {
		return testevidence.Record{}, fmt.Errorf("go-record-identity-mismatch")
	}
	records, err := p.native(ctx)
	if err != nil {
		return testevidence.Record{}, err
	}
	for _, record := range records {
		if record.Test == header.Test {
			return testevidence.Assemble(header, testevidence.Native{Executed: record.Executed, Regions: record.Regions, Loaded: record.Loaded}, trace)
		}
	}
	return testevidence.Record{}, fmt.Errorf("go-record-test-not-in-inventory")
}
