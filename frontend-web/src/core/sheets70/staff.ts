import { localClient } from '../local/useLocalSession';
import type { Adapter } from './contracts';
export const staff: Adapter = async (context, filters) => {
  const members = await localClient.staff(context.token);
  return {
    columns: ['person', 'role', 'status'],
    rows: members
      .filter((m) => m.name.toLocaleLowerCase().includes(filters.q.toLocaleLowerCase()))
      .map((m) => [m.name, m.role, m.status]),
    partial: false,
    mock: false,
    at: Date.now(),
  };
};
