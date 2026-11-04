// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"io"
)

type commandNode struct {
	Path    []string `json:"path"`
	Command string   `json:"command"`
	Summary string   `json:"summary"`
	Usage   []string `json:"usage"`
	Flags   []string `json:"flags"`
}

func commandTree() any {
	nodes := []commandNode{
		{Path: []string{"collect"}, Summary: "Collect Go coverage records", Usage: []string{"linecov collect -root DIR -out INDEX [-packages a,b]"}, Flags: []string{"-root", "-out", "-packages", "-parallel", "-timeout", "-suite-timeout", "-work", "-prior", "-misses"}},
		{Path: []string{"select"}, Summary: "Select Go tests from an immutable index", Usage: []string{"linecov select -root DIR -index INDEX -base FULL_SHA [-static a,b]"}, Flags: []string{"-root", "-index", "-base", "-static"}},
		{Path: []string{"qualify"}, Summary: "Qualify Go selection by fault injection", Usage: []string{"linecov qualify -root DIR -index INDEX -packages a,b [-mutants N] [-seed N]"}, Flags: []string{"-root", "-index", "-packages", "-mutants", "-seed"}},
		{Path: []string{"redundancy"}, Summary: "Report repeated coverage", Usage: []string{"linecov redundancy -root DIR -index INDEX [-packages a,b]"}, Flags: []string{"-root", "-index", "-packages", "-low-unique", "-top", "-requirements"}},
		{Path: []string{"merge"}, Summary: "Merge compatible immutable indexes", Usage: []string{"linecov merge -out INDEX PART..."}, Flags: []string{"-out"}},
		{Path: []string{"ci"}, Summary: "Narrow a Go CI shard", Usage: []string{"linecov ci -root DIR -index INDEX -base FULL_SHA -packages FILE -slices FILE -out DIR"}, Flags: []string{"-root", "-index", "-base", "-packages", "-slices", "-out"}},
		{Path: []string{"ruby-collect"}, Summary: "Collect an immutable Ruby per-test index", Usage: []string{`linecov ruby-collect -root DIR -out INDEX -command '["ruby","test/all.rb"]'`}, Flags: []string{"-root", "-out", "-command", "-runner", "-executable", "-files", "-config", "-parallel", "-timeout", "-suite-timeout"}},
		{Path: []string{"ruby-select"}, Summary: "Report Ruby coverage shadow decisions", Usage: []string{"linecov ruby-select -root DIR -index INDEX -base FULL_SHA [-files a,b]"}, Flags: []string{"-root", "-index", "-base", "-files"}},
		{Path: []string{"js-collect"}, Summary: "Collect an immutable JavaScript or TypeScript per-test index", Usage: []string{"linecov js-collect -root DIR -out INDEX -runner node-test -executable /usr/bin/node -files test/a.test.js"}, Flags: []string{"-root", "-out", "-command", "-runner", "-executable", "-files", "-config", "-parallel", "-timeout", "-suite-timeout"}},
		{Path: []string{"js-select"}, Summary: "Report JavaScript coverage shadow decisions", Usage: []string{"linecov js-select -root DIR -index INDEX -base FULL_SHA [-files a,b]"}, Flags: []string{"-root", "-index", "-base", "-files"}},
		{Path: []string{"watch"}, Summary: "Rerun working-tree tests on foreground save notifications", Usage: []string{"linecov watch -root DIR -index INDEX [-once|-events-stdin] [-packages a,b] [-go-wrapper PATH] [-command ARGV_JSON]"}, Flags: []string{"-root", "-index", "-packages", "-once", "-events-stdin", "-go-wrapper", "-command"}},
	}
	for i := range nodes {
		nodes[i].Command = "linecov " + nodes[i].Path[0]
	}
	return struct {
		Schema   string        `json:"schema"`
		Binary   string        `json:"binary"`
		HelpJSON string        `json:"helpJson"`
		Source   string        `json:"source"`
		Nodes    []commandNode `json:"nodes"`
	}{"corvint-command-tree/0", "linecov", "linecov help --json", "tools/linecov/help.go", nodes}
}
func help(out io.Writer, args []string) error {
	if len(args) > 0 && (args[0] == "--json" || args[0] == "--tree") {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(commandTree())
	}
	_, err := fmt.Fprintln(out, `Usage: linecov collect|select|qualify|redundancy|merge|ci|ruby-collect|ruby-select|js-collect|js-select|watch [flags]
Use linecov COMMAND --help for required inputs and examples.
Ruby and JavaScript plans are experimental shadows; effective execution retains the static suite.
Watch output is user-scoped and cannot close a CI gate.`)
	return err
}
