// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// CMS-V0-912: for a fixture repository, `corvint flows ...` and `corvint how ...` through the
// shim, the built corvint-docs run directly, and the handlers called as corvint dispatched them
// before the move print the same stdout, stderr and exit status. The first two are separate
// processes; the third is the in-process baseline with corvint's own refusal envelope.
func TestCMSV0912FlowsAndHowParityWithCorvintDocs(t *testing.T) {
	t.Setenv("PATH", os.Getenv("PATH")) // restored after the test; BuildOnPATH prepends to it
	cleanup, err := testsupport.BuildOnPATH("github.com/Beamfall/corvint/cmd/corvint-docs", "corvint-docs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	binary, err := exec.LookPath("corvint-docs")
	if err != nil {
		t.Fatal(err)
	}
	shop := newShopFixture(t)
	app, mapFile := inferCLIRepo(t)
	how := []string{"--map", mapFile, "--source", "e2e", "--condition", "an order exists", "--full", "ship an order"}
	for _, test := range []struct {
		name, root string
		verb       string
		args       []string
		exit       int
	}{
		{"map", shop.root, "flows", []string{"map", "--flows", "flows", "--path", "profile/profile.go"}, 0},
		{"map all", shop.root, "flows", []string{"map", "--flows", "flows"}, 0},
		{"export inventory without an adapter anchor", shop.root, "flows", []string{"export", "--flows", "flows", "--emit", "inventory"}, 2},
		{"export provider", shop.root, "flows", []string{"export", "--flows", "flows", "--emit", "provider"}, 0},
		{"how", app, "how", how, 0},
		{"flows appmap how", app, "flows", append([]string{"appmap", "how"}, how...), 0},
		{"impact unknown base", shop.root, "flows", []string{"impact", "--flows", "flows", "--base", strings.Repeat("0", 40)}, 2},
		{"how without a map", app, "how", []string{"--source", "e2e", "ship"}, 2},
		{"unknown flows form", shop.root, "flows", []string{"no-such-verb"}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			argv := append([]string{"--root", test.root, test.verb}, test.args...)
			var shimOut, shimErr bytes.Buffer
			shimCode := runContext(context.Background(), argv, strings.NewReader(""), &shimOut, &shimErr)

			var appOut, appErr bytes.Buffer
			command := exec.Command(binary, argv...)
			command.Stdin, command.Stdout, command.Stderr = strings.NewReader(""), &appOut, &appErr
			appCode := 0
			var exit *exec.ExitError
			if err := command.Run(); errors.As(err, &exit) {
				appCode = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}

			handler := test.args
			if test.verb == "how" {
				handler = append([]string{"appmap", "how"}, test.args...)
			}
			nativeCode, nativeOut, nativeErr := runFlowsCLI(test.root, handler...)

			if shimCode != test.exit || appCode != test.exit || nativeCode != test.exit {
				t.Fatalf("exit shim=%d corvint-docs=%d handlers=%d, want %d\nshim stderr: %s", shimCode, appCode, nativeCode, test.exit, &shimErr)
			}
			if shimOut.String() != appOut.String() || appOut.String() != nativeOut {
				t.Fatalf("stdout differs\nshim: %q\ncorvint-docs: %q\nhandlers: %q", &shimOut, &appOut, nativeOut)
			}
			if shimErr.String() != appErr.String() || appErr.String() != nativeErr {
				t.Fatalf("stderr differs\nshim: %q\ncorvint-docs: %q\nhandlers: %q", &shimErr, &appErr, nativeErr)
			}
			if test.exit == 0 && shimOut.Len() == 0 {
				t.Fatal("a successful read printed nothing")
			}
		})
	}
}
