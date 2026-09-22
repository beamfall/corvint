package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const nativeHookMaxInput = 131072
const nativeActiveEnrollment = "/Library/CorvintAuthority/public/active-enrollment.json"

var nativeReleaseExecutable = regexp.MustCompile(`^/Library/CorvintAuthority/versions/[0-9a-f]{64}/corvint$`)
var nativeHookDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func runNativeHook(parent context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	direct := len(args) == 1 && args[0] == "--qualified-direct-lifecycle"
	qualified := direct || (len(args) == 1 && args[0] == "--qualified-lifecycle")
	fallback := func(reason string) int {
		message := "Corvint protected authority unavailable; Frontier UNKNOWN. Native qualification is incomplete."
		if qualified {
			message = "Corvint qualified lifecycle FALLBACK: candidate or completed native qualification is unavailable; no authority was computed."
		}
		if qualified && reason == "corvint-invocation-timeout" {
			message = "Corvint qualified lifecycle FALLBACK: corvint-invocation-timeout: the 1600 ms post-bootstrap adapter deadline was exceeded. This is a time bound, not a diagnosed fault. Unrelated coding continues."
		}
		if emit(stdout, map[string]any{"systemMessage": message}) != nil {
			return 2
		}
		return 0
	}
	if len(args) != 0 && !qualified {
		return fallback("invalid-mode")
	}
	executable, err := os.Executable()
	if err != nil || validateNativeHookExecutable(executable) != nil {
		return fallback("consumer-unadmitted")
	}
	environment := nativeHookEnvironment(executable)
	if !exactNativeHookEnvironment(os.Environ(), environment) {
		// Replacing this process preserves the original adapter's closed execve
		// boundary without adding a supervisor or changing global environment in tests.
		if replaceNativeHook(executable, append([]string{executable, "native-hook"}, args...), environment) != nil {
			return fallback("consumer-unadmitted")
		}
		return 2
	}
	// The host retains its 2 s watchdog; this context covers normalized input and
	// the consumer together after the bounded, fixed-path bootstrap.
	ctx, cancel := context.WithTimeout(parent, 1600*time.Millisecond)
	defer cancel()
	raw, err := readInputBounded(ctx, stdin, nativeHookMaxInput)
	if err != nil {
		if ctx.Err() != nil {
			return fallback("corvint-invocation-timeout")
		}
		return fallback("input-unavailable")
	}
	handle := ""
	if !qualified {
		handle, err = readNativeEnrollment()
		if err != nil {
			return fallback("not-enrolled")
		}
	}
	request, err := normalizeNativeHook(raw, qualified, handle)
	if err != nil {
		return fallback("invalid-input")
	}
	if direct {
		request["profile"] = directQualifiedLifecycleProfile
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return fallback("invalid-input")
	}
	consumerArgs := []string{"--input", "-", "--native-output"}
	if qualified {
		return runQualifiedLifecycle(ctx, consumerArgs, bytes.NewReader(encoded), stdout, stderr)
	}
	return runAuthorityEvent(ctx, consumerArgs, bytes.NewReader(encoded), stdout, stderr)
}
func validateNativeHookExecutable(path string) error {
	if !nativeReleaseExecutable.MatchString(path) || filepath.Clean(path) != path {
		return errors.New("consumer-unadmitted")
	}
	current := "/"
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("consumer-unadmitted")
		}
	}
	return nil
}
func nativeHookEnvironment(executable string) []string {
	return []string{"LANG=C.UTF-8", "PATH=" + filepath.Dir(executable) + "/bin"}
}
func exactNativeHookEnvironment(actual, want []string) bool {
	if len(actual) != 2 {
		return false
	}
	return actual[0] == want[0] && actual[1] == want[1] || actual[0] == want[1] && actual[1] == want[0]
}
func readNativeEnrollment() (string, error) {
	raw, err := readNativeHookFile(nativeActiveEnrollment, 1024)
	if err != nil {
		return "", err
	}
	value, err := decodeAdapterJSON(raw)
	if err != nil || len(value) != 2 || value["profile"] != "corvint-protected-active-enrollment/0" {
		return "", errors.New("not-enrolled")
	}
	handle, ok := value["enrollmentHandle"].(string)
	if !ok || !nativeHookDigest.MatchString(handle) {
		return "", errors.New("not-enrolled")
	}
	return handle, nil
}
func normalizeNativeHook(raw []byte, qualified bool, handle string) (map[string]any, error) {
	invalid := errors.New("invalid-native-hook-input")
	if len(raw) > nativeHookMaxInput || !utf8.Valid(raw) {
		return nil, invalid
	}
	value, err := decodeAdapterJSON(raw)
	if err != nil {
		return nil, invalid
	}
	eventName, _ := value["hook_event_name"].(string)
	if !qualified {
		active, ok := value["stop_hook_active"].(bool)
		if eventName != "Stop" || !ok || !nativeHookDigest.MatchString(handle) {
			return nil, invalid
		}
		return map[string]any{"profile": "corvint-authority-event/0", "event": "stop", "enrollmentHandle": handle, "stopHookActive": active}, nil
	}
	event := map[string]string{"SessionStart": "session-start", "UserPromptSubmit": "user-prompt", "Stop": "stop", "SessionEnd": "session-end"}[eventName]
	if event == "" {
		return nil, invalid
	}
	input := map[string]any{}
	if value["session_id"] != nil {
		session, ok := value["session_id"].(string)
		if !ok || len(session) > 4096 || !utf8.ValidString(session) {
			return nil, invalid
		}
		if session != "" {
			sum := sha256.Sum256([]byte("corvint-local-completion-session/0\x00" + session))
			input["sessionIdSha256"] = hex.EncodeToString(sum[:])
		}
	}
	switch event {
	case "session-start":
		if source, exists := value["source"]; exists {
			if source != "startup" && source != "resume" && source != "clear" && source != "compact" {
				return nil, invalid
			}
			input["startSource"] = source
		}
	case "user-prompt":
		task, ok := value["prompt"].(string)
		if !ok {
			return nil, invalid
		}
		task = contextindex.TrimPythonSpace(task)
		if task == "" || utf8.RuneCountInString(task) > 2000 || len(task) > 16384 {
			return nil, invalid
		}
		input["task"] = task
	case "stop":
		active, ok := value["stop_hook_active"].(bool)
		if !ok {
			return nil, invalid
		}
		input["stopHookActive"], input["changedPaths"] = active, []string{}
	case "session-end":
		input["openedPaths"], input["changedPaths"], input["verification"] = []string{}, []string{}, []any{}
	}
	return map[string]any{"profile": qualifiedLifecycleProfile, "event": event, "input": input}, nil
}
