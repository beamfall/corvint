import { Page } from '@playwright/test';

export class HomePage {
  constructor(private readonly page: Page) {}

  async open() {
    await this.page.goto('/#!/home');
  }

  async goToTeeSheets() {
    await this.page.getByTestId('nav-teesheets').click();
  }
}
