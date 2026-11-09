// POST /api/orders creates an order and answers 201 Created.
export function createOrder(body) {
  return { status: 201, order: { id: "order-1", items: body.items } };
}
