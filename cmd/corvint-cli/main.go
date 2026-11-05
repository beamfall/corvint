// SPDX-License-Identifier: Apache-2.0
// corvint-cli is the opt-in, offline discovery and refusal adapter for native binaries.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/cliprofile"
)

const usage = `usage: corvint-cli --binary BINARY [ARGS...]
Serve uniform help, catalogs, release versions and coded CLI refusals without changing native defaults.
Required: --binary names a shipped binary; remaining arguments belong to that binary.
Example: corvint-cli --binary corvint-analyzer-go --help
Use help --json after the binary for its complete command tree.
Native execution is limited to short CLI operations (30 seconds, 1 MiB per output stream);
protocol/server entrypoints support discovery only. Use their native binary for protocol traffic.`

func main() {
	if code, handled := cliapp.Answer("corvint-cli", os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	path, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(cliprofile.Profile(ctx, filepath.Dir(path), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
