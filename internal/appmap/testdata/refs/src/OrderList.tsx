export function OrderList({ orders }) {
  return (
    <section id="order-list" className="orders-panel" data-region="orders">
      {orders.map((o) => <div key={o.id} data-testid={`order-row-${o.id}`}>{o.name}</div>)}
      <button data-testid="save-order" className="btn primary">Save</button>
    </section>
  );
}
