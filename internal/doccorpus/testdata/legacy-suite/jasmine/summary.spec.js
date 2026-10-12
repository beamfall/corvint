// Legacy Jasmine suite read by the discover-legacy tests (DCP-V1-060..066).
describe('Order summary', function () {
  beforeEach(function () {
    seedOrders({ count: 2, status: 'paid' });
    browser.get('/orders');
  });

  it('shows the order count', function () {
    expect(element(by.css('.count')).getText()).toBe('2 orders');
    expect(page.title).toEqual("Orders");
  });

  it('lists paid orders', () => {
    expect(rows()).toContain('paid');
    expect(rows().length > 0).toBeTruthy();
  });

  xit('exports a CSV', function () {
    expect(exportButton()).toBeDefined();
  });

  it('is not written yet');

  it('waits on the backend', function () {
    pending('backend not ready');
  });

  fdescribe('focused filters', function () {
    it('filters by status', function () {
      expect(filter('paid')).toEqual(['A-1', 'A-2']);
    });
  });

  xdescribe('archived orders', function () {
    it('hides archived rows', function () {
      expect(archived()).toBe(0);
    });
  });

  it('checks totals with a custom helper', function () {
    verifyTotals(expect, 42);
  });
});
