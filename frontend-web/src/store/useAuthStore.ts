// Exemplo sugerido usando Zustand (ou Context API).
// import { create } from 'zustand';

/*
AUTH STORE (RBAC - Role Based Access Control)
=============================================
Gerenciador Global do Estado de Autenticação do Frontend.

Regras de Negócio:
1. NUNCA armazene o JWT aqui. O JWT fica no Cookie HttpOnly invisível ao JS.
2. Guarde apenas os DADOS DO USUÁRIO inofensivos (Nome, Email) e, principalmente, 
   o ROLE ('admin', 'pdv_cashier') para esconder ou mostrar telas/botões antes 
   mesmo da requisição chegar no Backend.
*/

export interface UserProfile {
  id: string;
  name: string;
  role: 'admin' | 'manager' | 'cashier';
}

export interface AuthState {
  isAuthenticated: boolean;
  user: UserProfile | null;
  login: (userData: UserProfile) => void;
  logout: () => void;
}

// Pseudo-Zustand Implementation
// export const useAuthStore = create<AuthState>((set) => ({
//   isAuthenticated: false,
//   user: null,
//   login: (userData) => set({ isAuthenticated: true, user: userData }),
//   logout: () => set({ isAuthenticated: false, user: null }),
// }));
