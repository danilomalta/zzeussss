import { create } from 'zustand';
import { api } from '../api/api';

/*
INTERFACES DO MÓDULO DE AUTENTICAÇÃO
====================================
Tipagem completa das entidades e estado global de sessão.
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
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

/*
ESTADO GLOBAL (ZUSTAND STORE)
==============================
Store volátil de alto desempenho para gerenciamento de JWT e autenticação de usuários.
*/

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  user: null,
  accessToken: null,

  login: async (email, password) => {
    // Realiza a chamada HTTP POST para autenticação
    const response = await api.post<{ access_token: string; user: User }>('/auth/login', {
      email,
      password,
    });
    
    const { access_token, user } = response.data;

    // Atualiza o estado global de autenticação com dados do backend
    set({
      isAuthenticated: true,
      user,
      accessToken: access_token,
    });
  },

  logout: () => {
    // Limpa completamente os tokens e dados de sessão
    set({
      isAuthenticated: false,
      user: null,
      accessToken: null,
    });
  },
}));
