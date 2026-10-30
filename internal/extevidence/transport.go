package extevidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Command transport bounds (EEP-TR-004). One launch per selected command,
// zero retries, one record per launch.
const (
	CommandTimeout        = 10 * time.Second
	MaxCommandArguments   = 32
	MaxCommandArgvBytes   = 4096
	maxCommandStderrBytes = 64 << 10
)

// commandTimeout is CommandTimeout; tests shorten it to observe the kill.
var commandTimeout = CommandTimeout

// commandSourcePrefix marks a source produced only by ParseCommand. A
// `--provider` value is one argv element and can never hold a NUL byte, so no
// file source reaches the command transport (EEP-TR-001).
const commandSourcePrefix = "\x00command\x00"

// ParseCommand reads a `--provider-command` value: a JSON array of 1 to
// MaxCommandArguments strings whose first element is an absolute, clean
// executable path. It returns the provider source that selects it (EEP-TR-002).
func ParseCommand(value string) (string, error) {
	if len(value) > MaxCommandArgvBytes {
		return "", fmt.Errorf("argv exceeds %d bytes", MaxCommandArgvBytes)
	}
	if !utf8.ValidString(value) {
		return "", errors.New("argv is not valid UTF-8")
	}
	var argv []string
	decoder := json.NewDecoder(strings.NewReader(value))
	if err := decoder.Decode(&argv); err != nil {
		return "", errors.New("expected a JSON array of strings")
	}
	if decoder.More() {
		return "", errors.New("trailing content after the JSON array")
	}
	if len(argv) == 0 || len(argv) > MaxCommandArguments {
		return "", fmt.Errorf("argv must hold 1 to %d strings", MaxCommandArguments)
	}
	for _, argument := range argv {
		if strings.IndexByte(argument, 0) >= 0 {
			return "", errors.New("argv must not contain NUL bytes")
		}
	}
	if !filepath.IsAbs(argv[0]) || filepath.Clean(argv[0]) != argv[0] {
		return "", errors.New("the executable must be an absolute, clean path; no PATH lookup is made")
	}
	encoded, _ := json.Marshal(argv)
	return commandSourcePrefix + string(encoded), nil
}

// commandArgv reports whether source selects the command transport.
func commandArgv(source string) ([]string, bool) {
	encoded, isCommand := strings.CutPrefix(source, commandSourcePrefix)
	if !isCommand {
		return nil, false
	}
	var argv []string
	if json.Unmarshal([]byte(encoded), &argv) != nil {
		return nil, false
	}
	return argv, true
}

// loadCommand runs one contained provider command and hands its complete
// stdout to the same decode the file transport uses (EEP-TR-005). Any
// transport failure is a provider row with no record (EEP-TR-006).
func loadCommand(ctx context.Context, root rootRepository, argv []string) provider {
	encoded, _ := json.Marshal(argv)
	entry := provider{source: "command:" + string(encoded), state: StateUnavailable}
	data, state, reason := runCommand(ctx, root.dir, argv)
	if reason != "" {
		entry.state, entry.reason = state, reason
		return entry
	}
	return decodeRecord(ctx, root, entry, data)
}

// runCommand launches argv once in its own process group with a scrubbed
// environment, empty stdin, and the repository root as working directory
// (EEP-TR-003, EEP-TR-004). stderr is bounded and never returned.
func runCommand(ctx context.Context, dir string, argv []string) ([]byte, string, string) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, StateUnavailable, "command did not start: repository root unavailable"
	}
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: argv, Dir: filepath.Clean(absolute), Env: commandEnvironment(), Stdin: nil,
		Timeout: commandTimeout, OutputLimit: MaxRecordBytes, StderrLimit: maxCommandStderrBytes,
	})
	state, reason := commandFailure(observation)
	return observation.Stdout, state, reason
}

// commandFailure names the first transport failure in a fixed precedence;
// an empty reason means the command completed and stdout is the record.
func commandFailure(observation procgroup.Observation) (string, string) {
	switch {
	case !observation.Started:
		return StateUnavailable, "command did not start"
	case observation.TimedOut:
		return StateUnavailable, fmt.Sprintf("command exceeded %s wall time; process group killed", commandTimeout)
	case observation.Cancelled:
		return StateUnavailable, "command cancelled"
	case observation.StdoutOverflow:
		return StateInvalid, fmt.Sprintf("record exceeds %d bytes", MaxRecordBytes)
	case observation.StderrOverflow:
		return StateInvalid, fmt.Sprintf("command stderr exceeds %d bytes", maxCommandStderrBytes)
	case observation.ExitObserved && observation.ExitStatus != 0:
		return StateUnavailable, fmt.Sprintf("command exited with status %d", observation.ExitStatus)
	case !observation.ExitObserved:
		return StateUnavailable, "command exit not observed"
	case observation.Err != nil || !observation.PipesDrained || !observation.OwnedProcessGroupCleanup:
		return StateUnavailable, "command process group not proven cleaned up"
	}
	return "", ""
}

// commandEnvironment is the whole child environment: an executable search
// path, a temporary directory, and a fixed locale. No HOME, credential, proxy,
// or Corvint variable passes (EEP-TR-003).
func commandEnvironment() []string {
	environment := make([]string, 0, 4)
	for _, name := range []string{"PATH", "TMPDIR"} {
		if value, exists := os.LookupEnv(name); exists && !strings.ContainsRune(value, 0) {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment, "LANG=C", "LC_ALL=C")
}
