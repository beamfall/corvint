// Corvint obligation-ledger qualification fixture (TOL-V0-060).
// Run it unchanged with the Playwright version to qualify and the json
// reporter, for example:
//   npx playwright test obligation-qualification.spec.ts --reporter=json > report.json
// then: corvint-tasks ticket obligations qualify-version --from-playwright-report report.json
import { test, expect } from '@playwright/test';

test.describe('qualification', () => {
  test.describe.configure({ retries: 1 });

  test('soft steps', async () => {
    await test.step('TOLQ-1 passes', async () => {
      expect(1).toBe(1);
    });
    await test.step('TOLQ-2 soft-fails', async () => {
      expect.soft(1).toBe(2);
    });
    await test.step('TOLQ-3 passes after a soft failure', async () => {
      expect(1).toBe(1);
    });
    await test.step('TOLQ-4 parent', async () => {
      await test.step('nested soft failure', async () => {
        expect.soft(1).toBe(2);
      });
    });
  });

  test('TOLQ-5 soft failure outside steps', async () => {
    await test.step('TOLQ-6 passes', async () => {
      expect(1).toBe(1);
    });
    expect.soft(1).toBe(2);
  });

  test('TOLQ-7 fails at retry 0', async ({}, testInfo) => {
    expect(testInfo.retry).toBe(1);
  });

  test('TOLQ-8 passes', async () => {
    expect(1).toBe(1);
  });

  test('TOLQ-9 expected-fail', async () => {
    test.fail();
    expect(1).toBe(2);
  });
});
