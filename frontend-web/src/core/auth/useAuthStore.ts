import { create } from 'zustand';
import { api } from '../api/api';

/*
INTERFACES DE AUTENTICAÇÃO
==========================
Tipagem estrita das informações de usuário e estado global do Zustand.
*/

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
  login: (credentials: { email: string; password: string }) => Promise<void>;
  logout: () => void;
}

/*
STORE DE AUTENTICAÇÃO (ZUSTAND)
===============================
Armazena a sessão ativa do usuário (memória volátil para o JWT) e expõe ações 
de controle de acesso integradas ao Axios.
*/

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  user: null,
  accessToken: null,

  login: async (credentials) => {
    // POST real para o backend na rota /auth/login
    const response = await api.post<{ access_token: string; user: User }>('/auth/login', credentials);
    const { access_token, user } = response.data;

    // Atualiza o estado global com os dados da sessão
    set({
      isAuthenticated: true,
      user,
      accessToken: access_token,
    });
  },

  logout: () => {
    // Reseta o estado global de autenticação
    set({
      isAuthenticated: false,
      user: null,
      accessToken: null,
    });
  },
}));
