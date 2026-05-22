<template>
  <header class="flex items-center justify-between h-16 px-6 bg-[var(--color-bg)] border-b border-[var(--color-border)]">
    <!-- Left: mobile hamburger -->
    <button
      class="p-2 rounded-md hover:bg-[var(--color-bg-tertiary)] lg:hidden"
      @click="$emit('toggleSidebar')"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </button>

    <div class="flex-1" />

    <!-- Right: controls -->
    <div class="flex items-center gap-3">
      <!-- Language selector -->
      <select
        :value="locale"
        class="text-sm bg-[var(--color-bg)] border border-[var(--color-border)] rounded-md px-2 py-1 text-[var(--color-text)]"
        @change="changeLocale(($event.target as HTMLSelectElement)?.value)"
      >
        <option value="en">EN</option>
        <option value="zh-CN">CN</option>
        <option value="zh-TW">TW</option>
        <option value="ja">JA</option>
        <option value="ru">RU</option>
      </select>

      <!-- Theme toggle -->
      <button
        class="p-2 rounded-md hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)]"
        @click="toggleDark()"
        :title="isDark ? $t('theme.light') : $t('theme.dark')"
      >
        <svg v-if="isDark" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
        <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
        </svg>
      </button>

      <!-- Logout -->
      <button
        class="p-2 rounded-md hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)]"
        @click="handleLogout"
        :title="$t('nav.logout')"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useDarkMode } from "@/composables/useDarkMode";

defineEmits<{
  toggleSidebar: [];
}>();

const { locale } = useI18n();
const router = useRouter();
const auth = useAuthStore();
const { isDark, toggle: toggleDark } = useDarkMode();

function changeLocale(val: string) {
  (locale as any).value = val;
  localStorage.setItem("locale", val);
}

async function handleLogout() {
  await auth.logout();
  router.push("/login");
}
</script>
