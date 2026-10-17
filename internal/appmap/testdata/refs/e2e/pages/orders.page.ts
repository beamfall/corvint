export class OrdersPage {
  constructor(page) {
    this.page = page;
  }

  async save() {
    await this.page.getByTestId('save-order').click();
  }

  async cancel() {
    await this.page.getByTestId('cancel-order').click();
  }
}
