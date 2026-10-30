package frontier

import "github.com/Beamfall/corvint/internal/wp3codec"

// RenderError returns the one bounded stderr envelope CF-V0-022 allows:
// exactly `{"code":"CODE","profile":"frontier-error/0"}` plus one LF, carrying
// a code and nothing else. Nothing about the input reaches it, which is how
// CF-V0-024's no-echo rule is enforced structurally rather than by review.
func RenderError(err error) []byte {
	code := CodeOf(err)
	if code == "" {
		code = CodeInternalError
	}
	encoded, encodeErr := wp3codec.Encode(wp3codec.Object(
		wp3codec.Member{Key: "code", Value: wp3codec.String(code)},
		wp3codec.Member{Key: "profile", Value: wp3codec.String(ErrorProfile)},
	))
	if encodeErr != nil {
		return []byte(`{"code":"` + CodeInternalError + `","profile":"` + ErrorProfile + `"}` + "\n")
	}
	return append(encoded, '\n')
}

// ErrorExitCode is the CF-V0-004 operational-failure exit: 2, with no Frontier
// JSON on stdout.
const ErrorExitCode = 2

// errorMessages is the static human rendering of each operational code. Every
// message is a fixed string: CF-V0-022 permits one static message per code and
// forbids including unverified values in it.
var errorMessages = map[string]string{
	CodeInvalidInput:       "frontier input was invalid",
	CodeUnsupportedContext: "frontier context is not supported by this profile",
	CodeNoncanonical:       "frontier document was not canonical",
	CodeResourceExhausted:  "frontier exhausted a declared resource bound",
	CodeInterrupted:        "frontier was interrupted",
	CodeInternalError:      "frontier failed internally",
}

// RenderErrorHuman renders one operational failure for a person. An inherited
// upstream code has no Frontier-owned message, so it is rendered as itself
// rather than as an invented sentence.
func RenderErrorHuman(err error) string {
	code := CodeOf(err)
	if code == "" {
		code = CodeInternalError
	}
	if message, found := errorMessages[code]; found {
		return message + " (" + code + ")\n"
	}
	return "frontier failed (" + code + ")\n"
}
