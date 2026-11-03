// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// playwright drives @playwright/test with one worker and no retries; its
// webServer, when configured, is the server under test (LPCV-V0-971).
type playwright struct {
	webServer bool
}

type pwReport struct {
	Config struct {
		RootDir   string          `json:"rootDir"`
		WebServer json.RawMessage `json:"webServer"`
	} `json:"config"`
	Suites []pwSuite `json:"suites"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type pwSuite struct {
	Title  string    `json:"title"`
	File   string    `json:"file"`
	Specs  []pwSpec  `json:"specs"`
	Suites []pwSuite `json:"suites"`
}

type pwSpec struct {
	Title string `json:"title"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Tests []struct {
		ProjectName string `json:"projectName"`
		Status      string `json:"status"`
	} `json:"tests"`
}

// pwKey runs one Playwright test alone.
type pwKey struct {
	location, project string
}

func (p *playwright) version(_ context.Context, c *collector) (string, error) {
	return packageVersion(c.Root, "@playwright/test", "playwright")
}

func (p *playwright) serves(*collector) bool { return p.webServer }

func (p *playwright) args(extra ...string) []string {
	return append([]string{"test", "--workers=1", "--retries=0", "--reporter=json"}, extra...)
}

func (p *playwright) report(ctx context.Context, c *collector, dir string, extra ...string) (*pwReport, error) {
	args := p.args(extra...)
	if c.Config != "" {
		args = append(args, "--config", c.Config)
	}
	file := filepath.Join(dir, "report.json")
	cmd := c.command(ctx, dir, "playwright", args...)
	cmd.Env = append(cmd.Env, "PLAYWRIGHT_JSON_OUTPUT_FILE="+file)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stderr, &stderr
	runErr := runGroup(cmd)
	if c.Server == "" {
		reapServers(dir) // a webServer the runner started; the collector stops its own
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("browsercov: playwright wrote no report (%v): %s", runErr, tail(stderr.Bytes()))
	}
	var r pwReport
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("browsercov: playwright report: %v", err)
	}
	if len(r.Errors) > 0 {
		return nil, fmt.Errorf("browsercov: playwright: %s", r.Errors[0].Message)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return &r, nil
}

func (p *playwright) specs(ctx context.Context, c *collector) ([]string, error) {
	dir, err := c.newDir()
	if err != nil {
		return nil, err
	}
	// Listing loads the configuration and test files only; nothing runs.
	r, err := p.report(ctx, c, dir, "--list")
	if err != nil {
		return nil, err
	}
	p.webServer = len(r.Config.WebServer) > 0 && string(r.Config.WebServer) != "null"
	seen := map[string]bool{}
	var out []string
	var walk func([]pwSuite)
	walk = func(ss []pwSuite) {
		for _, s := range ss {
			for _, sp := range s.Specs {
				if f := p.rel(c, r, sp.File); !seen[f] {
					seen[f] = true
					out = append(out, f)
				}
			}
			walk(s.Suites)
		}
	}
	walk(r.Suites)
	sort.Strings(out)
	return out, nil
}

// rel turns a report path (relative to the config's rootDir) root-relative.
func (p *playwright) rel(c *collector, r *pwReport, file string) string {
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.Config.RootDir, file)
	}
	if rel, err := filepath.Rel(c.Root, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(abs)
}

func (p *playwright) run(ctx context.Context, c *collector, dir, spec string, only *result) ([]result, error) {
	target := filepath.Join(c.Root, filepath.FromSlash(spec))
	extra := []string{target}
	if only != nil {
		k := only.key.(pwKey)
		extra = []string{k.location, "--project=" + k.project}
	}
	r, err := p.report(ctx, c, dir, extra...)
	if err != nil {
		return nil, err
	}
	var out []result
	var walk func(ss []pwSuite, titles []string) error
	walk = func(ss []pwSuite, titles []string) error {
		for _, s := range ss {
			path := titles
			if s.File == "" || s.Title != s.File {
				path = append(append([]string(nil), titles...), s.Title)
			}
			for _, sp := range s.Specs {
				file := p.rel(c, r, sp.File)
				if file != spec {
					return fmt.Errorf("browsercov: running %s also ran %s", spec, file)
				}
				for _, t := range sp.Tests {
					out = append(out, result{
						Name:  pwName(t.ProjectName, append(append([]string(nil), path...), sp.Title)),
						State: pwState(t.Status),
						key:   pwKey{location: target + ":" + strconv.Itoa(sp.Line), project: t.ProjectName},
					})
				}
			}
			if err := walk(s.Suites, path); err != nil {
				return err
			}
		}
		return nil
	}
	// A top-level suite is the file itself; its title is the file path.
	for _, s := range r.Suites {
		if err := walk([]pwSuite{{Specs: s.Specs, Suites: s.Suites}}, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func pwName(project string, titles []string) string {
	var parts []string
	for _, t := range titles {
		if t != "" {
			parts = append(parts, t)
		}
	}
	name := strings.Join(parts, " > ")
	if project != "" {
		name = "[" + project + "] " + name
	}
	return name
}

func pwState(status string) string {
	switch status {
	case "expected":
		return "passed"
	case "skipped":
		return "skipped"
	}
	return "failed" // unexpected, flaky
}

// packageVersion reads the version of the first installed package named.
func packageVersion(root string, names ...string) (string, error) {
	for _, n := range names {
		body, err := os.ReadFile(filepath.Join(root, "node_modules", filepath.FromSlash(n), "package.json"))
		if err != nil {
			continue
		}
		var p struct{ Version string }
		if json.Unmarshal(body, &p) == nil && p.Version != "" {
			return n + "@" + p.Version, nil
		}
	}
	return "", fmt.Errorf("browsercov: %s is not installed under %s", names[0], filepath.Join(root, "node_modules"))
}
