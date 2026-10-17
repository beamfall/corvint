import { createBrowserRouter } from 'react-router-dom';

export const router = createBrowserRouter([
  {id: 'home', path: '/home'},
  {id: 'orders', path: '/orders'},
  {id: 'order', path: '/orders/:orderId'},
  {id: 'files', path: '/files/*'},
  {id: 'fallback', path: '*'},
]);
