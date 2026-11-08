// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// Only evaluated compile items and references can close loaded. Merely finding
// a csproj or project.assets.json never proves the build graph was complete.
func (p *dotnetPlugin) loaded(ctx context.Context) testevidence.Component[testevidence.Blob] {
	out := testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{}}
	fail := func(err error) { out.State, out.Reason = testevidence.Incomplete, err.Error() }
	seen, visiting, files := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if visiting[name] {
			return fmt.Errorf("dotnet-project-reference-cycle")
		}
		if seen[name] {
			return nil
		}
		if len(seen) >= testrunner.MaxTests {
			return fmt.Errorf("dotnet-project-cap")
		}
		seen[name], visiting[name] = true, true
		project, ok := p.c.Projects[name]
		if !ok || !project.Complete || !dotnetWord(project.Framework) || name == p.c.Project && project.Framework != p.c.TFM || len(project.Compile) == 0 || len(project.Compile) > testrunner.MaxTests || len(project.References) > testrunner.MaxTests {
			return fmt.Errorf("dotnet-project-closure-incomplete:%s", name)
		}
		add := func(file string) error {
			source, err := p.source(file)
			if err != nil {
				return err
			}
			if !files[file] {
				if len(files) >= testrunner.MaxTests {
					return fmt.Errorf("dotnet-loaded-cap")
				}
				files[file] = true
				out.Items = append(out.Items, source.Blob)
			}
			return nil
		}
		if err := add(name); err != nil {
			return err
		}
		// The evaluated graph also depends on restore and ancestor configuration.
		// Probe the immutable core snapshot, never the mutable checkout.
		assets := path.Join(path.Dir(name), "obj/project.assets.json")
		if err := add(assets); err != nil {
			return err
		}
		if !bytes.Equal(p.c.Sources[assets].Data, project.Assets) {
			return fmt.Errorf("dotnet-restore-assets-blob-mismatch:%s", name)
		}
		for dir := path.Dir(name); ; dir = path.Dir(dir) {
			for _, base := range []string{"Directory.Build.props", "Directory.Build.targets", "Directory.Packages.props", "global.json", "nuget.config", "NuGet.Config", "NuGet.config"} {
				file := path.Join(dir, base)
				if _, exists := p.c.Sources[file]; exists {
					if err := add(file); err != nil {
						return err
					}
				}
			}
			if dir == "." {
				break
			}
		}
		for _, file := range project.Compile {
			if err := add(file); err != nil {
				return err
			}
		}
		if err := p.assets(name, project); err != nil {
			return err
		}
		for _, reference := range project.References {
			if err := visit(reference); err != nil {
				return err
			}
		}
		visiting[name] = false
		return nil
	}
	if err := visit(p.c.Project); err != nil {
		fail(err)
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Path.Lexical < out.Items[j].Path.Lexical })
	return out
}

func (p *dotnetPlugin) assets(name string, project DotnetProject) error {
	if len(project.Assets) == 0 || len(project.Assets) > testrunner.MaxReportBytes {
		return fmt.Errorf("dotnet-restore-assets-missing-or-capped:%s", name)
	}
	var doc struct {
		Version int `json:"version"`
		Targets map[string]map[string]struct {
			Type string `json:"type"`
		} `json:"targets"`
		Libraries map[string]struct {
			Type    string `json:"type"`
			Path    string `json:"path"`
			Project string `json:"msbuildProject"`
		} `json:"libraries"`
	}
	if err := jsonv2.Unmarshal(project.Assets, &doc); err != nil || doc.Version != 3 || doc.Targets == nil || doc.Libraries == nil {
		return fmt.Errorf("dotnet-restore-assets-invalid:%s", name)
	}
	target, ok := doc.Targets[project.Framework]
	if !ok || target == nil {
		return fmt.Errorf("dotnet-restore-target-missing:%s", project.Framework)
	}
	refs := map[string]bool{}
	for _, ref := range project.References {
		if !dotnetPath(ref) || !strings.HasSuffix(ref, ".csproj") || refs[ref] {
			return fmt.Errorf("dotnet-project-reference-invalid")
		}
		refs[ref] = true
	}
	assets := map[string]bool{}
	for key, library := range target {
		declared, exists := doc.Libraries[key]
		if !exists || declared.Type != library.Type {
			return fmt.Errorf("dotnet-restore-library-unresolved")
		}
		switch library.Type {
		case "package": // restored external packages; repository inputs are compile items and core traces
		case "project":
			file := declared.Project
			if file == "" {
				file = declared.Path
			}
			file = strings.ReplaceAll(file, "\\", "/")
			if mapped := p.c.SourceNames[file]; mapped != "" {
				file = mapped
			} else if filepath.IsAbs(file) {
				rel, err := filepath.Rel(p.c.Root, file)
				if err != nil {
					return fmt.Errorf("dotnet-restore-project-path-unresolved")
				}
				file = filepath.ToSlash(rel)
			} else {
				file = path.Join(path.Dir(name), file)
			}
			if !dotnetPath(file) || !refs[file] || assets[file] {
				return fmt.Errorf("dotnet-restore-project-reference-mismatch")
			}
			assets[file] = true
		default:
			return fmt.Errorf("dotnet-restore-library-type-unsupported")
		}
	}
	if len(assets) != len(refs) {
		return fmt.Errorf("dotnet-restore-project-reference-missing")
	}
	return nil
}
