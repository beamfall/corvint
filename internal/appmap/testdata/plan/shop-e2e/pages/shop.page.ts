import { Page } from '@playwright/test';

export class ShopPage {
  constructor(private readonly page: Page) {}

  async buyGiftCard() {
    await this.page.getByTestId('gift-card').click();
  }
}
