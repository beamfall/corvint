// SPDX-License-Identifier: Apache-2.0

// Package nativelinecov collects experimental per-test native coverage for JVM,
// .NET, C/C++, Rust and Swift, using the unchanged linecov record format.
package nativelinecov

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov/v8"
)

const MaxReportBytes = 4 << 20

// Report is an existing native export, not a new on-disk record format.
// Format is jacoco, cobertura or lcov. Sources maps native source names to
// tracked repository-relative paths. Every reported source needs a mapping.
type Report struct {
	Format string
	Data   []byte
}

func validPath(p string) bool {
	return p != "" && p != "." && path.Clean(p) == p && !strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\\x00\r\n:")
}

// Decode retains both executed and instrumented-but-unexecuted lines
// (LPCV-V0-8206). A malformed export returns no partial coverage.
func Decode(r Report, sources map[string]string) (v8.Lines, error) {
	if len(r.Data) == 0 || len(r.Data) > MaxReportBytes {
		return nil, fmt.Errorf("native coverage report size")
	}
	out := v8.Lines{}
	add := func(file string, n, hits int64) error {
		rel, ok := sources[file]
		if !ok || !validPath(rel) {
			return fmt.Errorf("unmapped native source %q", file)
		}
		if n < 1 || n > 10000000 || hits < 0 {
			return fmt.Errorf("invalid native line/count")
		}
		out.Add(rel, int(n), hits > 0)
		return nil
	}
	var err error
	switch r.Format {
	case "lcov":
		err = lcov(r.Data, add)
	case "jacoco", "cobertura":
		err = xmlLines(r, add)
	default:
		err = fmt.Errorf("unsupported native coverage format %q", r.Format)
	}
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("native coverage has no mapped lines")
	}
	return out, nil
}

func number(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid native count %q", s)
	}
	return n, nil
}

func lcov(data []byte, add func(string, int64, int64) error) error {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 4096), MaxReportBytes)
	file := ""
	lines := 0
	for sc.Scan() {
		s := sc.Text()
		switch {
		case strings.HasPrefix(s, "SF:"):
			if file != "" {
				return fmt.Errorf("unterminated LCOV record")
			}
			file = strings.TrimPrefix(s, "SF:")
			lines = 0
			if file == "" {
				return fmt.Errorf("empty LCOV source")
			}
		case strings.HasPrefix(s, "DA:"):
			if file == "" {
				return fmt.Errorf("LCOV line outside source")
			}
			fields := strings.Split(strings.TrimPrefix(s, "DA:"), ",")
			if len(fields) < 2 || len(fields) > 3 {
				return fmt.Errorf("invalid LCOV DA")
			}
			n, e := number(fields[0])
			if e != nil {
				return e
			}
			hits, e := number(fields[1])
			if e != nil {
				return e
			}
			if e = add(file, n, hits); e != nil {
				return e
			}
			lines++
		case s == "end_of_record":
			if file == "" || lines == 0 {
				return fmt.Errorf("empty LCOV record")
			}
			file = ""
		case s == "", strings.HasPrefix(s, "TN:"), strings.HasPrefix(s, "FN:"), strings.HasPrefix(s, "FNDA:"), strings.HasPrefix(s, "FNF:"), strings.HasPrefix(s, "FNH:"), strings.HasPrefix(s, "BRDA:"), strings.HasPrefix(s, "BRF:"), strings.HasPrefix(s, "BRH:"), strings.HasPrefix(s, "LF:"), strings.HasPrefix(s, "LH:"):
		default:
			return fmt.Errorf("unknown LCOV entry")
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if file != "" {
		return fmt.Errorf("truncated LCOV record")
	}
	return nil
}

func xmlLines(r Report, add func(string, int64, int64) error) error {
	d := xml.NewDecoder(bytes.NewReader(r.Data))
	var stack []string
	pkg, file := "", ""
	root := ""
	done := false
	nodes := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			nodes++
			if nodes > 100000 || len(stack) > 32 || t.Name.Space != "" || done {
				return fmt.Errorf("native XML bounds/root")
			}
			attrs := map[string]string{}
			for _, a := range t.Attr {
				if a.Name.Space != "" {
					return fmt.Errorf("native XML namespace")
				}
				if _, ok := attrs[a.Name.Local]; ok {
					return fmt.Errorf("duplicate XML attribute")
				}
				attrs[a.Name.Local] = a.Value
			}
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			} else {
				root = t.Name.Local
				if r.Format == "jacoco" && root != "report" || r.Format == "cobertura" && root != "coverage" {
					return fmt.Errorf("native XML root")
				}
			}
			stack = append(stack, t.Name.Local)
			if r.Format == "jacoco" {
				switch t.Name.Local {
				case "package":
					pkg = attrs["name"]
				case "sourcefile":
					if parent != "package" {
						return fmt.Errorf("JaCoCo source hierarchy")
					}
					file = attrs["name"]
					if file == "" {
						return fmt.Errorf("missing JaCoCo source name")
					}
					if pkg != "" {
						file = pkg + "/" + file
					}
				case "line":
					if parent != "sourcefile" || file == "" {
						return fmt.Errorf("JaCoCo line hierarchy")
					}
					n, e := number(attrs["nr"])
					if e != nil {
						return e
					}
					hits, e := number(attrs["ci"])
					if e != nil {
						return e
					}
					for _, k := range []string{"mi", "mb", "cb"} {
						if _, e = number(attrs[k]); e != nil {
							return e
						}
					}
					if e = add(file, n, hits); e != nil {
						return e
					}
				}
			} else {
				switch t.Name.Local {
				case "class":
					file = attrs["filename"]
					if file == "" {
						return fmt.Errorf("missing Cobertura source name")
					}
				case "line":
					if parent != "lines" || file == "" {
						return fmt.Errorf("Cobertura line hierarchy")
					}
					n, e := number(attrs["number"])
					if e != nil {
						return e
					}
					hits, e := number(attrs["hits"])
					if e != nil {
						return e
					}
					if e = add(file, n, hits); e != nil {
						return e
					}
				}
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return fmt.Errorf("native XML unbalanced")
			}
			stack = stack[:len(stack)-1]
			if t.Name.Local == "sourcefile" || t.Name.Local == "class" {
				file = ""
			}
			if t.Name.Local == "package" {
				pkg = ""
			}
			if len(stack) == 0 {
				done = true
			}
		case xml.Directive:
			// JaCoCo/Cobertura commonly declare a DTD. encoding/xml never fetches it.
			s := string(t)
			if len(stack) != 0 || root != "" || !strings.HasPrefix(s, "DOCTYPE ") || strings.Contains(s, "[") {
				return fmt.Errorf("unsupported native XML directive")
			}
		case xml.CharData:
			if len(stack) == 0 && len(bytes.TrimSpace(t)) > 0 {
				return fmt.Errorf("trailing native XML")
			}
		case xml.ProcInst:
			if t.Target != "xml" || root != "" {
				return fmt.Errorf("unsupported native XML instruction")
			}
		}
	}
	if !done || len(stack) != 0 {
		return fmt.Errorf("truncated native XML")
	}
	return nil
}
