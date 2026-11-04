package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ticketShowCommand is `ticket show`. With --format text it renders the one
// ticket's title, fields, body and acceptance criteria as plain text, so a
// lane ticket packet needs no JSON parsing (TCL-V0-741). A refusal stays the
// JSON envelope; --format json is the default form.
func ticketShowCommand(env Env, args []string) int {
	cmd := []string{"ticket", "show"}
	format, rest := "json", make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--root" || strings.HasPrefix(a, "--root="):
			// TCL-V0-926: share the global flag's validation and relative
			// path resolution, before either JSON or text reads the store.
			rootEnv := env
			rootEnv.Args = args[i:]
			if res := globalRoot(&rootEnv); res != nil {
				res.Command = cmd
				return emit(env.Stdout, res)
			}
			i += len(args[i:]) - len(rootEnv.Args) - 1
			env.Cwd, env.rootOriginalCwd, env.rootSelected = rootEnv.Cwd, rootEnv.rootOriginalCwd, rootEnv.rootSelected
		case a == "--format":
			if i+1 >= len(args) {
				return emit(env.Stdout, usage(cmd, "--format needs json or text"))
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		default:
			rest = append(rest, a)
			continue
		}
		if format != "json" && format != "text" {
			return emit(env.Stdout, usage(cmd, "--format must be json or text"))
		}
	}
	if format == "text" {
		for _, a := range rest {
			if a == "--summary" || a == "--fields" || strings.HasPrefix(a, "--fields=") {
				return emit(env.Stdout, usage(cmd, "--format text renders the whole ticket; drop "+strings.SplitN(a, "=", 2)[0]))
			}
		}
	}
	res := compactRead(cmd, rest, compactSpec{summary: ticketSummary}, func(a []string) *wire.Result { return ticketShow(env, a, true) })
	if format != "text" || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		return emit(env.Stdout, res)
	}
	if err := writeTicketText(env.Stdout, res.Items[0]); err != nil {
		return 1
	}
	return 0
}

// writeTicketText writes one `ticket show` item as plain text: a header
// line, one line per scalar field, then the body and the numbered
// acceptance criteria verbatim. Control characters other than newline and
// tab are dropped, so the text cannot carry terminal escapes.
func writeTicketText(w interface{ Write([]byte) (int, error) }, item wire.Value) error {
	bw := bufio.NewWriter(w)
	rec, _ := objGet(item, "record")
	str := func(v wire.Value, key string) string {
		f, ok := objGet(v, key)
		if !ok || f.Kind != wire.KindString {
			return ""
		}
		return plainText(f.Str)
	}
	list := func(v wire.Value, key string) []string {
		f, ok := objGet(v, key)
		if !ok {
			return nil
		}
		var out []string
		for _, e := range f.Arr {
			if e.Kind == wire.KindString {
				out = append(out, plainText(e.Str))
			}
		}
		return out
	}
	join := func(xs []string) string {
		if len(xs) == 0 {
			return "(none)"
		}
		return strings.Join(xs, ", ")
	}
	fmt.Fprintf(bw, "# corvint-tasks ticket text (untrusted ticket content)\n")
	fmt.Fprintf(bw, "%s  %s\n", str(rec, "ticketId"), strings.ReplaceAll(str(rec, "title"), "\n", " "))
	fmt.Fprintf(bw, "status: %s  priority: %s  kind: %s  revision: %s\n", str(rec, "status"), str(rec, "priority"), str(rec, "kind"), str(item, "revision"))
	if m := str(rec, "milestone"); m != "" {
		fmt.Fprintf(bw, "milestone: %s\n", m)
	}
	fmt.Fprintf(bw, "labels: %s\n", join(list(rec, "labels")))
	effects, _ := objGet(rec, "effects")
	fmt.Fprintf(bw, "touchPaths: %s\n", join(list(effects, "touchPaths")))
	fmt.Fprintf(bw, "coverage: %s\n", str(effects, "coverage"))
	if refs := list(rec, "requirementRefs"); len(refs) > 0 {
		fmt.Fprintf(bw, "requirementRefs: %s\n", join(refs))
	}
	fmt.Fprintf(bw, "\nBody:\n")
	if body := str(rec, "body"); body != "" {
		fmt.Fprintf(bw, "%s\n", strings.TrimRight(body, "\n"))
	} else {
		fmt.Fprintf(bw, "(none)\n")
	}
	fmt.Fprintf(bw, "\nAcceptance criteria:\n")
	acs := list(rec, "acceptanceCriteria")
	if len(acs) == 0 {
		fmt.Fprintf(bw, "(none)\n")
	}
	for i, ac := range acs {
		fmt.Fprintf(bw, "%d. %s\n", i+1, strings.TrimRight(ac, "\n"))
	}
	return bw.Flush()
}

func objGet(v wire.Value, key string) (wire.Value, bool) {
	if v.Obj == nil {
		return wire.Value{}, false
	}
	return v.Obj.Get(key)
}

// plainText drops control characters but newline and tab.
func plainText(s string) string {
	return strings.Map(func(r rune) rune {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, s)
}
