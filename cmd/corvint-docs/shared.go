// SPDX-License-Identifier: Apache-2.0

package main

// The helpers below reproduce the corvint CLI's argument, root and stderr
// envelope conventions byte for byte (CMS-V0-004), so `corvint docs ...`
// forwarded to this binary prints what the in-process verbs printed. They are
// copies, not imports: the core's command package is not a library, and the
// documentation app must not reach into it.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/diagnostic"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/observations"
)

func argumentError(message string) error {
	return &gokernel.Error{Code: "invalid-arguments", Message: message}
}

// emitError writes the one-line stderr envelope every corvint verb writes:
// code, error and ok:false, with diagnostic members when the error carries a
// refusal (GPK-V0-071 maps Git's dubious-ownership exit first).
func emitError(stderr io.Writer, err error) {
	if refusal := dubiousOwnershipRefusal(err); refusal != nil {
		err = refusal
	}
	code := "internal-error"
	var kernelError *gokernel.Error
	if errors.As(err, &kernelError) {
		code = kernelError.Code
	}
	var contextError *contextindex.Error
	if errors.As(err, &contextError) && contextError.Code != "" {
		beforeOK, afterOK := diagnosticMembers(err)
		_, _ = fmt.Fprintf(
			stderr, "{\"code\": %s, \"error\": %s, %s\"ok\": false%s}\n",
			pythonJSONString(contextError.Code), pythonJSONString(err.Error()), beforeOK, afterOK,
		)
		return
	}
	if errors.As(err, &contextError) {
		if class := contextindex.GitStartErrorClass(err); class != "" {
			// DRC-V0-910: the start class is additive; the error bytes are unchanged.
			_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"git_start_error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()), pythonJSONString(class))
			return
		}
		_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()))
		return
	}
	beforeOK, afterOK := diagnosticMembers(err)
	_, _ = fmt.Fprintf(
		stderr, "{\"code\": %s, \"error\": %s, %s\"ok\": false%s}\n",
		pythonJSONString(code), pythonJSONString(err.Error()), beforeOK, afterOK,
	)
}

func diagnosticMembers(err error) (beforeOK, afterOK string) {
	var carried *diagnostic.Error
	if !errors.As(err, &carried) {
		return "", ""
	}
	refusal := carried.Refusal.Bounded()
	evidence := make([]string, 0, len(refusal.Evidence))
	for _, pair := range refusal.Evidence {
		evidence = append(evidence, fmt.Sprintf("{\"name\": %s, \"value\": %s}", pythonJSONString(pair.Name), pythonJSONString(pair.Value)))
	}
	fixes := make([]string, 0, len(refusal.SupportedFixes))
	for _, fix := range refusal.SupportedFixes {
		fixes = append(fixes, pythonJSONString(fix))
	}
	beforeOK = "\"evidence\": [" + strings.Join(evidence, ", ") + "], "
	afterOK = fmt.Sprintf(
		", \"subject\": {\"kind\": %s, \"value\": %s}, \"supported_fixes\": [%s]",
		pythonJSONString(refusal.Subject.Kind), pythonJSONString(refusal.Subject.Value), strings.Join(fixes, ", "),
	)
	if refusal.Terminal != "" {
		afterOK += ", \"terminal\": " + pythonJSONString(refusal.Terminal)
	}
	return beforeOK, afterOK
}

const dubiousOwnershipMarker = "detected dubious ownership in repository at '"

func dubiousOwnershipRefusal(err error) error {
	message := err.Error()
	start := strings.Index(message, dubiousOwnershipMarker)
	if start < 0 {
		return nil
	}
	repository, _, quoted := strings.Cut(message[start+len(dubiousOwnershipMarker):], "'")
	if !quoted || repository == "" {
		return nil
	}
	return &gokernel.Error{Code: "repository-ownership-refused", Message: "Git refuses the repository at " + pythonRepr(repository) + " because another user owns it (detected dubious ownership); Corvint runs Git without global or system configuration, so a safe.directory exception cannot take effect: run Corvint as the user who owns the repository, or give the current user ownership of it"}
}

func pythonJSONString(value string) string {
	var output strings.Builder
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			switch {
			case character >= 0x20 && character <= 0x7e:
				output.WriteRune(character)
			case character <= 0xffff:
				fmt.Fprintf(&output, `\u%04x`, character)
			default:
				value := character - 0x10000
				fmt.Fprintf(&output, `\u%04x\u%04x`, 0xd800+(value>>10), 0xdc00+(value&0x3ff))
			}
		}
	}
	output.WriteByte('"')
	return output.String()
}

func pythonRepr(value string) string {
	quote := '\''
	if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
		quote = '"'
	}
	var output strings.Builder
	output.WriteRune(quote)
	for _, character := range value {
		switch character {
		case '\\':
			output.WriteString(`\\`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character == quote {
				output.WriteByte('\\')
				output.WriteRune(character)
			} else if pythonPrintable(character) {
				output.WriteRune(character)
			} else if character <= 0xff {
				fmt.Fprintf(&output, `\x%02x`, character)
			} else if character <= 0xffff {
				fmt.Fprintf(&output, `\u%04x`, character)
			} else {
				fmt.Fprintf(&output, `\U%08x`, character)
			}
		}
	}
	output.WriteRune(quote)
	return output.String()
}

func pythonPrintable(character rune) bool {
	if character >= 0x20 && character <= 0x7e {
		return true
	}
	return unicode.IsLetter(character) || unicode.IsMark(character) || unicode.IsNumber(character) ||
		unicode.IsPunct(character) || unicode.IsSymbol(character)
}

func normalizeRoot(value string) (string, error) {
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", argumentError("cannot expand --root")
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	root, err := filepath.Abs(value)
	if err != nil {
		return "", argumentError("invalid --root")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	return root, nil
}

func resolveExplicitRoot(value string) (string, error) {
	root, err := normalizeRoot(value)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return "", notRepositoryRootRefusal(value, root)
	}
	return root, nil
}

func notRepositoryRootRefusal(operand, root string) error {
	topLevel := enclosingRepositoryRoot(root)
	if topLevel == "" {
		return refused("invalid-arguments", "not a Git repository: "+root, diagnostic.Refusal{
			Subject:        diagnostic.Subject{Kind: "value", Value: operand},
			Evidence:       []diagnostic.Evidence{{Name: "root", Value: root}},
			SupportedFixes: []string{"cli.use-git-repository-root"},
		})
	}
	return refused("invalid-arguments", "not the repository root; top level is "+topLevel, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: operand},
		Evidence:       []diagnostic.Evidence{{Name: "root", Value: root}, {Name: "top_level", Value: topLevel}},
		SupportedFixes: []string{"cli.use-git-repository-root"},
	})
}

func refused(code, message string, refusal diagnostic.Refusal) error {
	return &diagnostic.Error{Err: &gokernel.Error{Code: code, Message: message}, Refusal: refusal}
}

func enclosingRepositoryRoot(root string) string {
	for parent := filepath.Dir(root); parent != root; root, parent = parent, filepath.Dir(parent) {
		if _, err := os.Stat(filepath.Join(parent, ".git")); err == nil {
			return parent
		}
	}
	return ""
}

// commandPositionAfterRoots is the index of the first argument after the
// `--root PATH` / `--root=PATH` preamble, or -1 when nothing follows it.
func commandPositionAfterRoots(arguments []string) int {
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" && rootPreambleValue(arguments, index+1) || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			index += 2
		} else {
			index++
		}
	}
	if index >= len(arguments) {
		return -1
	}
	return index
}

func rootPreambleValue(arguments []string, index int) bool {
	return index < len(arguments) && (arguments[index] == "--" || !argparseOptionLike(arguments[index]))
}

var argparseNegativeNumber = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

func argparseOptionLike(token string) bool {
	return len(token) > 1 && token[0] == '-' && !argparseNegativeNumber.MatchString(token) && !strings.Contains(token, " ")
}

type readResult struct {
	value []byte
	err   error
}

// readInputBounded reads at most limit+1 bytes of stdin, so a caller can tell
// an over-limit input from a full one, and stops at ctx's end.
func readInputBounded(ctx context.Context, stdin io.Reader, limit int64) ([]byte, error) {
	result := make(chan readResult, 1)
	go func() {
		value, err := io.ReadAll(io.LimitReader(stdin, limit+1))
		result <- readResult{value: value, err: err}
	}()
	select {
	case completed := <-result:
		return completed.value, completed.err
	case <-ctx.Done():
		if closer, ok := stdin.(io.Closer); ok {
			_ = closer.Close()
		}
		return nil, ctx.Err()
	}
}

// observeUnsupported appends an unsupported-* failure to the bounded
// self-observation ledger (SOL-V0-007), the one write `docs draft` and
// `docs consume` make.
func observeUnsupported(root string, err error, task string) {
	code := ""
	var kernelError *gokernel.Error
	if errors.As(err, &kernelError) {
		code = kernelError.Code
	}
	var contextError *contextindex.Error
	if errors.As(err, &contextError) {
		code = contextError.Code
	}
	if root == "" || !strings.HasPrefix(code, "unsupported-") {
		return
	}
	intent := "unknown"
	if task != "" {
		intent = contextindex.QueryIntent(task)
	}
	event := observations.Event{Kind: "unsupported", Code: code, QueryIntent: intent}
	if task != "" {
		event.TaskSHA256 = observations.TaskHash(task)
	}
	_ = observations.Append(root, event)
}
