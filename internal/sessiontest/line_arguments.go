// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// pytestArguments runs only at launch, after execution is authorized. Planning
// writes no argument file and keeps the conservative argv-limit fallback. Pytest
// added @file in 8.2; an unavailable or older version keeps that fallback.
func pytestArguments(ctx context.Context, plan *contractPlan, command contract.Command, root string, env []string, report string) contract.Command {
	request := plan.request
	if request.Instance.Runner != testid.Pytest || command.Reason != contract.ReasonArgvLimit || len(request.Full) == 0 {
		return command
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	argv := append(append([]string(nil), request.Full...), "--version")
	if !contract.ArgvFits(argv) {
		return command
	}
	probe := exec.CommandContext(probeCtx, argv[0], argv[1:]...)
	probe.Dir, probe.Env = filepath.Join(root, filepath.FromSlash(request.Instance.Root)), env
	probe.WaitDelay = time.Second
	var output providerCapture
	probe.Stdout, probe.Stderr = &output, &output
	if err := probe.Run(); err != nil || output.overflow {
		return command
	}
	version := contract.PytestVersion(output.data)
	return pytestArgumentsForVersion(request, command, version, report)
}

func pytestArgumentsForVersion(request contract.Request, command contract.Command, version, report string) contract.Command {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return command
	}
	major, e1 := strconv.Atoi(parts[0])
	minor, e2 := strconv.Atoi(parts[1])
	_, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil || major < 8 || major == 8 && minor < 2 {
		return command
	}
	// The 1.0 Python contract remains file-level, even when an input names methods.
	var files []string
	exact := true
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return command
		}
		if len(id.Segments) != 0 {
			exact = false
		}
		id.Segments = nil
		files = append(files, id.String())
	}
	nodes, err := contract.PytestNodes(request.Instance, files)
	if err != nil {
		return command
	}
	for i, node := range nodes {
		if strings.ContainsAny(node, "\r\n\x00") {
			return command
		}
		if strings.HasPrefix(node, "@") {
			nodes[i] = "./" + node
		}
	}
	name := filepath.Join(filepath.Dir(report), "selected.args")
	argv := append(append(append([]string(nil), request.Full...), contract.PytestReportArgs(report)...), "@"+name)
	if !contract.ArgvFits(argv) {
		return command
	}
	body := strings.Join(nodes, "\n") + "\n"
	if len(body) > 8<<20 {
		return command
	}
	if err := os.WriteFile(name, []byte(body), 0600); err != nil {
		return command
	}
	return contract.Command{Argv: argv, Dir: request.Instance.Root, Report: report, Tier: contract.TierFile, Exact: exact, Reason: contract.ReasonArgvLimit}
}
