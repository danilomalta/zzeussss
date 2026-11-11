import { create } from 'zustand';
import { api } from '../api/api';
import { interpretOnlineLogout, onlineLogoutRequest } from './onlineLogout.mjs';

export interface User {
  id: string;
  name: string;
  email: string;
  role: string;
}

export interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  accessToken: string | null;
  logoutUncertain: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<'closed' | 'uncertain'>;
  forgetSession: () => void;
}

const empty = { isAuthenticated: false, user: null, accessToken: null };

export const useAuthStore = create<AuthState>((set, get) => ({
  ...empty,
  logoutUncertain: false,

  login: async (email, password) => {
    const response = await api.post<{ access_token: string; user: User }>('/auth/login', {
      email,
      password,
    });
    const { access_token, user } = response.data;
    set({
      isAuthenticated: true,
      user,
      accessToken: access_token,
      logoutUncertain: false,
    });
  },

  forgetSession: () => {
    set({ ...empty, logoutUncertain: false });
  },

  logout: async () => {
    const token = get().accessToken;
    if (!token) {
      set({ ...empty, logoutUncertain: false });
      return 'closed';
    }
    try {
      const response = await api.request(onlineLogoutRequest());
      const outcome = interpretOnlineLogout(response.status);
      if (outcome === 'closed') {
        set({ ...empty, logoutUncertain: false });
        return 'closed';
      }
      set({ logoutUncertain: true });
      return 'uncertain';
    } catch {
      set({ logoutUncertain: true });
      return 'uncertain';
    }
  },
}));
