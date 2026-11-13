package contextindex

// spanFixtureFiles define RunEngine in one file and call it from another, so
// the span ranker has a core definition and a call site to emit.
var spanFixtureFiles = map[string]string{
	"go.mod": "module example.test/spans\n\ngo 1.27.0\n",
	"pkg/core/engine.go": "package core\n\n// RunEngine turns the crank once.\nfunc RunEngine(x int) int {\n\treturn crankOnce(x)\n}\n\n" +
		"func crankOnce(x int) int {\n\treturn x + 1\n}\n",
	"pkg/app/main.go":  "package app\n\nimport \"example.test/spans/pkg/core\"\n\n// Start runs the engine.\nfunc Start() int {\n\treturn core.RunEngine(1)\n}\n",
	"pkg/util/text.go": "package util\n\nfunc Trim(value string) string { return value }\n",
	"docs/engine.md":   "# engine\n\nThe engine turns a crank.\n",
}
