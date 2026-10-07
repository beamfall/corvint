package cli_test

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"strings"
	"testing"
	"time"
)

type unreadHelpInput struct{}

func (unreadHelpInput) Read([]byte) (int, error) { panic("help read stdin") }

// CAL-V0-047: every public leaf and parent family has help before any I/O.
func TestCALV0047_AllCommandHelpIsReadOnly(t *testing.T) {
	t.Run("CAL-V0-047 complete read-only help", func(t *testing.T) {
		initialized, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		names := map[string]bool{"--version": true}
		for _, name := range append(append([]string{}, cli.ReadVerbs...), cli.OmittedVerbs...) {
			names[name] = true
			if i := strings.IndexByte(name, ' '); i >= 0 {
				names[name[:i]] = true
			}
		}
		for _, root := range []string{t.TempDir(), initialized} {
			before := fixture.TreeSnapshot(t, root)
			for name := range names {
				for _, flag := range [][]string{{"--help"}, {"-h"}, {"--help", "--verbose"}, {"--verbose", "-h"}} {
					args := append(strings.Fields(name), flag...)
					verbose := len(flag) == 2
					var out, errb bytes.Buffer
					code := cli.Run(cli.Env{Cwd: root, Args: args, Stdin: unreadHelpInput{}, Stdout: &out, Stderr: &errb})
					r, e := wire.DecodeResult(out.Bytes())
					if e != nil || code != 0 || r.Outcome != wire.OutcomeOK || r.Snapshot != nil || r.Mutation != nil || errb.Len() != 0 {
						t.Fatalf("%v: %s %s (%v)", args, out.Bytes(), errb.Bytes(), e)
					}
					if name != "help" && (field(r.Items[0], "usage").Str == "" || len(field(r.Items[0], "flags").Arr) == 0) {
						t.Fatalf("command-specific help missing: %v %s", args, out.Bytes())
					}
					for _, f := range field(r.Items[0], "flags").Arr {
						if strings.ContainsAny(f.Str, "[]=") {
							t.Fatalf("help flag %q is not a bare flag name: %v", f.Str, args)
						}
					}
					// CAL-V0-170: terse help carries only the call-forming keys
					// and points at the verbose form when it left anything out.
					if !verbose && name != "help" {
						for _, key := range r.Items[0].Obj.Keys {
							if !strings.Contains(" usage implemented flags operation payloadKeys optionalPayloadKeys note verboseHelp ", " "+key+" ") {
								t.Fatalf("terse help carries %q: %v", key, args)
							}
						}
					}
					if name == "release" && verbose == (len(field(r.Items[0], "handoffPreconditions").Arr) == 0) {
						t.Fatalf("handoff contract must appear only in verbose help: %v", args)
					}
					if name == "release" && !verbose && field(r.Items[0], "verboseHelp").Str != "corvint-tasks release --help --verbose" {
						t.Fatalf("terse release help lacks its verbose pointer: %s", out.Bytes())
					}
					if strings.HasPrefix(name, "criterion-binding ") && verbose {
						var flags []string
						for _, value := range field(r.Items[0], "flags").Arr {
							flags = append(flags, value.Str)
						}
						want := "--help,-h"
						if name == "criterion-binding capture" {
							want = "--attempt,--help,--ticket,-h"
						}
						if strings.Join(flags, ",") != want || !strings.Contains(field(r.Items[0], "note").Str, "canonical capture") || !strings.Contains(field(r.Items[0], "note").Str, "stdin") {
							t.Fatalf("criterion-binding input contract missing: %v %s", args, out.Bytes())
						}
					}
					if os.Getenv("CORVINT_HANDOFF_TEST_BINARY") != "" {
						actual := handoffCLI(t, root, args...)
						if actual.code != 0 || actual.res.Snapshot != nil || actual.res.Mutation != nil {
							t.Fatalf("actual help: %v %s", args, actual.stdout)
						}
					}
				}
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) {
				t.Fatal("help wrote files")
			}
		}
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"ticket", "create", "--payload-stdin", "--help"}, Stdin: unreadHelpInput{}, Stdout: &out})
		r, e := wire.DecodeResult(out.Bytes())
		if e != nil || code != 0 || field(r.Items[0], "operation").Str != "CREATE" || len(field(r.Items[0], "payloadKeys").Arr) == 0 {
			t.Fatal("legacy mutation help changed")
		}
		// CAL-V0-170: --verbose is accepted beside the in-parser mutation
		// help, which is already the full text, and refused without --help.
		var verbose bytes.Buffer
		code = cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"ticket", "create", "--payload-stdin", "--help", "--verbose"}, Stdin: unreadHelpInput{}, Stdout: &verbose})
		if code != 0 || !bytes.Equal(verbose.Bytes(), out.Bytes()) {
			t.Fatalf("verbose mutation help: %s", verbose.Bytes())
		}
		var bare bytes.Buffer
		code = cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"ticket", "create", "--payload-stdin", "--verbose"}, Stdin: unreadHelpInput{}, Stdout: &bare})
		if r, e := wire.DecodeResult(bare.Bytes()); e != nil || code == 0 || r.Outcome == wire.OutcomeOK || !strings.Contains(strings.Join(r.Warnings, ";"), "--verbose") {
			t.Fatalf("--verbose without --help: %s", bare.Bytes())
		}

	})
}

func TestCALV0047_MalformedInputsStillRefuse(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"release", "--evidence", ""},
		{"unknown", "--help"}, {"ticket", "unknown", "--help"}, {"release", "unknown", "--help"},
		{"plan", "preview", "--unknown"}, {"submit", "--unknown"},
		{"ticket", "create", "--payload", "--help"}, {"ticket", "create", "--payload", "-h"},
	} {
		r := atm(t, root, nil, args...)
		if r.code == 0 {
			t.Fatalf("malformed input became help: %v %s", args, r.stdout)
		}
	}
	for _, args := range [][]string{{"version"}, {"--version"}} {
		r := atm(t, root, nil, args...)
		if r.code != 0 || field(r.res.Items[0], "version").Str == "" {
			t.Fatal("version alias changed")
		}
	}
}

func TestPSRPublicRouteAndHelp(t *testing.T) {
	for _, args := range [][]string{{"pool", "sweep", "--help", "--verbose"}, {"pool", "sweep", "--verbose", "-h"}} {
		var out bytes.Buffer
		if cli.Run(cli.Env{Cwd: t.TempDir(), Args: args, Stdin: unreadHelpInput{}, Stdout: &out}) != 0 {
			t.Fatal(out.String())
		}
		result, err := wire.DecodeResult(out.Bytes())
		if err != nil || !strings.Contains(field(result.Items[0], "usage").Str, "--timeout-seconds") || !strings.Contains(field(result.Items[0], "note").Str, "Private logs") {
			t.Fatal(result, err)
		}
	}
}
