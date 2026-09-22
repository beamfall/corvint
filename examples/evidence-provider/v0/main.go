// SPDX-License-Identifier: AGPL-3.0-or-later
// Copy this single file to author a local provider; it uses only Go's standard library.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

const providerID = "kit-example"
const providerRevision = "0.1.0"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("kit-example", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	schema := flags.String("profile", "", "exact external-evidence-provider/0, /1 or /2")
	version := flags.String("provider-version", "", "exact provider version 0.1.0")
	revision := flags.String("revision", "", "full repository commit id")
	origin := flags.String("origin", "", "full repository root commit id (/1 and /2)")
	source := flags.String("path", "", "repository-relative tracked path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *version != providerRevision {
		return fmt.Errorf("unsupported provider version: require %s", providerRevision)
	}
	if *schema != "external-evidence-provider/0" && *schema != "external-evidence-provider/1" && *schema != "external-evidence-provider/2" {
		return errors.New("unsupported profile")
	}
	oid := regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	if !oid.MatchString(*revision) {
		return errors.New("revision must be a full commit id")
	}
	if !validPath(*source) {
		return errors.New("path must be a clean repository-relative path")
	}
	record := map[string]any{
		"schema":   *schema,
		"provider": map[string]string{"id": providerID, "revision": providerRevision},
		"entities": []any{map[string]string{"id": "example", "kind": "capability", "summary": "Example capability declared by the local provider"}},
	}
	relation := map[string]any{"type": "implements", "evidence": "declared", "rule": "example-declaration", "reference": "local author declaration"}
	if *schema == "external-evidence-provider/0" {
		if *origin != "" {
			return errors.New("profile /0 has no repository origin field")
		}
		record["repository"] = map[string]string{"revision": *revision}
		relation["from"], relation["to"] = "path:"+*source, providerID+":example"
	} else {
		if !oid.MatchString(*origin) {
			return errors.New("origin must be a full root commit id")
		}
		record["repositories"] = []any{map[string]string{"id": "app", "origin": *origin, "revision": *revision}}
		relation["from"] = map[string]string{"repository": "app", "path": *source}
		relation["to"] = map[string]string{"provider": providerID, "entity": "example"}
	}
	record["relations"] = []any{relation}
	return json.NewEncoder(out).Encode(record)
}

func validPath(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 1024 {
		return false
	}
	if !utf8.ValidString(value) {
		return false
	}
	if path.IsAbs(value) || path.Clean(value) != value || strings.HasPrefix(value, "../") {
		return false
	}
	return !strings.ContainsAny(value, "\\\x00\r\n\t:")
}
