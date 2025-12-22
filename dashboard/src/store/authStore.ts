import { create } from 'zustand';
import { auth, LoginRequest } from '../lib/auth';

interface User {
  id: number;
  username: string;
  email: string;
  role: string;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  login: (credentials: LoginRequest) => Promise<void>;
  logout: () => void;
  checkAuth: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  
  login: async (credentials: LoginRequest) => {
    const response = await auth.login(credentials);
    set({
      user: response.user,
      isAuthenticated: true,
    });
  },
  
  logout: () => {
    auth.logout();
    set({
      user: null,
      isAuthenticated: false,
    });
  },
  
  checkAuth: () => {
    const user = auth.getUser();
    const isAuthenticated = auth.isAuthenticated();
    set({
      user,
      isAuthenticated,
    });
  },
}));

