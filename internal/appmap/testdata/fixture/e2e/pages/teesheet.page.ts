import { Page } from '@playwright/test';

export class TeeSheetPage {
  constructor(private readonly page: Page) {}

  async selectSlot() {
    await this.page.getByTestId('slot').first().click();
  }

  async book() {
    await this.page.getByRole('button', { name: 'Book' }).click();
  }

  async status() {
    return this.page.locator('.slot-status');
  }
}
