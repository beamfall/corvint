package testsource

import "testing"

const jsStubPathSpec = `import { checkout } from '../src/checkout';
import { totalFor } from '../src/pricing';
import * as tax from '@/tax';
import axios from 'axios';
import dayjs from 'dayjs';
import { Store } from '../src/store';

describe('checkout', () => {
  it('returns the stubbed total', () => {
    jest.mocked(totalFor).mockReturnValue(42);
    const result = checkout();
    expect(result.total).toBe(42);
  });
  it('asserts the stub directly', () => {
    jest.spyOn(tax, 'rate').mockReturnValue(0.2);
    checkout();
    expect(tax.rate()).toBe(0.2);
  });
  it('stubs an HTTP client', () => {
    jest.spyOn(axios, 'get').mockResolvedValue(42);
    expect(checkout()).toBe(42);
  });
  it('stubs a third-party package', () => {
    jest.spyOn(dayjs, 'unix').mockReturnValue(42);
    expect(checkout().total).toBe(42);
  });
  it('checks a value the stub does not return', () => {
    jest.mocked(totalFor).mockReturnValue(42);
    expect(checkout().count).toBe(3);
  });
  it('stubs an unresolved receiver', () => {
    calculator.total.mockReturnValue(42);
    expect(checkout().total).toBe(42);
  });
  it('stubs through a cast', () => {
    (totalFor as jest.Mock).mockImplementation(() => 'seven');
    expect(checkout().label).toEqual('seven');
  });
  it('spies on an object it never passes', () => {
    const store = new Store();
    jest.spyOn(store, 'load').mockReturnValue([1]);
    expect(checkout().rows).toEqual([1]);
  });
  it('spies on an object it passes', () => {
    const store = new Store();
    vi.spyOn(store, 'load');
    checkout(store);
    expect(store.load).toHaveBeenCalled();
  });
  it('spies on an object literal it never passes', () => {
    const fake = { load: () => 1 };
    vi.spyOn(fake, 'load');
    expect(checkout().rows).toEqual([]);
  });
});
`

// TestStubOnPathJS traces TSA-V0-120..124 for Jest and Vitest: first-party stubs on the path are
// advisory, boundary and third-party stubs and unresolved receivers give no finding, and a spy on
// an object the test builds and never hands on is a static STUB-NEVER-INVOKED.
func TestStubOnPathJS(t *testing.T) {
	got := flowBrief(jsBrief(t, "test/checkout.test.ts", jsStubPathSpec))
	wantExamples(t, got, map[int][]string{
		9:  {"STUB-ON-PATH advisory test/checkout.test.ts:10 test/checkout.test.ts:12"},
		14: {"STUB-ON-PATH advisory test/checkout.test.ts:15 test/checkout.test.ts:17"},
		35: {"STUB-ON-PATH advisory test/checkout.test.ts:36 test/checkout.test.ts:37"},
		39: {"STUB-NEVER-INVOKED advisory test/checkout.test.ts:41 test/checkout.test.ts:40"},
		50: {"STUB-NEVER-INVOKED advisory test/checkout.test.ts:52 test/checkout.test.ts:51"},
	})
}

const jsValueOriginSpec = `import { render, screen } from '@testing-library/react';
import { Banner } from '../src/banner';
import { notify } from '../src/notify';

describe('banner', () => {
  it('asserts on an object it built', () => {
    const user = { name: 'Ann' };
    Banner();
    expect(user.name).toBe('Ann');
  });
  it('asserts on an object the subject receives', () => {
    const user = { name: 'Ann' };
    Banner(user);
    expect(user.name).toBe('Ann');
  });
  it('asserts on the subject result', () => {
    expect(Banner().title).toBe('hi');
  });
  it('asserts on a DOM it wrote', () => {
    document.body.innerHTML = '<p>hi</p>';
    expect(screen.getByText('hi')).toBeTruthy();
  });
  it('asserts on a rendered DOM', () => {
    document.body.innerHTML = '<p>hi</p>';
    render(Banner());
    expect(screen.getByText('hi')).toBeTruthy();
  });
  it('asserts on an element it created', () => {
    const el = document.createElement('div');
    el.textContent = 'hi';
    expect(el.textContent).toBe('hi');
  });
  it('asserts on an element it hands to the subject', () => {
    const el = document.createElement('div');
    notify(el);
    expect(el.textContent).toBe('hi');
  });
  it('calls the mock itself', () => {
    const cb = vi.fn();
    cb('saved');
    expect(cb).toHaveBeenCalledWith('saved');
  });
  it('lets the subject call the mock', () => {
    const cb = vi.fn();
    notify(cb);
    expect(cb).toHaveBeenCalledWith('saved');
  });
  it('calls the mock inside a callback', () => {
    const cb = vi.fn();
    notify(() => cb('saved'));
    expect(cb).toHaveBeenCalledWith('saved');
  });
  it('asserts on an unknown call', () => {
    const v = compute();
    expect(v).toBe(1);
  });
});
`

// TestAssertedValueNotFromSubjectJS traces TSA-V0-126..129 for Jest and Vitest: a test-built
// object, a DOM the test wrote with nothing that reaches the subject, a self-built element and a
// call expectation the test meets itself are advisory; a value the subject receives, the subject's
// result, a rendered DOM, a callback the subject runs and an unresolved call are not reported.
func TestAssertedValueNotFromSubjectJS(t *testing.T) {
	got := flowBrief(jsBrief(t, "test/banner.test.ts", jsValueOriginSpec))
	wantExamples(t, got, map[int][]string{
		6:  {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory test/banner.test.ts:9 test/banner.test.ts:7"},
		19: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory test/banner.test.ts:21 test/banner.test.ts:20"},
		28: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory test/banner.test.ts:31 test/banner.test.ts:29"},
		38: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory test/banner.test.ts:41 test/banner.test.ts:40"},
	})
}

const jsFlowReviewHooks = `import { checkout } from '../src/checkout';
import { totalFor } from '../src/pricing';

describe('checkout', () => {
  beforeEach(() => {
    vi.mocked(totalFor).mockReturnValue(42);
  });
  afterEach(() => {
    vi.mocked(totalFor).mockReturnValue(42);
  });
  it('returns the total', () => {
    expect(checkout().total).toBe(42);
  });
});
`

const jsFlowReviewSpec = `import { checkout } from '../src/checkout';
import { notify } from '../src/notify';

describe('checkout', () => {
  it('writes the subject result onto a local', () => {
    const result = {};
    result.total = checkout().total;
    expect(result.total).toBe(42);
  });
  it('passes the spied object inside the assertion', () => {
    const store = { load: () => 42 };
    vi.spyOn(store, 'load');
    expect(checkout(store)).toBe(42);
  });
  it('expects the second call', () => {
    const cb = vi.fn();
    cb('saved');
    notify(cb);
    expect(cb).toHaveBeenNthCalledWith(2, 'saved');
  });
  it('clears the mock before the action', () => {
    const cb = vi.fn();
    cb('saved');
    cb.mockClear();
    notify(cb);
    expect(cb).toHaveBeenCalledWith('saved');
  });
  it('compares fixture fields', () => {
    const fixture = { items: [1], expectedCount: 1 };
    expect(fixture.items).toHaveLength(fixture.expectedCount);
  });
});
`

const jsFlowReviewDOM = `import * as helpers from './helpers';

it('updates the page', () => {
  document.body.innerHTML = '<p>old</p>';
  helpers.update();
  expect(document.body.textContent).toBe('new');
});
`

// TestFlowReviewJS traces the #1184 review regressions for TSA-V0-122, TSA-V0-124 and
// TSA-V0-126..128: a beforeEach stub is on-path and an afterEach stub is not read, a member write
// of a subject value and a spy handed to the subject inside an expectation escape, ordered call
// expectations and calls before a mock clear give no finding, matcher arguments that only read a
// fixture do not hide it, and a call through an imported namespace can write the DOM.
func TestFlowReviewJS(t *testing.T) {
	wantExamples(t, flowBrief(jsBrief(t, "test/checkout.test.ts", jsFlowReviewHooks)), map[int][]string{
		11: {"STUB-ON-PATH advisory test/checkout.test.ts:6 test/checkout.test.ts:12"},
	})
	wantExamples(t, flowBrief(jsBrief(t, "test/checkout.test.ts", jsFlowReviewSpec)), map[int][]string{
		28: {"ASSERTED-VALUE-NOT-FROM-SUBJECT advisory test/checkout.test.ts:30 test/checkout.test.ts:29"},
	})
	wantExamples(t, flowBrief(jsBrief(t, "test/widget.test.ts", jsFlowReviewDOM)), map[int][]string{})
}
