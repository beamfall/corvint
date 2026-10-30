package changee2e

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// runLoss is the concrete account of one run that ended without a qualified receipt: an
// unqualified side pool, or one BBF control attempt that read infrastructure_failed
// (CCE-V0-116). It is diagnostic only: no verdict, status, score or note reads it.
type runLoss struct {
	// Attempt is the BBF attempt number of a control attempt; absent for a side.
	Attempt int `json:"attempt,omitempty"`
	// AttemptReasons are BBF's own reasons for the attempt (controls only).
	AttemptReasons []string `json:"attempt_reasons,omitempty"`
	// Reason is the qualify reason of the run, empty when the attempt kept no hook receipt.
	Reason string `json:"reason,omitempty"`
	// ErrorLocation is a run-level error's receipt location as a tree-relative `path:line`,
	// outside-test-tree, or unlocated; absent for any other loss.
	ErrorLocation string `json:"error_location,omitempty"`
	// ErrorMessage is the run's provider or run-level error text, ANSI-stripped, with absolute
	// paths removed, secret-screened and bounded to lossMessageBytes.
	ErrorMessage string `json:"error_message,omitempty"`
	// HookProcess summarises a control attempt's hook process when the attempt kept no receipt.
	HookProcess string `json:"hook_process,omitempty"`
}

const (
	// lossMessageBytes bounds a runLoss message and location (CCE-V0-116).
	lossMessageBytes = 512
	// locationUnlocated is the error location of a run-level error the receipt does not locate.
	locationUnlocated = "unlocated"
	// hookDetailBytes is BBF's bound on an infrastructure receipt's detail; a longer detail makes
	// the attempt an invalid control.
	hookDetailBytes = 4096
)

var (
	// fileURL matches a file URL to an absolute host path, line and column included.
	fileURL = regexp.MustCompile(`file:///[^\s'"()\[\]]*`)
	// absolutePath matches an absolute POSIX path that starts a word, so a URL's path is kept.
	absolutePath = regexp.MustCompile(`(^|[\s'"(\[=])/[^\s'"()\[\]:]+`)
	// windowsPath matches an absolute drive path that starts a word.
	windowsPath = regexp.MustCompile(`(^|[\s'"(\[=])[A-Za-z]:\\[^\s'"()\[\]]+`)
	// pathContinuation matches the words after a replaced path that still hold a separator, the
	// rest of a path that contained spaces.
	pathContinuation = regexp.MustCompile(`<path>(?: [^\s'"()\[\]:]*[/\\][^\s'"()\[\]:]*)+`)
)

// runErrorDetail is the location and bounded message of the error that lost a run: the receipt's
// provider failure (a run-level reporter error carries its PWP-V3-009 location, or unlocated),
// or else the provider's own run error.
func runErrorDetail(r jstestprovider.Receipt, runErr error, roots []string) (location, message string) {
	switch {
	case r.Infrastructure != nil:
		message = lossText(r.Infrastructure.Detail, roots)
		if r.Infrastructure.Reason == jstestprovider.ReasonReporterGlobalError {
			location = locationUnlocated
			if l := r.Infrastructure.Location; l != nil && l.Line >= 1 {
				location = lossText(locationText(treeLocation(roots, *l), l.Line), roots)
			}
		}
	case runErr != nil:
		message = lossText(runErr.Error(), roots)
	}
	return location, message
}

// sideLoss is an unqualified side's loss, nil for a qualified side.
func sideLoss(reason string, r jstestprovider.Receipt, runErr error, roots []string) *runLoss {
	if reason == "" {
		return nil
	}
	location, message := runErrorDetail(r, runErr, roots)
	return &runLoss{Reason: reason, ErrorLocation: location, ErrorMessage: message}
}

// lossText makes free provider text safe to keep: ANSI escapes stripped, the staged tree's roots
// made relative, any other absolute path replaced, secrets redacted, and the result bounded.
func lossText(text string, roots []string) string {
	text = ansiEscape.ReplaceAllString(text, "")
	sorted := slices.Clone(roots)
	slices.SortFunc(sorted, func(a, b string) int { return len(b) - len(a) })
	for _, root := range sorted {
		if root != "" {
			text = strings.ReplaceAll(text, root+"/", "")
			text = strings.ReplaceAll(text, root, ".")
		}
	}
	text = fileURL.ReplaceAllString(text, "<path>")
	text = absolutePath.ReplaceAllString(text, "${1}<path>")
	text = windowsPath.ReplaceAllString(text, "${1}<path>")
	text = pathContinuation.ReplaceAllString(text, "<path>")
	text, _ = secretscreen.Screen(text)
	return boundText(strings.TrimSpace(text), lossMessageBytes)
}

func boundText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// hookInfrastructureDetail is the detail of a control hook's infrastructure receipt: the qualify
// reason, then, when the run kept an error, its location and message on the next two lines. A
// one-line detail is the reason alone, as hooks wrote before CCE-V0-116. The reason is kept whole,
// since CCE-V0-087 reads it, and the location and message are cut so the detail stays within BBF's
// bound; they never make a detail that BBF would accept as the reason alone invalid.
func hookInfrastructureDetail(reason string, r jstestprovider.Receipt, runErr error, roots []string) string {
	location, message := runErrorDetail(r, runErr, roots)
	room := hookDetailBytes - len(reason) - 2
	if room < 0 {
		return reason
	}
	location = boundText(location, room)
	message = boundText(message, room-len(location))
	if location == "" && message == "" {
		return reason
	}
	return reason + "\n" + location + "\n" + message
}

// parseHookDetail splits a hook infrastructure detail into its reason, location and message.
func parseHookDetail(detail string) (reason, location, message string) {
	parts := strings.SplitN(detail, "\n", 3)
	reason = parts[0]
	if len(parts) > 1 {
		location = parts[1]
	}
	if len(parts) > 2 {
		message = parts[2]
	}
	return reason, location, message
}

// controlLosses are a control's attempts that read infrastructure_failed, in attempt order.
func controlLosses(r behaviorfalsify.ControlResult) []runLoss {
	var out []runLoss
	for _, a := range r.Attempts {
		if a.Status != behaviorfalsify.StatusInfrastructureFailed {
			continue
		}
		l := runLoss{Attempt: a.Attempt, AttemptReasons: append([]string{}, a.Reasons...)}
		switch {
		case a.Receipt == nil:
			p := a.HookProcess
			l.HookProcess = fmt.Sprintf("started=%t completed=%t exit=%d timed_out=%t cancelled=%t overflow=%t owned_cleanup=%t descendants_gone=%t",
				p.Started, p.Completed, p.Exit, p.TimedOut, p.Cancelled, p.Overflow, p.OwnedCleanup, p.DescendantsGone)
			if p.Error != "" {
				l.HookProcess += " error=" + lossText(p.Error, nil)
			}
		case a.Receipt.Infrastructure != nil:
			reason, location, message := parseHookDetail(a.Receipt.Infrastructure.Detail)
			l.Reason, l.ErrorLocation, l.ErrorMessage = lossText(reason, nil), lossText(location, nil), lossText(message, nil)
		}
		out = append(out, l)
	}
	return out
}
