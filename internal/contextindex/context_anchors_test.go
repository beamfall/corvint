package contextindex

import (
	"context"
	"testing"
)

// anchorFixtureIndex holds, per anchor class, one source carrying the literal
// verbatim and one carrying only its split tokens.
func anchorFixtureIndex(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/anchors\n\ngo 1.27.0\n",
		"AGENTS.md":       "# Standing instructions\n\nRead before editing.\n",
		"dial.go":         "package sample\n\nvar errDial = errors.New(\"connection refused: dial tcp\")\n",
		"dial_split.go":   "package sample\n\n// connection refused dial tcp\n",
		"client.go":       "package sample\n\nconst endpoint = \"https://api.example.test/v1/items\"\n",
		"client_split.go": "package sample\n\n// https api example test v1 items\n",
		"state.rs":        "fn main() { let s = Status::Active; }\n",
		"state_split.rs":  "// Status Active\n",
		"config.go":       "package sample\n\nvar port = get(\"server.http.port\")\n",
		"config_split.go": "package sample\n\n// server http port\n",
		"config_super.go": "package sample\n\n// xserver.http.port\n",
		"trace.txt":       "goroutine 1 [running]:\nmain.run()\n\tcmd/app/main.go:42 +0x1d\n",
		"trace_split.txt": "cmd/app/main.go line 42\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}
