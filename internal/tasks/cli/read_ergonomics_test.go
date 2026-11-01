package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0870_UnfilteredSearchNamesTicketList: an unfiltered search is the
// defined MALFORMED usage error before any read, naming the unfiltered read.
func TestCTSV0870_UnfilteredSearchNamesTicketList(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	x := atm(t, root, nil, "ticket", "search")
	if x.code != 1 || x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil || !slices.Equal(x.res.Codes, []string{wire.CodeMalformed}) ||
		!warningsContain(x.res.Warnings, "MALFORMED: argv:") || !warningsContain(x.res.Warnings, "`ticket list` is the unfiltered read") {
		t.Fatalf("unfiltered search: %s", x.stdout)
	}
}

// TestCTSV0871_JSONIsANoOpAndRootPrecedesTheVerb: --json changes nothing on
// a compact read, a leading --root runs the command from that directory, and
// ticket show help names the revision and acceptance-criteria paths.
func TestCTSV0871_JSONIsANoOpAndRootPrecedesTheVerb(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, local := tcdTicket(t, root, "ergo", "")
	plain := atm(t, root, nil, "ticket", "show", local)
	if plain.res.Outcome != wire.OutcomeOK {
		t.Fatalf("show: %s", plain.stdout)
	}
	for _, args := range [][]string{{"ticket", "show", local, "--json"}, {"ticket", "show", "--json", local}} {
		if x := atm(t, root, nil, args...); x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || x.res.Items[0].Obj == nil {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if x := atm(t, root, nil, "ticket", "search", "--status", "OPEN", "--json", "--summary"); x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("search --json: %s", x.stdout)
	}
	parent := filepath.Dir(root)
	for _, args := range [][]string{{"--root", root}, {"--root=" + root}, {"--root", filepath.Base(root)}} {
		x := atm(t, parent, nil, append(args, "ticket", "show", local)...)
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "revision").Str != field(plain.res.Items[0], "revision").Str {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	for _, args := range [][]string{{"--root"}, {"--root="}, {"--root", "--json", "ticket", "show", local}} {
		x := atm(t, parent, nil, args...)
		if x.res.Outcome != wire.OutcomeError || !warningsContain(x.res.Warnings, "--root needs a directory: corvint-tasks --root DIR <verb>") {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	help := atm(t, root, nil, "ticket", "show", "--help")
	for _, want := range []string{"items[0].revision", "items[0].record.acceptanceCriteria", "--json is a no-op", "[--root DIR]"} {
		if !bytes.Contains(help.stdout, []byte(want)) {
			t.Fatalf("ticket show help lacks %q: %s", want, help.stdout)
		}
	}
}

// TestCTSV0872_ClosedStdoutPipeKeepsTheOutcomeStatus: the compiled binary
// writing into a pipe whose reader is gone neither dies of SIGPIPE nor
// reports an error; the exit status is the read's own.
func TestCTSV0872_ClosedStdoutPipeKeepsTheOutcomeStatus(t *testing.T) {
	bin := cli.CompiledTasksBinary(t)
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, local := tcdTicket(t, root, "pipe", "")
	for _, c := range []struct {
		args []string
		code int
	}{{[]string{"ticket", "show", local}, 0}, {[]string{"ticket", "search", "--status", "OPEN"}, 0}, {[]string{"ticket", "show", "ZZ-9999"}, 1}} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		r.Close()
		var stderr bytes.Buffer
		cmd := exec.Command(bin, c.args...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = root, w, &stderr
		err = cmd.Run()
		w.Close()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode() // -1 when a signal killed it
		} else if err != nil {
			t.Fatal(err)
		}
		if code != c.code || stderr.Len() != 0 {
			t.Fatalf("%v into a closed pipe: exit %d (%v), want %d; stderr %q", c.args, code, err, c.code, stderr.String())
		}
	}
}

// TestCTSV0873_UnpaddedTokenResolvesTheOnePaddedTicket: QB-1 reads QB-0001;
// the exact token still reads and a number no ticket pads to is not found.
func TestCTSV0873_UnpaddedTokenResolvesTheOnePaddedTicket(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id, local := tcdTicket(t, root, "pad", "")
	m := regexp.MustCompile(`^(.*-)0+([1-9][0-9]*)$`).FindStringSubmatch(local)
	if m == nil {
		t.Fatalf("local token %q has no zero-padded number", local)
	}
	unpadded := m[1] + m[2]
	for _, arg := range []string{local, unpadded, id} {
		for _, verb := range []string{"show", "blockers"} {
			x := atm(t, root, nil, "ticket", verb, arg)
			if x.res.Outcome != wire.OutcomeOK || !bytes.Contains(x.stdout, []byte(id)) {
				t.Fatalf("ticket %s %s: %s", verb, arg, x.stdout)
			}
		}
	}
	missing := m[1] + m[2] + "9"
	if x := atm(t, root, nil, "ticket", "show", missing); x.res.Outcome != wire.OutcomeRefused || !warningsContain(x.res.Warnings, "does not exist in this queue") || strings.Contains(strings.Join(x.res.Warnings, " "), "it names") {
		t.Fatalf("absent unpadded token: %s", x.stdout)
	}
}
