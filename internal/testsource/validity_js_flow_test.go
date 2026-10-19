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
