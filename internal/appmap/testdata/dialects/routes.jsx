import { createRoutesFromElements, Route } from 'react-router';
export const routes = createRoutesFromElements(
  <Route id="shop" path="/shop" element={<Layout />}>
    <Route id="shop.index" index element={<Home />} />
    <Route id="shop.product" path="products/:id" element={<Product />} />
    <Route id="shop.files" path="files/*" element={<Files />} />
  </Route>
);
