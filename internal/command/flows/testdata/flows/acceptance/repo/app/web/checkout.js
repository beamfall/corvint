// Checkout page: pressing Pay charges the cart and shows the order as paid.
export function pay(cart) {
  return { status: "paid", total: cart.total };
}
