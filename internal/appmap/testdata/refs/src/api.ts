export const listOrders = () => fetch('/api/v2/orders?limit=20');
export const orderStatus = (id) => fetch(`/api/v2/status/${id}`);
