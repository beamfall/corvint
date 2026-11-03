package jstestprovider

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// nodeVersionFor is the version of the node executable that runs the runner (CCE-V0-140): argv[0]
// when it is an absolute path to a node binary, otherwise the executable npm names in
// npm_node_execpath for an npx launch. The node on PATH is never recorded; when neither source is
// available the version is "" (unobserved), and an external run later fills it from the version
// the runner itself reported (recordedNodeVersion).
func nodeVersionFor(ctx context.Context, argv []string) string {
	if len(argv) > 0 && isNodeExecutable(argv[0]) {
		return versionOfNode(ctx, argv[0])
	}
	if exe := os.Getenv("npm_node_execpath"); isNodeExecutable(exe) {
		return versionOfNode(ctx, exe)
	}
	return ""
}

func isNodeExecutable(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	name := filepath.Base(path)
	return name == "node" || name == "node.exe"
}

// Node version probe bounds (V1-3020): the probe ends with ctx or after
// nodeVersionTimeout, whichever is first, a descendant holding its output
// open is abandoned after nodeVersionWaitDelay, and output beyond
// nodeVersionOutputLimit makes the version unobserved instead of growing.
const (
	nodeVersionTimeout     = 10 * time.Second
	nodeVersionWaitDelay   = time.Second
	nodeVersionOutputLimit = 256
)

func versionOfNode(ctx context.Context, executable string) string {
	ctx, cancel := context.WithTimeout(ctx, nodeVersionTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "--version")
	command.WaitDelay = nodeVersionWaitDelay
	out := cappedOutput{limit: nodeVersionOutputLimit}
	command.Stdout = &out
	if err := command.Run(); err != nil || out.overflow {
		return ""
	}
	return strings.TrimSpace(string(out.data))
}

// cappedOutput keeps at most limit bytes and records that more arrived.
type cappedOutput struct {
	data     []byte
	limit    int
	overflow bool
}

func (c *cappedOutput) Write(p []byte) (int, error) {
	if room := c.limit - len(c.data); len(p) > room {
		c.data = append(c.data, p[:max(room, 0)]...)
		c.overflow = true
		return len(p), nil
	}
	c.data = append(c.data, p...)
	return len(p), nil
}

// recordedNodeVersion is the one Node version the runner reported (process.version): every test
// that is not browser "none" must name it in its browser identity, and a present, non-empty
// report-level version must agree (PWP-V0-961). A browser "none" test names no version, so it
// constrains nothing when the report names one and leaves the version unknown when it does not.
// Any disagreement, a missing per-test version, or no source at all is "" (unobserved).
func recordedNodeVersion(tests []TestOutcome, reported *string) string {
	version := ""
	if reported != nil {
		version = *reported
	}
	reportNamed := version != ""
	for _, t := range tests {
		if reportNamed && t.Project != nil && t.Project.Browser == BrowserlessProject {
			continue
		}
		var use playwrightUseIdentity
		if t.Project == nil || json.Unmarshal(t.Project.Use, &use) != nil || use.CorvintBrowser.NodeVersion == nil || *use.CorvintBrowser.NodeVersion == "" {
			return ""
		}
		if version != "" && version != *use.CorvintBrowser.NodeVersion {
			return ""
		}
		version = *use.CorvintBrowser.NodeVersion
	}
	return version
}
