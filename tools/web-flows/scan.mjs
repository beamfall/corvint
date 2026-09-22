import ts from 'typescript';
import { createHash } from 'node:crypto';

export const digest = value => createHash('sha256').update(value).digest('hex');
const idSelector = /^#[A-Za-z][A-Za-z0-9_-]{0,63}$/;
const methods = new Map([
  ['toHaveText', 'text'], ['toContainText', 'contains'],
  ['toHaveCount', 'count'], ['toBeVisible', 'visible'],
]);
const isName = (node, name) => ts.isIdentifier(node) && node.text === name;
const property = (node, name) => ts.isPropertyAccessExpression(node) && node.name.text === name;
const literal = node => {
  if (ts.isStringLiteral(node)) return node.text;
  if (ts.isNumericLiteral(node)) return Number(node.text);
  if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
  if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
  throw new Error('nonliteral');
};

function locator(node) {
  if (!ts.isCallExpression(node) || node.arguments.length !== 1) throw new Error('locator');
  if (!property(node.expression, 'locator') || !isName(node.expression.expression, 'page')) throw new Error('locator');
  const selector = literal(node.arguments[0]);
  if (typeof selector !== 'string' || !idSelector.test(selector)) throw new Error('selector');
  return selector;
}

function assertion(call, anchor) {
  if (!ts.isPropertyAccessExpression(call.expression)) return null;
  const kind = methods.get(call.expression.name.text);
  if (!kind) return null;
  const expect = call.expression.expression;
  if (!ts.isCallExpression(expect) || !isName(expect.expression, 'expect') || expect.arguments.length !== 1) throw new Error('expect');
  const selector = locator(expect.arguments[0]);
  let want = true;
  if (kind === 'visible' && call.arguments.length !== 0) throw new Error('options');
  if (kind !== 'visible') {
    if (call.arguments.length !== 1) throw new Error('options');
    want = literal(call.arguments[0]);
  }
  return { kind, selector, wantDigest: digest(JSON.stringify(want)), anchor };
}

function action(call) {
  if (!ts.isPropertyAccessExpression(call.expression)) throw new Error('action');
  const kind = call.expression.name.text;
  if (kind === 'reload' && isName(call.expression.expression, 'page') && call.arguments.length === 0) return { kind };
  if (kind !== 'fill' && kind !== 'click') throw new Error('action');
  const selector = locator(call.expression.expression);
  if (kind === 'click' && call.arguments.length === 0) return { kind, selector };
  if (kind !== 'fill' || call.arguments.length !== 1) throw new Error('action');
  const value = literal(call.arguments[0]);
  if (typeof value !== 'string' || value.length > 128) throw new Error('fill');
  return { kind, selector, value: digest(value) };
}

function validImport(s) {
  if (!ts.isImportDeclaration(s) || s.moduleSpecifier.text !== '@playwright/test') return false;
  const clause = s.importClause;
  if (!clause || clause.name || clause.isTypeOnly || !clause.namedBindings || !ts.isNamedImports(clause.namedBindings)) return false;
  const names = clause.namedBindings.elements;
  return names.length === 2 && names.every(n => !n.propertyName && !n.isTypeOnly) &&
    names.map(n => n.name.text).sort().join(',') === 'expect,test';
}

function callbackOf(call) {
  if (!isName(call.expression, 'test') || call.arguments.length !== 2 || !ts.isStringLiteral(call.arguments[0])) throw new Error('test');
  const fn = call.arguments[1];
  if (!ts.isArrowFunction(fn) || !ts.isBlock(fn.body) || fn.parameters.length !== 1) throw new Error('callback');
  const p = fn.parameters[0];
  if (p.initializer || p.dotDotDotToken || !ts.isObjectBindingPattern(p.name) || p.name.elements.length !== 1) throw new Error('fixture');
  const element = p.name.elements[0];
  if (!isName(element.name, 'page') || element.propertyName || element.initializer || element.dotDotDotToken) throw new Error('fixture');
  if (!fn.modifiers?.some(m => m.kind === ts.SyntaxKind.AsyncKeyword)) throw new Error('async');
  return fn;
}

function scanTest(statement, sf, path, sourceDigest) {
  const anchorOf = node => ({ path, line: sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1, digest: sourceDigest });
  const anchor = anchorOf(statement);
  const t = { id: digest(`${path}\0${anchor.line}`), anchor, route: '', actions: [], assertions: [], complete: false };
  try {
    if (!ts.isExpressionStatement(statement) || !ts.isCallExpression(statement.expression)) throw new Error('statement');
    const fn = callbackOf(statement.expression);
    let sawAssertion = false;
    for (const s of fn.body.statements) {
      if (!ts.isExpressionStatement(s) || !ts.isAwaitExpression(s.expression) || !ts.isCallExpression(s.expression.expression)) throw new Error('unsupported-body');
      const call = s.expression.expression;
      if (property(call.expression, 'goto') && isName(call.expression.expression, 'page')) {
        if (t.route || t.actions.length || sawAssertion || call.arguments.length !== 1) throw new Error('route');
        t.route = literal(call.arguments[0]);
        if (typeof t.route !== 'string' || !t.route.startsWith('/') || t.route.startsWith('//')) throw new Error('route');
        continue;
      }
      if (!t.route) throw new Error('missing-route');
      const a = assertion(call, anchorOf(s));
      if (a) { t.assertions.push(a); sawAssertion = true; continue; }
      if (sawAssertion) throw new Error('action-after-assertion');
      t.actions.push(action(call));
      if (t.actions.length > 50 || t.assertions.length > 50) throw new Error('budget');
    }
    t.complete = !!t.route;
  } catch { t.complete = false; }
  return t;
}

// This intentionally closed grammar never executes tests or follows imports/configuration.
export function scan(sources, hashes) {
  const tests = [], gaps = [];
  let inventoryComplete = true;
  for (const source of sources) {
    const sf = ts.createSourceFile(source.path, source.text, ts.ScriptTarget.Latest, true);
    const imports = sf.statements.filter(ts.isImportDeclaration);
    let complete = sf.parseDiagnostics.length === 0 && imports.length === 1 && validImport(imports[0]);
    const local = [];
    for (const s of sf.statements) {
      if (ts.isImportDeclaration(s)) continue;
      const t = scanTest(s, sf, source.path, hashes[source.path]);
      local.push(t);
      if (!t.complete) complete = false;
      if (local.length + tests.length > 256) { complete = false; break; }
    }
    if (!complete) {
      inventoryComplete = false;
      gaps.push(`test-syntax-unresolved:${source.path}`);
      for (const t of local) t.complete = false;
    }
    tests.push(...local.slice(0, 256 - tests.length));
  }
  if (sources.length === 0) gaps.push('empty-declared-test-inventory');
  return { tests, gaps, inventoryComplete, parser: `typescript-${ts.version}` };
}
