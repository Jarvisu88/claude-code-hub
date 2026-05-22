import { ref, watchEffect } from "vue";
import { usePreferredDark, useStorage } from "@vueuse/core";

export function useDarkMode() {
  const prefersDark = usePreferredDark();
  const stored = useStorage<"light" | "dark" | "auto">("theme", "auto");

  const isDark = ref(false);

  watchEffect(() => {
    if (stored.value === "auto") {
      isDark.value = prefersDark.value;
    } else {
      isDark.value = stored.value === "dark";
    }

    if (isDark.value) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  });

  function toggle() {
    if (stored.value === "auto") {
      stored.value = prefersDark.value ? "light" : "dark";
    } else if (stored.value === "dark") {
      stored.value = "light";
    } else {
      stored.value = "dark";
    }
  }

  return { isDark, toggle, stored };
}
