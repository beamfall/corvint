import { test, expect } from '@playwright/test';

const note = (type: string, description: string) => test.info().annotations.push({ type, description });
const event = (value: object) => note('corvint-behavior-event', JSON.stringify({ browser_context: 'context-1', page: 'page-1', frame: 'main', navigation: '', parent_page: '', behavior: '', criterion: '', matcher: '', locator: '', value: '', passed: true, ...value }));

test.describe('items', () => {
  test('lists items', async () => {
    note('corvint-behavior-fixture', 'seeded-items');
    event({ sequence: 1, kind: 'page', id: '/items', navigation: 'main-frame' });
    expect(['first', 'second']).toHaveLength(2);
    event({ sequence: 2, kind: 'assertion', id: 'items-visible', behavior: 'list-items', criterion: 'items-visible', matcher: 'toHaveText', locator: '#count', value: '2' });
    event({ sequence: 3, kind: 'negative-control', id: 'empty-hidden' });
    event({ sequence: 4, kind: 'page', id: '/done', navigation: 'main-frame' });
    note('corvint-behavior-cleanup', 'passed');
  });

  test('rejects empty item', async () => {
    expect(1).toBe(2);
  });

  test.skip('archives item', async () => {});

  test('retries item', async () => {
    expect(test.info().retry).toBe(1);
  });
});
