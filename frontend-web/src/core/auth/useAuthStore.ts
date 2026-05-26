// Global Auth Store following Zustand pattern

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
