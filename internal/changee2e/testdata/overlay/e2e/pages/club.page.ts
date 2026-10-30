import { Page } from '@playwright/test';

export class ClubPage {
  constructor(private readonly page: Page) {}

  async seedOpenSlots() {
    await this.page.addInitScript(() => localStorage.setItem('open-slots', '1'));
  }

  async cleanup() {
    await this.page.evaluate(() => localStorage.removeItem('open-slots'));
  }
}
