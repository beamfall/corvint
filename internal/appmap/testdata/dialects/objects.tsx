import { createBrowserRouter } from 'react-router';
export const router = createBrowserRouter([
  { id: 'dashboard', path: '/dashboard', element: <Layout />, children: [
    { id: 'dashboard.index', index: true, Component: Home },
    { id: 'dashboard.project', path: 'projects/:projectId', element: <Project /> },
    { id: 'dashboard.files', path: 'files/*', element: <Files /> },
    { id: 'dashboard.absolute', path: '/dashboard/absolute', Component: Absolute },
    { id: 'dashboard.layout', children: [
      { id: 'dashboard.settings', path: 'settings', Component: Settings }
    ] }
  ] }
]);
