<template>
  <div class="min-h-screen flex items-center justify-center bg-[var(--color-bg-secondary)]">
    <div class="w-full max-w-md p-8">
      <div class="bg-[var(--color-bg)] rounded-xl shadow-lg border border-[var(--color-border)] p-8">
        <div class="text-center mb-8">
          <div class="w-16 h-16 rounded-xl bg-[var(--color-primary)] flex items-center justify-center mx-auto mb-4">
            <span class="text-2xl font-bold text-white">C</span>
          </div>
          <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("auth.title") }}</h1>
          <p class="text-[var(--color-text-secondary)] mt-2 text-sm">{{ $t("auth.subtitle") }}</p>
        </div>

        <form @submit.prevent="handleLogin" class="space-y-4">
          <div>
            <UiInput
              v-model="token"
              type="password"
              :placeholder="$t('auth.tokenPlaceholder')"
              autocomplete="current-password"
            />
          </div>

          <p v-if="error" class="text-sm text-[var(--color-danger)]">{{ $t("auth.loginFailed") }}</p>

          <UiButton type="submit" class="w-full" :loading="loading" :disabled="!token.trim()">
            {{ loading ? $t("auth.loggingIn") : $t("auth.login") }}
          </UiButton>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import UiButton from "@/components/ui/UiButton.vue";
import UiInput from "@/components/ui/UiInput.vue";

const router = useRouter();
const auth = useAuthStore();

const token = ref("");
const loading = ref(false);
const error = ref(false);

async function handleLogin() {
  loading.value = true;
  error.value = false;
  try {
    const ok = await auth.login(token.value);
    if (ok) {
      router.push("/dashboard");
    } else {
      error.value = true;
    }
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}
</script>
