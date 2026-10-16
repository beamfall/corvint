package appflows

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
)

// The Playwright adapters apply the AFU-V1-037 record, attempt, link and traversal bounds in one
// streaming token pass before any document is decoded, so an over-bound report is never decoded.
// The pass counts what the decode keeps, matching member names case-insensitively as encoding/json
// does, and names the bound the decode would hit first. A counted member that repeats in one object is
// merged into the earlier value by the decode, which the pass does not model: it then abstains and
// the post-decode checks apply.

var errRunJSON = errors.New("run report is not valid JSON")

// runTally counts a run-report fragment in decode order: the tests it yields and the index of the
// first test that breaks its own attempt or link bound.
type runTally struct {
	tests int
	at    int
	bound string
}

// then appends next in decode order.
func (t runTally) then(next runTally) runTally {
	if t.bound == "" && next.bound != "" {
		t.at, t.bound = t.tests+next.at, next.bound
	}
	t.tests += next.tests
	return t
}

// test appends one test whose own attempt or link bound is bound ("" for none).
func (t runTally) test(bound string) runTally { return t.then(runTally{tests: 1, bound: bound}) }

// check names the first bound the decode would hit. appendFirst is true when the adapter appends a test
// before keeping its attempts (Playwright JSON) and false when it keeps them first (the receipt).
func (t runTally) check(appendFirst bool) error {
	if t.bound != "" && (t.at < maxRunRecords || !appendFirst && t.at == maxRunRecords) {
		return boundError(t.bound)
	}
	if t.tests > maxRunRecords {
		return boundError(BoundRecords)
	}
	return nil
}

// runScan reads a JSON run report token by token, refusing nesting deeper than maxRunDepth as it
// descends.
type runScan struct {
	d        *json.Decoder
	depth    int
	repeated bool
}

func (s *runScan) token() (json.Token, error) {
	tok, err := s.d.Token()
	if err == io.EOF && s.depth == 0 {
		return nil, io.EOF
	}
	if err != nil {
		return nil, errRunJSON
	}
	switch tok {
	case json.Delim('{'), json.Delim('['):
		if s.depth++; s.depth > maxRunDepth {
			return nil, boundError(BoundTraversal)
		}
	case json.Delim('}'), json.Delim(']'):
		s.depth--
	}
	return tok, nil
}

// skip consumes the rest of the value that tok opens.
func (s *runScan) skip(tok json.Token) error {
	if tok != json.Delim('{') && tok != json.Delim('[') {
		return nil
	}
	for floor := s.depth - 1; s.depth > floor; {
		if _, err := s.token(); err != nil {
			return err
		}
	}
	return nil
}

// object hands each member of the object tok opens whose name case-folds to names[i] to member with i
// and its first value token, skipping every other member, and marks the scan repeated when one of
// names occurs twice; any other value is skipped.
func (s *runScan) object(tok json.Token, names []string, member func(i int, tok json.Token) error) error {
	if tok != json.Delim('{') {
		return s.skip(tok)
	}
	seen := make([]bool, len(names))
	for {
		key, err := s.token()
		if err != nil || key == json.Delim('}') {
			return err
		}
		value, err := s.token()
		if err != nil {
			return err
		}
		i := slices.IndexFunc(names, func(name string) bool { return strings.EqualFold(key.(string), name) })
		if i < 0 {
			err = s.skip(value)
		} else {
			s.repeated = s.repeated || seen[i]
			seen[i] = true
			err = member(i, value)
		}
		if err != nil {
			return err
		}
	}
}

// array hands the first token of each element of the array tok opens to elem; any other value is
// skipped.
func (s *runScan) array(tok json.Token, elem func(tok json.Token) error) error {
	if tok != json.Delim('[') {
		return s.skip(tok)
	}
	for {
		next, err := s.token()
		if err != nil || next == json.Delim(']') {
			return err
		}
		if err = elem(next); err != nil {
			return err
		}
	}
}

// member reads the single member name of the object tok opens with read, skipping the rest.
func (s *runScan) member(tok json.Token, name string, read func(tok json.Token) error) error {
	return s.object(tok, []string{name}, func(_ int, value json.Token) error { return read(value) })
}

// attempts reads one test's attempt array and returns the first attempt or link bound it breaks, in
// the order the adapters keep attempts.
func (s *runScan) attempts(tok json.Token, links func(tok json.Token) (int, error)) (string, error) {
	bound, n := "", 0
	err := s.array(tok, func(tok json.Token) error {
		count, err := links(tok)
		switch {
		case bound != "":
		case n >= maxRunAttempts:
			bound = BoundAttempts
		case count > maxRunLinks:
			bound = BoundLinks
		}
		n++
		return err
	})
	return bound, err
}

func (s *runScan) count(tok json.Token) (int, error) {
	n := 0
	err := s.array(tok, func(tok json.Token) error {
		n++
		return s.skip(tok)
	})
	return n, err
}

// scanRun runs root over the first JSON value and scans any trailing values for the depth bound
// before it checks the tally; an empty report has nothing to bound.
func scanRun(raw []byte, appendFirst bool, root func(s *runScan, tok json.Token) (runTally, error)) error {
	s := &runScan{d: json.NewDecoder(bytes.NewReader(raw))}
	var tally runTally
	for first := true; ; first = false {
		tok, err := s.token()
		switch {
		case err == io.EOF && s.repeated:
			return nil
		case err == io.EOF:
			return tally.check(appendFirst)
		case err != nil:
		case first:
			tally, err = root(s, tok)
		default:
			err = s.skip(tok)
		}
		if err != nil {
			return err
		}
	}
}

// playwrightRunBounded applies the record, attempt, link and traversal bounds to a Playwright JSON
// report before it is decoded (AFU-V1-037).
func playwrightRunBounded(raw []byte) error {
	return scanRun(raw, true, func(s *runScan, tok json.Token) (tally runTally, err error) {
		err = s.member(tok, "suites", func(tok json.Token) (err error) {
			tally, err = s.playwrightSuites(tok)
			return err
		})
		return tally, err
	})
}

// playwrightSuites tallies suites as collectSpecs orders them: each suite's specs, then its suites.
func (s *runScan) playwrightSuites(tok json.Token) (runTally, error) {
	var all runTally
	err := s.array(tok, func(tok json.Token) error {
		var specs, nested runTally
		err := s.object(tok, []string{"specs", "suites"}, func(i int, tok json.Token) (err error) {
			if i == 0 {
				specs, err = s.playwrightSpecs(tok)
			} else {
				nested, err = s.playwrightSuites(tok)
			}
			return err
		})
		all = all.then(specs).then(nested)
		return err
	})
	return all, err
}

func (s *runScan) playwrightSpecs(tok json.Token) (runTally, error) {
	var all runTally
	err := s.array(tok, func(tok json.Token) error {
		var tests runTally
		err := s.member(tok, "tests", func(tok json.Token) error {
			return s.array(tok, func(tok json.Token) error {
				bound := ""
				err := s.member(tok, "results", func(tok json.Token) (err error) {
					bound, err = s.attempts(tok, s.playwrightLinks)
					return err
				})
				tests = tests.test(bound)
				return err
			})
		})
		all = all.then(tests)
		return err
	})
	return all, err
}

// playwrightLinks counts one result's links as playwrightAttempt keeps them: its attachments and the
// located entries of errors, or of error when errors is empty.
func (s *runScan) playwrightLinks(tok json.Token) (int, error) {
	attachments, errs, located, fallback := 0, 0, 0, 0
	err := s.object(tok, []string{"attachments", "errors", "error"}, func(i int, tok json.Token) (err error) {
		switch i {
		case 0:
			attachments, err = s.count(tok)
		case 1:
			err = s.array(tok, func(tok json.Token) error {
				errs++
				ok, err := s.playwrightLocated(tok)
				if ok {
					located++
				}
				return err
			})
		default:
			var ok bool
			if ok, err = s.playwrightLocated(tok); ok {
				fallback = 1
			}
		}
		return err
	})
	if errs == 0 {
		located = fallback
	}
	return attachments + located, err
}

// playwrightLocated reports whether the error tok opens carries a location object.
func (s *runScan) playwrightLocated(tok json.Token) (bool, error) {
	located := false
	err := s.member(tok, "location", func(tok json.Token) error {
		located = tok == json.Delim('{')
		return s.skip(tok)
	})
	return located, err
}

// playwrightReceiptBounded applies the record, attempt, link and traversal bounds to a
// playwright-attempts/3 receipt before it is decoded (AFU-V1-037).
func playwrightReceiptBounded(raw []byte) error {
	return scanRun(raw, false, func(s *runScan, tok json.Token) (tally runTally, err error) {
		err = s.member(tok, "receipt", func(tok json.Token) error {
			return s.member(tok, "tests", func(tok json.Token) error {
				return s.array(tok, func(tok json.Token) error {
					bound := ""
					err := s.member(tok, "attemptDetails", func(tok json.Token) (err error) {
						bound, err = s.attempts(tok, s.receiptLinks)
						return err
					})
					tally = tally.test(bound)
					return err
				})
			})
		})
		return tally, err
	})
}

// receiptLinks counts one attempt detail's anchor and artifacts.
func (s *runScan) receiptLinks(tok json.Token) (int, error) {
	anchor, artifacts := 0, 0
	err := s.object(tok, []string{"anchor", "artifacts"}, func(i int, tok json.Token) (err error) {
		if i == 1 {
			artifacts, err = s.count(tok)
			return err
		}
		if tok == json.Delim('{') {
			anchor = 1
		}
		return s.skip(tok)
	})
	return anchor + artifacts, err
}
