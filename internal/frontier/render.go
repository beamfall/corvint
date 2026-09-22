package frontier

import (
	"strings"

	"github.com/Beamfall/corvint/internal/wp3codec"
)

// RenderJSON returns the exact canonical document bytes. CF-V0-026 makes JSON
// and human two renderings of ONE computation, so both take a sealed Document
// and neither recomputes anything.
func RenderJSON(document Document) ([]byte, error) { return CanonicalBytes(document) }

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

// RenderHuman renders the same computation as RenderJSON for a person. It
// emits only the verified fields of the valid result, and it states the
// CF-V0-025 assertion boundary in full so the queue cannot be read as proof
// that evidence does not exist, that code is wrong, that a test lacks
// behavioral coverage, or that the repository was exhaustively searched.
func RenderHuman(document Document) string {
	var out strings.Builder
	out.WriteString("frontier/0 " + document.FrontierState + " policy=" + Policy +
		" testMode=" + string(document.Inputs.TestMode) + "\n")
	out.WriteString("universe " + document.UniverseID + "\n")
	out.WriteString("id " + document.ID + "\n")
	for _, item := range document.Items {
		out.WriteString(renderItemHuman(item))
	}
	out.WriteString(assertionBoundary)
	return out.String()
}

func renderItemHuman(item Item) string {
	var out strings.Builder
	out.WriteString(item.Kind + " " + item.SubjectID + "\n")
	out.WriteString("  reasons " + strings.Join(item.Reasons, ",") + "\n")
	out.WriteString("  authority " + item.AuthorityClass +
		" resolution " + item.ResolutionClass +
		" next " + item.NextAction + "\n")
	if len(item.RelatedIDs) > 0 {
		out.WriteString("  related " + strings.Join(item.RelatedIDs, ",") + "\n")
	}
	return out.String()
}

// assertionBoundary is the exact CF-V0-025 disclaimer. It ships with the human
// rendering because that is the rendering a person reads and misreads.
const assertionBoundary = "each item asserts only that no qualifying relation was accepted for its named " +
	"obligation after deterministic verification of the declared inputs and profiles; it is not proof " +
	"that evidence does not exist, that code is wrong, that a test lacks behavioral coverage, that a " +
	"capability is impossible, or that the repository was exhaustively searched\n"
