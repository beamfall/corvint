package testsource

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// jsBrief renders each example as its read, expectation count and "RULE severity location cause"
// findings, keyed by declaring line.
func jsBrief(t *testing.T, name, text string) map[int][]string {
	t.Helper()
	examples, reason := JSExampleSources(name, text)
	if reason != "" {
		t.Fatalf("%s: uncertain %q", name, reason)
	}
	out := map[int][]string{}
	for line, e := range examples {
		rows := []string{fmt.Sprintf("%s expectations=%d", e.Read, e.Expectations)}
		for _, f := range e.Findings {
			rows = append(rows, fmt.Sprintf("%s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause))
		}
		out[line] = rows
	}
	return out
}

func wantJS(t *testing.T, got, want map[int][]string) {
	t.Helper()
	for line, rows := range want {
		if !reflect.DeepEqual(got[line], rows) {
			t.Errorf("line %d\n got %q\nwant %q", line, got[line], rows)
		}
	}
	for line := range got {
		if _, ok := want[line]; !ok {
			t.Errorf("unexpected example at line %d: %q", line, got[line])
		}
	}
}

const located = testvalidity.ExampleSourceLocated

const shapesSpec = `import { describe, it, expect, test, beforeEach } from 'vitest';
import { sum } from '../src/sum';

describe('sum', () => {
  beforeEach(() => { sum(0, 0); });
  it('adds', () => {
    expect(sum(1, 2)).toBe(3);
  });
  test('adds again', async () => {
    const v = sum(2, 2);
    expect(v).toEqual(4);
    expect(v).not.toBe(5);
  });
  test.concurrent('concurrent', function () { expect(sum(0, 1)).toBe(1); });
  test.step('a step is not a declaration', async () => {});
  test.describe.serial('serial group', () => {
    test('nested', () => { expect(sum(3, 3)).toBe(6); });
  });
});
`

// TestJSExampleSourcesShapes: one source per example keyed by its callee line, with the
// expectation count; hooks, test.step and describes are not examples, and a repeated reading is
// identical (TSA-V0-073, TSA-V0-083).
func TestJSExampleSourcesShapes(t *testing.T) {
	got := jsBrief(t, "test/sum.test.ts", shapesSpec)
	wantJS(t, got, map[int][]string{
		6:  {located + " expectations=1"},
		9:  {located + " expectations=2"},
		14: {located + " expectations=1"},
		17: {located + " expectations=1"},
	})
	examples, _ := JSExampleSources("test/sum.test.ts", shapesSpec)
	if anchors := examples[9].ExpectationAnchors; !reflect.DeepEqual(anchors, []string{"test/sum.test.ts:11", "test/sum.test.ts:12"}) {
		t.Errorf("anchors %q", anchors)
	}
	again, _ := JSExampleSources("test/sum.test.ts", shapesSpec)
	if !reflect.DeepEqual(again, examples) {
		t.Errorf("a repeated reading differs")
	}
}

// TestJSExampleSourcesAbstain: a non-test file and an unbalanced file abstain as a whole;
// unsupported members, a non-literal body and two examples on one line abstain per example
// (TSA-V0-073).
func TestJSExampleSourcesAbstain(t *testing.T) {
	if examples, reason := JSExampleSources("src/sum.ts", shapesSpec); reason != testvalidity.ExampleSourceNotJSTestFile || len(examples) != 0 {
		t.Errorf("non-test file %q %v", reason, examples)
	}
	if examples, reason := JSExampleSources("test/broken.test.ts", "describe('x', () => {\n  it('y', () => {\n"); reason == "" || len(examples) != 0 {
		t.Errorf("unbalanced file %q %v", reason, examples)
	}
	got := jsBrief(t, "test/shape.spec.ts", `describe('shapes', () => {
  it.failing('fails on purpose', () => { expect(run()).toBe(1); });
  test.skipIf(isWindows)('skip if', () => { expect(run()).toBe(1); });
  it('by reference', handler);
  it('one', () => { expect(run()).toBe(1); }); it('two', () => { expect(run()).toBe(2); });
  describe.runIf(ci)('conditional group', () => {
    it('inside', () => { expect(run()).toBe(1); });
  });
  it('plain', () => { expect(run()).toBe(1); });
});
`)
	wantJS(t, got, map[int][]string{
		2: {testvalidity.HygieneReasonJSShapeUnsupported + " expectations=0"},
		3: {testvalidity.HygieneReasonJSShapeUnsupported + " expectations=0"},
		4: {testvalidity.HygieneReasonJSBodyNotLiteral + " expectations=0"},
		5: {testvalidity.HygieneReasonJSLineAmbiguous + " expectations=0"},
		7: {testvalidity.HygieneReasonJSShapeUnsupported + " expectations=0"},
		9: {located + " expectations=1"},
	})
}

const runStateSpec = `describe('run state', () => {
  it.skip('skipped', () => { expect(f()).toBe(1); });
  xit('x-skipped', () => { expect(f()).toBe(1); });
  it.todo('later');
  it.only('focused', () => { expect(f()).toBe(1); });
  fit('f-focused', () => { expect(f()).toBe(1); });
  it.each([[1, 2], [2, 3]])('row %i', (a, b) => { expect(f(a)).toBe(b); });
  test.each` + "`" + `
    a    | b
    ${1} | ${2}
  ` + "`" + `('template $a', ({ a, b }) => { expect(f(a)).toBe(b); });
  it('runtime skip', () => {
    test.skip();
    expect(f()).toBe(1);
  });
  it('conditional runtime skip', () => {
    test.skip(process.platform === 'win32', 'windows');
    expect(f()).toBe(1);
  });
  test.fixme('fixme', async () => { expect(f()).toBe(1); });
});
describe.skip('skipped group', () => {
  it('inherits', () => { expect(f()).toBe(1); });
});
describe.only('focused group', () => {
  it('inherits focus', () => { expect(f()).toBe(1); });
});
describe.each(rows)('group %i', (n) => {
  it('per row', () => { expect(f(n)).toBe(n); });
});
xdescribe('x group', () => {
  test.skip(true, 'always');
  it('x inherits', () => { expect(f()).toBe(1); });
});
describe('annotated group', () => {
  test.skip();
  it('annotation inherits', () => { expect(f()).toBe(1); });
});
`

// TestJSExampleRunState: skip, todo, focus and each facts with their causes; a conditional runtime
// skip is not a skip (TSA-V0-074..076).
func TestJSExampleRunState(t *testing.T) {
	name := "test/run.test.js"
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	skipped := func(line, cause int) string {
		return fmt.Sprintf("%s invalid %s %s", testvalidity.RuleJSSkipped, at(line), at(cause))
	}
	focused := func(line, cause int) string {
		return fmt.Sprintf("%s invalid %s %s", testvalidity.RuleJSFocused, at(line), at(cause))
	}
	each := func(line, cause int) string {
		return fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSEachRowBinding, at(line), at(cause))
	}
	one := located + " expectations=1"
	got := jsBrief(t, name, runStateSpec)
	wantJS(t, got, map[int][]string{
		2:  {one, skipped(2, 2)},
		3:  {one, skipped(3, 3)},
		4:  {located + " expectations=0", fmt.Sprintf("%s invalid %s %s", testvalidity.RuleJSTodo, at(4), at(4))},
		5:  {one, focused(5, 5)},
		6:  {one, focused(6, 6)},
		7:  {one, each(7, 7)},
		8:  {one, each(8, 8)},
		12: {one, skipped(12, 13)},
		16: {one},
		20: {one, skipped(20, 20)},
		23: {one, skipped(23, 22)},
		26: {one, focused(26, 25)},
		29: {one, each(29, 28)},
		33: {one, skipped(33, 31)},
		37: {one, skipped(37, 36)},
	})
	examples, _ := JSExampleSources(name, runStateSpec)
	for line, rows := range map[int]string{7: "(2 rows)", 8: "(1 rows)", 29: "(rows not counted)"} {
		if f := examples[line].Findings[0]; !strings.Contains(f.Message, rows) {
			t.Errorf("line %d message %q lacks %q", line, f.Message, rows)
		}
	}
}

const assertionsSpec = `import { check } from './helpers';
import { run } from '../src/assertions';
function checkOk(v) {
  expect(v.ok).toBe(true);
}
const deep = (v) => checkOk(v);
describe('assertions', () => {
  it('direct', () => { expect(run()).toBe(2); });
  it('none', () => { run(); });
  it('helper', () => { deep(run()); });
  it('imported helper', () => { check(run()); });
  it('constant', () => {
    expect(1).toBe(1);
    expect(run()).toEqual(run());
    const x = 5;
    expect(x).toBe(5);
  });
  it('assert member', () => { assert.equal(run(), 1); });
  it('should', () => { run().should.equal(1); });
  it('mixed constant', () => { expect(true).toBe(true); expect(run()).toBe(1); });
  it('expect.assertions', () => {
    expect.assertions(1);
    expect(1).toBe(1);
  });
  it('declared count only', () => {
    expect.assertions(0);
    expect.hasAssertions();
    run();
  });
  it('continued initializer', () => {
    const x = 1
      + run();
    expect(x).toBe(2);
  });
  it('ended initializer', () => {
    const y = 1
    expect(y).toBe(1);
  });
});
function d4(v) { expect(v).toBe(1); }
function c3(v) { d4(v); }
function b2(v) { c3(v); }
function a1(v) { b2(v); c3(v); }
it('shorter helper path', () => { a1(run()); });
`

// TestJSExampleAssertions: expectations count direct assertion calls and same-file helpers that
// assert, two on one line counting twice; expect.assertions and expect.hasAssertions are neither
// counted nor read as constant; a helper reached too deep by one path is still followed by a
// shorter one; a body with none is flagged unless it calls an unresolved relative import;
// all-constant expectations are flagged, and an initializer continued on the next line is not a
// literal (TSA-V0-077, TSA-V0-078).
func TestJSExampleAssertions(t *testing.T) {
	name := "test/assertions.test.ts"
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	got := jsBrief(t, name, assertionsSpec)
	wantJS(t, got, map[int][]string{
		8:  {located + " expectations=1"},
		9:  {located + " expectations=0", fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSNoAssertion, at(9), at(9))},
		10: {located + " expectations=1"},
		11: {located + " expectations=0"},
		12: {located + " expectations=3", fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSConstantExpectation, at(13), at(12))},
		18: {located + " expectations=1"},
		19: {located + " expectations=1"},
		20: {located + " expectations=2"},
		21: {located + " expectations=1", fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSConstantExpectation, at(23), at(21))},
		25: {located + " expectations=0", fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSNoAssertion, at(25), at(25))},
		30: {located + " expectations=1"},
		35: {located + " expectations=1", fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSConstantExpectation, at(37), at(35))},
		44: {located + " expectations=1"},
	})
	examples, _ := JSExampleSources(name, assertionsSpec)
	if anchors := examples[20].ExpectationAnchors; !reflect.DeepEqual(anchors, []string{at(20)}) {
		t.Errorf("same-line anchors %q, want one distinct line", anchors)
	}
}

const mocksSpec = `import axios from 'axios';
import { Widget } from '../src/widget';
import { helper } from '../src/other';
jest.mock('../src/widget');
jest.mock('axios');
describe(Widget, () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });
  it('spies on the subject', () => {
    jest.spyOn(Widget.prototype, 'save');
    expect(new Widget().save()).toBe(true);
  });
  it('spies on fetch', () => {
    jest.spyOn(globalThis, 'fetch');
    global.fetch = jest.fn();
    expect(helper()).toBe(1);
  });
  it('mocks an unrelated module', () => {
    jest.mock('../src/other');
    expect(helper()).toBe(1);
  });
});
describe('other', () => {
  it('outside the subject describe', () => {
    vi.spyOn(axios, 'get');
    expect(helper()).toBe(1);
  });
});
function setup() {
  jest.spyOn(Widget, 'load');
}
`

// TestJSExampleMocks: subject and boundary mocks reach the examples they apply to; other module
// mocks and mocks in named helpers give no finding (TSA-V0-079).
func TestJSExampleMocks(t *testing.T) {
	name := "test/widget.test.ts"
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	subject := func(line, cause int) string {
		return fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSSubjectMocked, at(line), at(cause))
	}
	boundary := func(line, cause int) string {
		return fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSBoundaryMocked, at(line), at(cause))
	}
	one := located + " expectations=1"
	got := jsBrief(t, name, mocksSpec)
	wantJS(t, got, map[int][]string{
		10: {one, subject(4, 10), subject(11, 10), boundary(5, 10), boundary(8, 10)},
		14: {one, subject(4, 14), boundary(5, 14), boundary(8, 14), boundary(15, 14), boundary(16, 14)},
		19: {one, subject(4, 19), boundary(5, 19), boundary(8, 19)},
		25: {one, subject(4, 25), boundary(5, 25), boundary(26, 25)},
	})
}

// TestJSExampleBarrelAndAliasSubjects pins TSA-V0-079 (c) and the aliased import (V1-1862): a
// relative barrel that exports the file stem's name is the subject whether or not its specifier
// ends in index, so mocking it is JS-SUBJECT-MOCKED and never JS-BOUNDARY-MOCKED; an
// `import { X as Y }` binding names the subject both when the describe title is the exported X and
// when the local Y is spied on; a CommonJS `{ X: Y }` rename exports X; an identifier describe
// argument is a local binding, never an exported name; an unrelated relative barrel stays
// unclassified.
func TestJSExampleBarrelAndAliasSubjects(t *testing.T) {
	subject := func(name string, line, cause int) string {
		return fmt.Sprintf("%s advisory %s:%d %s:%d", testvalidity.RuleJSSubjectMocked, name, line, name, cause)
	}
	one := located + " expectations=1"
	for _, c := range []struct {
		name, text string
		want       map[int][]string
	}{
		{"src/widget.test.ts", "import { Widget } from './components';\njest.mock('./components');\ndescribe('renders', () => {\n  it('w', () => { expect(new Widget()).toBeTruthy(); });\n});\n",
			map[int][]string{4: {one, subject("src/widget.test.ts", 2, 4)}}},
		{"src/user-service.spec.ts", "import { UserService } from '../services/index';\nvi.mock('../services/index');\ndescribe('renders', () => {\n  it('w', () => { expect(new UserService()).toBeTruthy(); });\n});\n",
			map[int][]string{4: {one, subject("src/user-service.spec.ts", 2, 4)}}},
		{"src/thing.test.ts", "import { Widget as W } from './lib';\ndescribe('Widget', () => {\n  it('w', () => { jest.spyOn(W, 'render'); expect(W).toBeTruthy(); });\n});\n",
			map[int][]string{3: {one, subject("src/thing.test.ts", 3, 3)}}},
		{"src/thing.test.ts", "import { Widget as W } from './lib';\njest.mock('./lib');\ndescribe('Widget', () => {\n  it('w', () => { expect(W).toBeTruthy(); });\n});\n",
			map[int][]string{4: {one, subject("src/thing.test.ts", 2, 4)}}},
		{"src/widget.test.ts", "import { Gadget } from './components';\njest.mock('./components');\ndescribe('renders', () => {\n  it('w', () => { expect(new Gadget()).toBeTruthy(); });\n});\n",
			map[int][]string{4: {one}}},
		// A CommonJS destructuring rename exports Helper under the local Widget.
		{"src/helper.test.ts", "const { Helper: Widget } = require('./lib');\njest.mock('./lib');\ndescribe('renders', () => {\n  it('w', () => { expect(new Widget()).toBeTruthy(); });\n});\n",
			map[int][]string{4: {one, subject("src/helper.test.ts", 2, 4)}}},
		{"src/widget.test.ts", "const { Helper: Widget } = require('./lib');\njest.mock('./lib');\ndescribe('renders', () => {\n  it('w', () => { expect(new Widget()).toBeTruthy(); });\n});\n",
			map[int][]string{4: {one}}},
		// An identifier describe argument names its local binding, never an exported name.
		{"src/thing.test.ts", "import { Widget as W } from './lib';\njest.mock('./lib');\nconst Widget = class {};\ndescribe(Widget, () => {\n  it('w', () => { expect(W).toBeTruthy(); });\n});\n",
			map[int][]string{5: {one}}},
	} {
		wantJS(t, jsBrief(t, c.name, c.text), c.want)
	}
}

const typedSpec = `describe.skip('s', (): void => {
  it('inside', (): void => { expect(f()).toBe(1); });
});
`

// TestJSExampleTypedCallback: a callback with a TypeScript return type is still the describe's
// body, so its run state reaches the examples inside (TSA-V0-073, TSA-V0-074).
func TestJSExampleTypedCallback(t *testing.T) {
	name := "test/typed.test.ts"
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	got := jsBrief(t, name, typedSpec)
	wantJS(t, got, map[int][]string{
		2: {located + " expectations=1", fmt.Sprintf("%s invalid %s %s", testvalidity.RuleJSSkipped, at(2), at(1))},
	})
}

const namedCallbackSpec = `import { Widget } from './widget';
describe(Widget, () => {
  it('works', function example() {
    jest.mock('./widget');
    expect(new Widget().ok()).toBe(true);
  });
});
`

// TestJSExampleNamedCallbackMock: a mock inside the example's own named callback applies to the
// example; only an external named helper is excluded (TSA-V0-079).
func TestJSExampleNamedCallbackMock(t *testing.T) {
	name := "test/named.test.js"
	at := func(line int) string { return fmt.Sprintf("%s:%d", name, line) }
	got := jsBrief(t, name, namedCallbackSpec)
	wantJS(t, got, map[int][]string{
		3: {located + " expectations=1", fmt.Sprintf("%s advisory %s %s", testvalidity.RuleJSSubjectMocked, at(4), at(3))},
	})
}
