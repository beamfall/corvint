// Package sql observes two bounded experimental native SQL runner profiles.
// Database authority, dependency closure and semantic adequacy are not inferred.
package sql

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func Runners() []string { return []string{"sql-pgtap", "sql-sqllogictest-sqlite"} }

var digest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var dbName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// Connection is an explicit operator declaration. It is neither a database
// identity proof nor authority to execute SQL against a database.
type Connection struct {
	SocketDirectory string `json:"socketDirectory"`
	Port            int    `json:"port"`
	Database        string `json:"database"`
	User            string `json:"user"`
}

func literalFile(name string) bool {
	return name != "" && len(name) <= 1024 && filepath.IsLocal(name) && filepath.ToSlash(filepath.Clean(name)) == name && !strings.HasPrefix(name, "-") && !strings.ContainsAny(name, "\x00\r\n\\") && name != "."
}

func readConnection(r tr.Request) (Connection, error) {
	var c Connection
	if r.Config == "" || !digest.MatchString(r.ConfigSha256) {
		return c, fmt.Errorf("pgTAP requires a pinned connection JSON file")
	}
	name := r.Config
	if !filepath.IsAbs(name) {
		if !literalFile(name) {
			return c, fmt.Errorf("invalid connection file path")
		}
		name = filepath.Join(r.Root, filepath.FromSlash(name))
	}
	if filepath.Clean(name) != name {
		return c, fmt.Errorf("noncanonical connection file path")
	}
	root, e := os.OpenRoot(string(filepath.Separator))
	if e != nil {
		return c, e
	}
	defer root.Close()
	b, e := testvaliditydoc.ReadFileBounded(root, strings.TrimPrefix(name, string(filepath.Separator)), 16<<10)
	if e != nil {
		return c, e
	}
	if tr.Digest(b) != r.ConfigSha256 {
		return c, fmt.Errorf("connection JSON digest mismatch")
	}
	if e = tr.DecodeDocument(b, &c); e != nil {
		return c, e
	}
	if !filepath.IsAbs(c.SocketDirectory) || filepath.Clean(c.SocketDirectory) != c.SocketDirectory || len(c.SocketDirectory) > 1024 || strings.ContainsAny(c.SocketDirectory, ",\x00\r\n") || c.Port < 1 || c.Port > 65535 || !dbName.MatchString(c.Database) || !dbName.MatchString(c.User) {
		return c, fmt.Errorf("connection requires one Unix socket directory, port, literal database and user")
	}
	return c, nil
}

func Build(r tr.Request) (tr.Invocation, error) {
	v := tr.Invocation{Format: r.Runner, Environment: map[string]string{}}
	if !filepath.IsAbs(r.Root) || !filepath.IsAbs(r.ReportDir) || filepath.Clean(r.ReportDir) != r.ReportDir || !filepath.IsAbs(r.Executable) || !digest.MatchString(r.ExecutableSha256) || !literalFile(r.Project) || !digest.MatchString(r.InputFiles[r.Project]) {
		return v, fmt.Errorf("SQL profile requires explicit root, fresh report path, pinned executable and one pinned root-relative script")
	}
	if len(r.Selectors) > 1 || (len(r.Selectors) == 1 && r.Selectors[0] != r.Project) {
		return v, fmt.Errorf("SQL selection is one whole script only")
	}
	if r.Target != "" || r.Reporter != "" || r.ReporterSha256 != "" || len(r.Tools) != 0 || len(r.ReportFiles) != 0 {
		return v, fmt.Errorf("SQL profile does not admit auxiliary commands or report inputs")
	}
	switch r.Runner {
	case "sql-pgtap":
		if !strings.HasSuffix(r.Project, ".sql") {
			return v, fmt.Errorf("pgTAP requires a .sql script")
		}
		c, e := readConnection(r)
		if e != nil {
			return v, e
		}
		v.Argv = []string{"-X", "-A", "-t", "-q", "-w", "-h", c.SocketDirectory, "-p", strconv.Itoa(c.Port), "-U", c.User, "-d", c.Database, "-v", "ON_ERROR_STOP=1", "-f", r.Project}
		v.Environment["PGCONNECT_TIMEOUT"] = "5"
		v.Environment["LC_ALL"] = "C"
		// psql reports SQL execution status, not whether the TAP assertions passed.
		v.OutcomeNeutralExitCodes = []int{0}
	case "sql-sqllogictest-sqlite":
		if !strings.HasSuffix(r.Project, ".slt") || r.Config != "" || r.ConfigSha256 != "" {
			return v, fmt.Errorf("SQLite sqllogictest requires a .slt script and no external connection configuration")
		}
		v.Argv = []string{"-verify", "-engine", "SQLite", "-connection", filepath.Join(r.ReportDir, "scratch.sqlite"), r.Project}
		v.SuccessExitCodes = []int{0}
		for n := 1; n <= 255; n++ {
			v.FailureExitCodes = append(v.FailureExitCodes, n)
		}
	default:
		return v, fmt.Errorf("unsupported SQL runner")
	}
	return v, nil
}

func Parse(in tr.Input) (tr.Observation, error) {
	o := tr.Observation{Runner: in.Runner, RetryInformation: tr.NotReported}
	if in.Runner != "sql-pgtap" && in.Runner != "sql-sqllogictest-sqlite" {
		return o, fmt.Errorf("unsupported SQL runner")
	}
	if !literalFile(in.SourceFile) {
		return o, fmt.Errorf("SQL observation requires caller-bound root-relative sourceFile")
	}
	if len(in.Reports) != 0 || len(in.Stdout) > tr.MaxReportBytes || len(in.Stderr) > tr.MaxReportBytes || !utf8.Valid(in.Stdout) || !utf8.Valid(in.Stderr) || strings.ContainsRune(string(in.Stdout), 0) || strings.ContainsRune(string(in.Stderr), 0) {
		return o, fmt.Errorf("SQL native stream bound/encoding/report inventory")
	}
	if in.TimedOut || in.Interrupted || in.Overflow {
		problem(&o, "PROCESS_INCOMPLETE", "SQL process timed out, was interrupted or exceeded output bound")
		return o, nil
	}
	var err error
	if in.Runner == "sql-pgtap" {
		err = parseTAP(in, &o)
	} else {
		err = parseSQLite(in, &o)
	}
	if err != nil {
		return o, err
	}
	o.Complete = len(o.Problems) == 0 && len(o.Tests) > 0
	return o, nil
}
func problem(o *tr.Observation, code, detail string) {
	o.Problems = append(o.Problems, tr.Problem{Code: code, Detail: detail})
}
