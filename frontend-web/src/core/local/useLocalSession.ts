import { create } from 'zustand';
import { createLocalClient, LocalAPIError } from './localClient.mjs';
import type { LocalSession } from './localClient.mjs';

export const localClient = createLocalClient();
interface LocalState {
  token: string | null;
  session: LocalSession | null;
  notice: string;
  login: (identityID: string, password: string) => Promise<void>;
  endSession: () => Promise<void>;
  invalidate: () => void;
}
// Local opaque tokens stay only in memory, separate from the online JWT store.
export const useLocalSession = create<LocalState>((set, get) => ({
  token: null, session: null, notice: '',
  login: async (identityID, password) => {
    const result = await localClient.login(identityID, password);
    set({ token: result.token, session: result.session, notice: '' });
  },
  invalidate: () => set({ token: null, session: null, notice: 'Sessão local encerrada ou expirada. Entre novamente.' }),
  endSession: async () => {
    const token = get().token;
    set({ token: null, session: null, notice: '' });
    if (!token) return;
    try { await localClient.logout(token); }
    catch {
      set({ notice: 'A sessão foi removida deste navegador. Não foi possível confirmar sua revogação no servidor; ela continua sujeita à expiração.' });
    }
  },
}));
export function localErrorMessage(error: unknown): string {
  return error instanceof LocalAPIError ? error.message : 'Não foi possível concluir a operação local.';
}
