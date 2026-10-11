package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// TestProviderMainProcess runs main with the arguments in CORVINT_JS_PROVIDER_ARGS when the help
// test re-executes this test binary; otherwise it does nothing.
func TestProviderMainProcess(t *testing.T) {
	args, ok := os.LookupEnv("CORVINT_JS_PROVIDER_ARGS")
	if !ok {
		return
	}
	os.Args = append([]string{"corvint-js-test-provider"}, strings.Split(args, "\x1f")...)
	main()
	os.Exit(0)
}

func runProviderMain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestProviderMainProcess$")
	command.Env = append(os.Environ(), "CORVINT_JS_PROVIDER_ARGS="+strings.Join(args, "\x1f"))
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

// LPCV-V0-071: --help, -h and help exit 0 and print the usage of every subcommand, including
// each subcommand's flags, without running any test.
func TestProviderHelpPrintsEverySubcommandUsage(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		code, stdout, stderr := runProviderMain(t, arg)
		if code != 0 || stderr != "" {
			t.Fatalf("%s exited %d: %s", arg, code, stderr)
		}
		for _, want := range []string{
			"unit ", "e2e ", "qualify-keep-reporters ", "negate ",
			"Usage of unit:", "Usage of e2e:", "Usage of negate:",
			"-external-server", "-retain", "-keep-reporters-qualification", "-all-steps", "-runner-version",
			".corvint/test-evidence", "discover-playwright", "witness-playwright",
		} {
			if !strings.Contains(stdout, want) {
				t.Errorf("%s usage omits %q", arg, want)
			}
		}
		if strings.Contains(stdout, "unknown subcommand") || strings.Contains(stdout, "infrastructure failure") {
			t.Fatalf("%s ran a subcommand: %s", arg, stdout)
		}
	}
	if code, _, stderr := runProviderMain(t, "--bogus"); code != 2 || !strings.Contains(stderr, "unknown subcommand") {
		t.Fatalf("unknown subcommand exited %d: %s", code, stderr)
	}
}

// DCP-V1-053: the producer step of corvint docs help's witness-playwright example uses only e2e
// flags and parses through the real e2e flag set into a configuration that carries every field
// external-mode admission requires (internal/jstestprovider admitExternal), so it produces a
// retained qualified receipt rather than a refusal.
func TestDocsHelpProducerExampleUsesE2EFlags(t *testing.T) {
	source, err := os.ReadFile("../corvint/docs.go")
	if err != nil {
		t.Fatal(err)
	}
	var usage bytes.Buffer
	printUsage(&usage)
	text := usage.String()
	begin, end := strings.Index(text, "Usage of e2e:"), strings.Index(text, "Usage of negate:")
	if begin < 0 || end < begin {
		t.Fatal("usage has no bounded e2e section")
	}
	e2e := text[begin:end]
	found := false
	for _, line := range strings.Split(string(source), "\n") {
		if !strings.HasPrefix(line, "    corvint-js-test-provider e2e ") {
			continue
		}
		found = true
		fields := strings.Fields(line)
		if n := len(fields); n > 2 && fields[n-2] == ">" {
			fields = fields[:n-2]
		}
		args := fields[2:]
		for _, field := range args {
			name := strings.TrimPrefix(field, "--")
			if name != field && !strings.Contains(e2e, "  -"+name+" ") && !strings.Contains(e2e, "  -"+name+"\n") {
				t.Errorf("example flag %s is not an e2e flag", field)
			}
		}
		var observed jstestprovider.E2EConfig
		stop := errors.New("example stopped before running")
		previous := runE2EReceipt
		runE2EReceipt = func(_ context.Context, cfg jstestprovider.E2EConfig) (jstestprovider.Receipt, error) {
			observed = cfg
			return jstestprovider.Receipt{}, stop
		}
		err := runE2E(args)
		runE2EReceipt = previous
		if !errors.Is(err, stop) {
			t.Fatalf("example arguments refused before the run: %v", err)
		}
		if !observed.ExternalServer || observed.ServerReadyURL == "" || strings.TrimSpace(observed.AppIdentity) == "" || observed.ConfigFile == "" || len(observed.TestFiles) == 0 || (observed.RunnerVersion != "1.60.0" && observed.RunnerVersion != "1.63.0") || len(observed.ServerArgv) != 0 {
			t.Fatalf("example misses an external-mode admission field: %+v", observed)
		}
		if !strings.Contains(line, " --retain ") {
			t.Fatalf("example does not retain the receipt: %s", line)
		}
	}
	if !found {
		t.Fatal("docs help has no corvint-js-test-provider e2e example")
	}
}

// PWP-V0-019: the help states that --test-arg selects what runs while --test-file binds identity,
// that setup files reported through project dependencies need their own --test-file, and that
// the refusal names each unbound file.
func TestProviderHelpExplainsSetupProjectFileBinding(t *testing.T) {
	var usage bytes.Buffer
	printUsage(&usage)
	text := usage.String()
	for _, want := range []string{
		"--test-arg (what Playwright runs) and --test-file (what identity\nbinds) are independent",
		"project dependencies",
		"each setup file needs its own --test-file",
		"test-file-unbound detail that names each unbound file",
		"No file is bound unless it is passed with --test-file.",
		"including setup files that project dependencies run",
		"binds no file identity",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("usage omits %q", want)
		}
	}
}
