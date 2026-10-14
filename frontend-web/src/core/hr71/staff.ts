import { localClient } from '../local/useLocalSession';
import { sourceAccess } from './policy.mjs';
import { missing } from './base';
import type { Adapter } from './contracts';
export const staff: Adapter<'staff'> = async (context) => {
  if (context.mock) return missing('staff', context);
  const state = sourceAccess('staff', context.capabilities);
  if (state !== 'ready') return { state };
  const records = await localClient.staff(context.token);
  if (records.length > 10000) throw Error('limit');
  if (context.signal.aborted) throw new DOMException('Aborted', 'AbortError');
  return {
    state: records.length ? 'ready' : 'empty',
    data: records.map((person) => ({
      id: person.identity_id,
      name: person.name,
      role: person.role,
      status: person.status === 'active' ? 'access_active' : 'access_revoked',
    })),
    at: Date.now(),
  };
};
