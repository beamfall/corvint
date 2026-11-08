// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// Serve is the reference server and conformance transport for any compiled-in
// SPI implementation. It never executes the commands a plugin returns.
func Serve(ctx context.Context, in io.Reader, out io.Writer, name, version string, plugins map[string]registry.RunnerPlugin) error {
	reader := bufio.NewReaderSize(in, (1<<20)+1)
	seen := map[string]bool{}
	ready := false
	for {
		line, err := reader.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("protocol-error:frame:%w", err)
		}
		if len(line) > 1<<20+1 {
			return fmt.Errorf("oversize-message")
		}
		r, err := decodeRequest(line[:len(line)-1])
		if err != nil {
			return err
		}
		if seen[r.ID] || len(seen) >= 65536 {
			return fmt.Errorf("protocol-error:request-id")
		}
		seen[r.ID] = true
		if !ready && r.Method != "hello" || ready && r.Method == "hello" {
			return fmt.Errorf("protocol-error:hello-order")
		}
		var result any
		if r.Method == "hello" {
			var params struct {
				Protocols []string `json:"protocols"`
				Core      string   `json:"core"`
			}
			_ = json.Unmarshal(r.Params, &params)
			if !contains(params.Protocols, Protocol) {
				err = fmt.Errorf("version-mismatch")
			} else {
				h := Hello{Protocol: Protocol, Plugin: name, Version: version, Runners: []string{}, Pieces: map[string][]string{}, Features: []string{}}
				for runner, plugin := range plugins {
					h.Runners = append(h.Runners, runner)
					p := plugin.Pieces()
					pieces := []string{}
					if p.Inventory.Present {
						pieces = append(pieces, "inventory")
					}
					if p.TestEvidence.Present {
						pieces = append(pieces, "test-evidence")
					}
					if p.Qualification.Present {
						pieces = append(pieces, "qualification")
					}
					h.Pieces[runner] = pieces
				}
				sortStrings(h.Runners)
				result = h
				ready = true
			}
		} else if r.Method == "shutdown" {
			result = map[string]any{}
		} else {
			result, err = serveMethod(ctx, plugins, r)
		}
		response := response{ID: r.ID}
		if err != nil {
			response.Error = &remoteError{Code: "runner-failed", Message: err.Error()}
		} else {
			response.Result, err = json.Marshal(result)
			if err != nil {
				return err
			}
			var v any
			if err = validatePayload(r.Method, response.Result, true, &v); err != nil {
				return err
			}
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			return err
		}
		if len(encoded) > 1<<20 {
			return fmt.Errorf("oversize-message")
		}
		if _, err = out.Write(append(encoded, '\n')); err != nil {
			return err
		}
		if response.Error != nil || r.Method == "shutdown" {
			return nil
		}
	}
}
func serveMethod(ctx context.Context, plugins map[string]registry.RunnerPlugin, r request) (any, error) {
	var p struct {
		Runner    string   `json:"runner"`
		Purpose   string   `json:"purpose"`
		Tests     []string `json:"tests"`
		RunDir    string   `json:"run_dir"`
		Output    string   `json:"output"`
		Report    string   `json:"report"`
		Artifacts []string `json:"artifacts"`
	}
	if err := json.Unmarshal(r.Params, &p); err != nil {
		return nil, err
	}
	plugin := plugins[p.Runner]
	if plugin == nil {
		return nil, fmt.Errorf("undeclared-runner")
	}
	switch r.Method {
	case "inventory":
		in, err := plugin.Inventory(ctx, registry.InventoryRequest{Key: testrunner.InventoryKey{Runner: p.Runner}, Output: p.Output})
		if err != nil {
			return nil, err
		}
		if !in.Complete || in.Reason != "" {
			return nil, fmt.Errorf("inventory-incomplete:%s", in.Reason)
		}
		out := inventoryResult{ConfigDigest: in.Key.ConfigDigest, Tests: []inventoryTest{}}
		for _, t := range in.Tests {
			out.Tests = append(out.Tests, inventoryTest{ID: t.ID, Path: t.Path})
		}
		return out, nil
	case "command":
		cmd, err := plugin.Command(ctx, registry.CommandRequest{Key: testrunner.InventoryKey{Runner: p.Runner}, Purpose: p.Purpose, Tests: p.Tests, RunDir: p.RunDir})
		if err != nil {
			return nil, err
		}
		if cmd.Env == nil {
			cmd.Env = map[string]string{}
		}
		return commandResult{Argv: cmd.Argv, Cwd: cmd.Cwd, Env: cmd.Env, Artifacts: append([]string{}, cmd.Artifacts...), Superset: cmd.Superset}, nil
	case "parse":
		results, err := plugin.Parse(ctx, registry.ParseRequest{Runner: p.Runner, Report: p.Report})
		if err != nil {
			return nil, err
		}
		out := parseResult{Results: []testResult{}}
		for _, r := range results {
			out.Results = append(out.Results, testResult{ID: r.ID, Outcome: r.Outcome, Reason: r.Reason})
		}
		return out, nil
	case "evidence":
		records, err := plugin.Evidence(ctx, registry.EvidenceRequest{Runner: p.Runner, Artifacts: p.Artifacts})
		if err != nil {
			return nil, err
		}
		out := evidenceResult{Records: []nativeRecord{}}
		for _, record := range records {
			n := nativeRecord{Test: record.Test}
			for _, part := range []struct {
				data   []byte
				target any
			}{{record.Executed, &n.Executed}, {record.Regions, &n.Regions}, {record.Loaded, &n.Loaded}} {
				if err := json.Unmarshal(part.data, part.target); err != nil {
					return nil, err
				}
			}
			out.Records = append(out.Records, n)
		}
		return out, nil
	}
	return nil, fmt.Errorf("protocol-error:unknown-method")
}
