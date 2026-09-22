package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// An exec exit alone cannot distinguish failed tests from missing evidence.
func containerRunResult(o options, p containerProfile, dir string, exports map[string]string, runErr error) (int, error) {
	code := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) || exit.ExitCode() != 1 {
			return 2, runErr
		}
		code = 1
	}
	for _, name := range exportFiles("run") {
		if name != "plan.json" && name != "selector.txt" {
			info, statErr := os.Lstat(filepath.Join(dir, name))
			if exports[name] != "" || statErr != nil || !info.Mode().IsRegular() {
				return 2, errors.New("mandatory run evidence export failed")
			}
		}
	}
	var s selection
	var e execution
	if err := readJSON(filepath.Join(dir, "selection.json"), &s); err != nil {
		return 2, err
	}
	if err := readJSON(filepath.Join(dir, "execution.json"), &e); err != nil {
		return 2, err
	}
	id := s.Identity
	if s.Schema != schema || s.Base != o.base || s.Head != o.head || s.Target != o.target || s.ObservedTarget != o.target || !oid.MatchString(s.Tree) || s.ObservedTree != s.Tree || id.Source != o.source || id.GoVersion != "go1.27.0" || id.OS != "linux" || id.Arch != "amd64" || id.GoBinary == "" || id.Compiler == "" || len(id.Env) == 0 || !equal(id.Container, &p) || id.Driver != p.Tools["corvint-pr-tests"] || id.Planner != p.Tools["corvint"] || id.Selector != p.Tools["gate-affected-select"] || !equal(e.Selection, s) || e.Error != "" || e.Exit != code || !equal(e.Env, id.Env) || !equal(id.Args, testArgs) || !equal(e.Args, append(append([]string{}, testArgs...), s.Packages...)) || len(s.Packages) == 0 {
		return 2, errors.New("incoherent run execution evidence")
	}
	if s.Reason != "" && !equal(s.Packages, []string{"./..."}) {
		return 2, errors.New("fallback package mismatch")
	}
	for _, link := range []struct{ name, hash string }{{"plan.json", s.PlanSHA}, {"selector.txt", s.AuditSHA}} {
		actual, err := digestFile(filepath.Join(dir, link.name))
		if link.hash != "" && (err != nil || actual != link.hash) {
			return 2, errors.New("run plan/audit digest mismatch")
		}
		if link.hash == "" && (!os.IsNotExist(err) || s.Reason == "") {
			return 2, errors.New("unexplained run plan/audit absence")
		}
	}
	// This checks every observed package, not the historical full-universe contract.
	f, err := os.Open(filepath.Join(dir, "go.json"))
	if err != nil {
		return 2, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 8<<20)
	observed := map[string]bool{}
	for scan.Scan() {
		var event struct{ Package string }
		if err = json.Unmarshal(scan.Bytes(), &event); err != nil {
			return 2, fmt.Errorf("malformed run test JSON: %w", err)
		}
		if event.Package != "" {
			observed[event.Package] = true
		}
	}
	if err = scan.Err(); err != nil {
		return 2, err
	}
	expected := s.Packages
	if equal(expected, []string{"./..."}) {
		expected = nil
		for name := range observed {
			expected = append(expected, name)
		}
	}
	if len(expected) == 0 {
		return 2, errors.New("missing run package outcomes")
	}
	if _, err = outcomes(filepath.Join(dir, "go.json"), expected, e); err != nil {
		return 2, err
	}
	return code, nil
}
