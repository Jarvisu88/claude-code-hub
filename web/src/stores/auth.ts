import { create } from "zustand";
import { persist } from "zustand/middleware";
import { apiClient } from "@/api/client";
import type { LoginResponse } from "@/api/types";

interface AuthState {
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  login: (adminToken: string) => Promise<void>;
  logout: () => void;
  clearError: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,

      login: async (adminToken: string) => {
        set({ isLoading: true, error: null });
        try {
          const response = await apiClient.post<LoginResponse>("/auth/login", {
            token: adminToken,
          });
          set({
            token: response.token,
            isAuthenticated: true,
            isLoading: false,
            error: null,
          });
        } catch {
          set({
            token: null,
            isAuthenticated: false,
            isLoading: false,
            error: "auth.error",
          });
        }
      },

      logout: () => {
        set({
          token: null,
          isAuthenticated: false,
          isLoading: false,
          error: null,
        });
      },

      clearError: () => {
        set({ error: null });
      },
    }),
    {
      name: "auth-storage",
      partialize: (state) => ({
        token: state.token,
        isAuthenticated: state.isAuthenticated,
      }),
    },
  ),
);
