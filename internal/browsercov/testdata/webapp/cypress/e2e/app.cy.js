describe("app", () => {
  it("adds", () => {
    cy.visit("/");
    cy.get("#add").click();
    cy.get("#out").should("have.text", "5");
  });

  it("signs", () => {
    cy.visit("/");
    cy.get("#sign").click();
    cy.get("#out").should("have.text", "positive");
  });
});
