import { create } from "zustand";
import { persist } from "zustand/middleware";

type Theme = "light" | "dark";

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

function applyTheme(theme: Theme) {
  const root = document.documentElement;
  if (theme === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: "light",

      setTheme: (theme: Theme) => {
        applyTheme(theme);
        set({ theme });
      },

      toggleTheme: () => {
        const next = get().theme === "light" ? "dark" : "light";
        applyTheme(next);
        set({ theme: next });
      },
    }),
    {
      name: "theme-storage",
      onRehydrateStorage: () => {
        return (state) => {
          if (state) {
            applyTheme(state.theme);
          }
        };
      },
    },
  ),
);
