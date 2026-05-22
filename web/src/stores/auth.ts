import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { client } from "@/api/client";

export const useAuthStore = defineStore("auth", () => {
  const token = ref<string>(localStorage.getItem("auth_token") || "");
  const user = ref<any>(null);

  const isAuthenticated = computed(() => !!token.value);

  async function login(key: string) {
    const res = await client.post("/auth/login", { key });
    const data = res?.data;
    if (data?.ok) {
      token.value = key;
      user.value = data?.user ?? null;
      localStorage.setItem("auth_token", key);
      return true;
    }
    return false;
  }

  async function logout() {
    try {
      await client.post("/auth/logout");
    } catch {
      // ignore logout errors
    }
    token.value = "";
    user.value = null;
    localStorage.removeItem("auth_token");
  }

  return { token, user, isAuthenticated, login, logout };
});
