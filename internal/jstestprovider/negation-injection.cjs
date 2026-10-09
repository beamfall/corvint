'use strict';
// Corvint step-negation injection module (LPCV-V0-060). The provider's
// controlled config requires it with a private plan and results path; it
// installs only inside Playwright test workers and never edits test or
// application source. Network faults rewrite one application-origin response
// the browser fetched (LPCV-V0-061); DOM faults alter the elements the
// witnessed assertion's locator resolves to while that assertion runs
// (LPCV-V0-062). Each fault reports how often it applied, as JSON lines in the
// results file, never on stdout or stderr.
const fs = require('node:fs');
const path = require('node:path');

function install(planPath, resultsPath) {
  if (process.env.TEST_WORKER_INDEX === undefined) return;
  const plan = JSON.parse(fs.readFileSync(planPath, 'utf8'));
  const record = entry => fs.appendFileSync(resultsPath, JSON.stringify(entry) + '\n');
  const network = plan.filter(entry => entry.kind === 'network');
  const dom = plan.filter(entry => entry.kind === 'dom');
  const playwrightDir = path.dirname(require.resolve('playwright/package.json', {paths: [process.cwd()]}));
  const core = require(require.resolve('playwright-core', {paths: [playwrightDir]}));
  const requestCounts = new Map();
  const domCounts = new Map();
  const patched = new WeakSet();
  const routed = new Set(network.map(entry => entry.originPath));
  const originPath = raw => { try { const url = new URL(raw); return url.origin + url.pathname; } catch { return ''; } };

  async function onRoute(route) {
    const request = route.request();
    const key = request.method() + ' ' + originPath(request.url());
    const ordinal = (requestCounts.get(key) || 0) + 1;
    requestCounts.set(key, ordinal);
    const mine = network.filter(entry => entry.method + ' ' + entry.originPath === key && entry.requestOrdinal === ordinal);
    if (mine.length === 0) return route.fallback();
    const response = await route.fetch();
    let body = await response.text();
    for (const entry of mine) {
      const at = body.indexOf(entry.search);
      if (at < 0 || body.indexOf(entry.search, at + 1) >= 0) { record({id: entry.id, applied: 0}); continue; }
      body = body.slice(0, at) + entry.marker + body.slice(at + entry.search.length);
      record({id: entry.id, applied: 1});
    }
    await route.fulfill({response, body});
  }

  // In-page fault state: applies the fault to each element, reapplies it on
  // DOM changes and on a short interval, and restores it on settle.
  const pageApply = (elements, fault) => {
    const faults = window.__corvintFaults || (window.__corvintFaults = {});
    const state = faults[fault.id] || (faults[fault.id] = {elements: [], originals: new Map(), observer: null, timer: 0});
    const enforce = element => {
      if (fault.action === 'value') {
        if (!state.originals.has(element)) state.originals.set(element, element.value);
        if (element.value !== fault.marker) element.value = fault.marker;
      } else if (fault.action === 'text') {
        if (!state.originals.has(element)) state.originals.set(element, Array.from(element.childNodes));
        if (element.textContent !== fault.marker) element.textContent = fault.marker;
      } else if (fault.action === 'hide') {
        if (!state.originals.has(element)) state.originals.set(element, [element.style.getPropertyValue('display'), element.style.getPropertyPriority('display')]);
        if (element.style.getPropertyValue('display') !== 'none' || element.style.getPropertyPriority('display') !== 'important') element.style.setProperty('display', 'none', 'important');
      }
    };
    for (const element of elements) if (!state.elements.includes(element)) state.elements.push(element);
    const all = () => { for (const element of state.elements) enforce(element); };
    all();
    if (!state.observer) {
      state.observer = new MutationObserver(all);
      state.observer.observe(document, {subtree: true, childList: true, characterData: true, attributes: true});
      state.timer = setInterval(all, 10);
    }
    return state.elements.length;
  };
  const pageSettle = (elements, fault) => {
    const faults = window.__corvintFaults || {};
    const state = faults[fault.id];
    if (!state) return {count: 0, hidden: false};
    if (state.observer) state.observer.disconnect();
    clearInterval(state.timer);
    let count = 0, hidden = false;
    for (const element of state.elements) {
      if (!element.isConnected) continue;
      if (fault.action === 'value' && element.value === fault.marker) count++;
      if (fault.action === 'text' && element.textContent === fault.marker) count++;
      if (fault.action === 'hide' && getComputedStyle(element).display === 'none') { count++; hidden = true; }
      const original = state.originals.get(element);
      if (fault.action === 'value') element.value = original;
      if (fault.action === 'text') element.replaceChildren(...original);
      if (fault.action === 'hide') { if (original[0]) element.style.setProperty('display', original[0], original[1]); else element.style.removeProperty('display'); }
    }
    delete faults[fault.id];
    return {count, hidden};
  };

  function patchFrame(prototype) {
    if (patched.has(prototype)) return;
    patched.add(prototype);
    const original = prototype._expect;
    prototype._expect = async function (expression, options) {
      const key = options.selector + '\u0000' + expression;
      const ordinal = (domCounts.get(key) || 0) + 1;
      domCounts.set(key, ordinal);
      const fault = dom.find(entry => entry.selector === options.selector && entry.expression === expression && entry.domOrdinal === ordinal);
      if (!fault) return original.call(this, expression, options);
      const frame = this;
      const page = {id: fault.id, marker: fault.marker, action: fault.action};
      const evaluate = fn => frame._wrapApiCall(() => frame.locator(options.selector).evaluateAll(fn, page), {internal: true}).catch(() => 0);
      await evaluate(pageApply);
      let settled = false;
      const poll = (async () => { while (!settled) { await new Promise(resolve => setTimeout(resolve, 25)); if (!settled) await evaluate(pageApply); } })();
      let result;
      try {
        result = await original.call(this, expression, options);
      } finally {
        settled = true;
        await poll;
        const observed = await evaluate(pageSettle);
        const passed = result && result.matches === true;
        record({id: fault.id, applied: passed || !observed ? 0 : observed.count, hidden: !passed && !!observed && observed.hidden});
      }
      return result;
    };
  }

  core._instrumentation.addListener({
    runAfterCreateBrowserContext: async context => {
      if (routed.size > 0) await context.route(url => routed.has(originPath(String(url))), onRoute);
      if (dom.length > 0) {
        for (const page of context.pages()) patchFrame(Object.getPrototypeOf(page.mainFrame()));
        context.on('page', page => patchFrame(Object.getPrototypeOf(page.mainFrame())));
      }
    },
  });
}

module.exports = {install};
