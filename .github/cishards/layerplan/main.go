// Command layerplan prints the layer plan of a change set (AFP-V0-2790..2793, V1-5235): which
// packages a pull-request shard runs when the change stays inside some layers of decision 0594
// item 8. Run as
//
//	layerplan --root DIR --changed FILE [--universe FILE] [--summary FILE]
//
// where --changed holds the changed repository paths separated by NUL bytes and --universe the
// shard universe (go list ./...), one import path per line. It prints the corvint-ci-layer-plan/0
// document on stdout and, with --summary, writes its Markdown summary. A plan that cannot be
// derived is FULL; only an unreadable input or output exits non-zero, which the workflow treats
// as FULL too.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "layerplan:", err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("root", "", "repository root to scan")
	changedFile := flag.String("changed", "", "NUL-separated changed repository paths")
	universeFile := flag.String("universe", "", "shard universe, one import path per line")
	summary := flag.String("summary", "", "write the Markdown summary to this file")
	flag.Parse()
	if *root == "" || *changedFile == "" || flag.NArg() != 0 {
		return fmt.Errorf("usage: layerplan --root DIR --changed FILE [--universe FILE] [--summary FILE]")
	}
	raw, err := os.ReadFile(*changedFile)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var changed []string
	for _, p := range strings.Split(string(raw), "\x00") {
		if p = strings.TrimSpace(p); p != "" && !seen[p] {
			seen[p] = true
			changed = append(changed, p)
		}
	}
	var universe []string
	if *universeFile != "" {
		data, err := os.ReadFile(*universeFile)
		if err != nil {
			return err
		}
		universe = strings.Fields(string(data))
		if len(universe) == 0 {
			return fmt.Errorf("%s lists no package", *universeFile)
		}
	}
	in, err := cishards.LoadLayerInputs(*root)
	if err != nil {
		return err
	}
	pkgs, err := cishards.ScanLayerPackages(*root)
	if err != nil {
		return err
	}
	plan := cishards.PlanLayers(in, pkgs, changed, universe)
	if *summary != "" {
		var b bytes.Buffer
		if err := cishards.WriteLayerSummary(&b, plan); err != nil {
			return err
		}
		if err := os.WriteFile(*summary, b.Bytes(), 0o600); err != nil {
			return err
		}
	}
	out, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Printf("%s\n", out)
	return err
}
