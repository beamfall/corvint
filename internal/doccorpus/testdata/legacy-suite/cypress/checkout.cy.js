// Legacy Cypress (Mocha) suite read by the discover-legacy tests (DCP-V1-060..066).
describe('Checkout', () => {
  before(() => {
    cy.task('db:seed', { users: 'admin-users' });
  });

  beforeEach(() => {
    cy.intercept('GET', '/api/cart', { fixture: 'cart.json' }).as('cart');
    cy.visit('/checkout');
  });

  it('shows the cart total', () => {
    cy.get('[data-test=total]').should('have.text', '$42.00');
    cy.contains('Pay now');
    cy.get('nav').contains('a', 'Orders').should('be.visible').and('have.attr', 'href', '/orders');
  });

  it.skip('applies a coupon', () => {
    cy.get('#coupon').type('SAVE10');
    cy.contains('.discount', '10%');
  });

  context('when signed out', () => {
    it('redirects to login', function () {
      cy.visit('/logout');
      cy.location('pathname').should('eq', '/login');
      cy.get('form').should(($form) => {
        expect($form).to.have.length(1);
      });
    });
  });

  it.only('keeps the cart after reload', () => {
    cy.reload();
    expect(true).to.be.true;
  });

  specify('custom command assertion', () => {
    cy.get('.x').should;
  });
});
