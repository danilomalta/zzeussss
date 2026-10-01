import { useEffect } from 'react';
import type { ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuthStore } from '../auth/useAuthStore';
import { useLocalSession } from './useLocalSession';

export function OnlineSessionGuard({ children }: { children: ReactNode }) {
  const authenticated = useAuthStore((state) => state.isAuthenticated);
  return authenticated ? <>{children}</> : <Navigate to="/login" replace />;
}
export function LocalSessionGuard({ children }: { children: ReactNode }) {
  const session = useLocalSession((state) => state.session);
  const token = useLocalSession((state) => state.token);
  const invalidate = useLocalSession((state) => state.invalidate);
  useEffect(() => {
    if (!session) return;
    const timer = setTimeout(invalidate, Math.max(0, session.expires_unix * 1000 - Date.now()));
    return () => clearTimeout(timer);
  }, [session, invalidate]);
  if (!token || !session || session.expires_unix * 1000 <= Date.now()) return <Navigate to="/local/login" replace />;
  return <>{children}</>;
}
