package gateselect

import (
	"errors"
	"testing"
)

// AFP-V0-403 (V1-2370): a base go.work whose go, toolchain, godebug or replace
// arguments the planner's grammar refuses fails closed with a typed refusal
// naming the directive and line; valid arguments still list their modules.
func TestBaseWorkspaceRefusesMalformedDirectiveArguments_AFPV0403(t *testing.T) {
	for name, test := range map[string]struct {
		body   string
		reason string
		line   int
	}{
		"a go version that is no version":             {body: "go latest\nuse .\n", reason: WorkspaceMalformedGo, line: 1},
		"a quoted go version":                         {body: "go \"1.27\"\nuse .\n", reason: WorkspaceMalformedGo, line: 1},
		"two go directives":                           {body: "go 1.27\ngo 1.27\nuse .\n", reason: WorkspaceMalformedGo, line: 2},
		"a go directive with two arguments":           {body: "go 1.27 1.28\nuse .\n", reason: WorkspaceMalformedGo, line: 1},
		"a toolchain that is not go1":                 {body: "go 1.27\ntoolchain gccgo1.27\nuse .\n", reason: WorkspaceMalformedTool, line: 2},
		"a toolchain without an argument":             {body: "go 1.27\ntoolchain\nuse .\n", reason: WorkspaceMalformedTool, line: 2},
		"a godebug without a value":                   {body: "go 1.27\ngodebug panicnil\nuse .\n", reason: WorkspaceMalformedGodebug, line: 2},
		"a godebug block entry with a comma":          {body: "go 1.27\ngodebug (\n\tpanicnil=1,x=2\n)\nuse .\n", reason: WorkspaceMalformedGodebug, line: 3},
		"a replace without an arrow":                  {body: "go 1.27\nreplace example.com/a ./a\nuse .\n", reason: WorkspaceMalformedReplace, line: 2},
		"a replace with a non-canonical version":      {body: "go 1.27\nreplace example.com/a v1 => ./a\nuse .\n", reason: WorkspaceMalformedReplace, line: 2},
		"a replace of a module to a versionless path": {body: "go 1.27\nreplace example.com/a => example.com/b\nuse .\n", reason: WorkspaceMalformedReplace, line: 2},
		"a replace whose major does not match":        {body: "go 1.27\nreplace (\n\texample.com/a/v2 v1.0.0 => ./a\n)\nuse .\n", reason: WorkspaceMalformedReplace, line: 3},
		"a use with a punctuation argument":           {body: "go 1.27\nuse ,\n", reason: WorkspaceMalformedUse, line: 2},
	} {
		t.Run(name, func(t *testing.T) {
			listed, err := parseBaseWorkspace([]byte(test.body), true)
			var refusal *WorkspaceRefusal
			if listed != nil || !errors.As(err, &refusal) || refusal.Reason != test.reason || refusal.Line != test.line {
				t.Fatalf("listed %v, err %v; want %s at line %d", listed, err, test.reason, test.line)
			}
		})
	}
	valid := "go 1.27.2\ntoolchain go1.27.2\ngodebug panicnil=1\nreplace example.com/a v1.2.3 => ./a\nreplace example.com/b => example.com/c v0.1.0\nuse (\n\t.\n\t./a\n)\n"
	if listed, err := parseBaseWorkspace([]byte(valid), true); err != nil || len(listed) != 2 || !listed["."] || !listed["a"] {
		t.Fatalf("valid go.work: listed %v, err %v", listed, err)
	}
}
