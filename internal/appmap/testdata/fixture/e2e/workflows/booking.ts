import { Page } from '@playwright/test';
import { HomePage } from '../pages/home.page';
import { TeeSheetPage } from '../pages/teesheet.page';

export async function bookFirstSlot(page: Page) {
  const home = new HomePage(page);
  await home.goToTeeSheets();
  const sheet = new TeeSheetPage(page);
  await sheet.selectSlot();
  await sheet.book();
}
