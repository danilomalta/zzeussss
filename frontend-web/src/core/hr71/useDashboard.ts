import { useEffect, useRef, useState } from 'react';
import { useLocalSession } from '../local/useLocalSession';
import { useLocalAccess } from '../local/LocalAccess';
import { LocalAPIError } from '../local/localClient.mjs';
import { canView } from './policy.mjs';
import { adapters } from './sources';
import type { Source, SourceName, Sources } from './contracts';
export type Dashboard = { [K in SourceName]: Source<Sources[K]> };
const initial = (): Dashboard => ({
  staff: { state: 'loading' },
  attendance: { state: 'loading' },
  schedules: { state: 'loading' },
  leave: { state: 'loading' },
  documents: { state: 'loading' },
  payroll: { state: 'loading' },
  finance: { state: 'loading' },
  alerts: { state: 'loading' },
});
export function useDashboard(minutes: number) {
  const token = useLocalSession((s) => s.token),
    session = useLocalSession((s) => s.session),
    { capabilities } = useLocalAccess(),
    [sources, setSources] = useState(initial),
    [revision, setRevision] = useState(0);
  const latest = useRef({ token, session, capabilities });
  latest.current = { token, session, capabilities };
  const scope = [
    session?.tenant_id,
    session?.store_id,
    session?.device_id,
    session?.identity_id,
    token,
  ].join(':');
  const mock = import.meta.env.VITE_HR_MOCKS === 'true';
  useEffect(() => {
    setSources(initial());
    if (!token || !session || !capabilities || !canView(capabilities)) return;
    const controller = new AbortController(),
      context = {
        token,
        session,
        capabilities,
        signal: controller.signal,
        mock,
        empty: mock && import.meta.env.VITE_HR_MOCK_EMPTY === 'true',
      };
    const isCurrent = () =>
      !controller.signal.aborted &&
      useLocalSession.getState().token === token &&
      useLocalSession.getState().session === session &&
      latest.current.capabilities === capabilities;
    for (const name of Object.keys(adapters) as SourceName[])
      void adapters[name](context)
        .then((result) => {
          if (isCurrent()) setSources((old) => ({ ...old, [name]: result }));
        })
        .catch((error) => {
          if (!isCurrent()) return;
          if (error instanceof LocalAPIError && error.status === 401) {
            useLocalSession.getState().invalidate();
            setSources(initial());
            return;
          }
          setSources((old) => ({
            ...old,
            [name]: {
              state: error instanceof LocalAPIError && error.status === 403 ? 'forbidden' : 'error',
            },
          }));
        });
    return () => controller.abort();
  }, [scope, capabilities, revision, mock]);
  useEffect(() => {
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') setRevision((value) => value + 1);
    }, minutes * 60000);
    return () => clearInterval(timer);
  }, [minutes]);
  useEffect(
    () =>
      useLocalSession.subscribe((state) => {
        if (!state.token) setSources(initial());
      }),
    []
  );
  return { sources, refresh: () => setRevision((value) => value + 1), mock };
}
