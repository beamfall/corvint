// Modelled on the shape of a hooks function component that renders a JSX Routes tree (V1-1712):
// hooks, conditionals and expression props elsewhere in the file, literal Route paths.
import React, { useEffect, useState } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';

function NarrowAlternate({ name }: { name: string }) {
  const narrow = useNarrow();
  return narrow ? <Narrow name={name} /> : <Wide name={name} />;
}

export function FinancesApp() {
  const location = useLocation();
  const [ready, setReady] = useState(false);
  const accounts = useAccounts();

  useEffect(() => {
    if (accounts.length > 0 && !ready) {
      setReady(true);
    }
  }, [accounts, ready]);

  if (!ready) {
    setTimeout(() => setReady(true), 0);
  }

  return (
    <View style={{ height: '100%', flexDirection: 'row' }}>
      <Sidebar collapsed={accounts.length === 0 ? true : false} />
      <ErrorBoundary resetKeys={[location.pathname]} fallback={<Fallback />}>
        <Routes>
          <Route path="/" element={<Navigate to="/budget" replace />} />
          <Route path="/reports/*" element={<NarrowAlternate name="Reports" />} />
          <Route path="/budget" element={<NarrowAlternate name="Budget" />} />
          {/* a comment child */}
          {accounts.length > 0 && <Route path="/hidden" element={<Hidden />} />}
          <Route path="/accounts/:id" element={<NarrowAlternate name="Account" />} />
          <Route path="*" element={<Navigate to="/budget" replace />} />
        </Routes>
      </ErrorBoundary>
    </View>
  );
}
