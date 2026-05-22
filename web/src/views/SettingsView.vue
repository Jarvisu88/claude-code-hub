<template>
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)] mb-6">{{ $t("pages.settings.title") }}</h1>

    <UiSpinner v-if="loading" :fullPage="true" />

    <UiCard v-else>
      <form @submit.prevent="handleSave" class="space-y-6 max-w-2xl">
        <div v-for="field in settingsFields" :key="field">
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ field }}</label>
          <UiInput
            :modelValue="String(settings[field] ?? '')"
            @update:modelValue="settings[field] = $event"
          />
        </div>

        <div class="pt-4">
          <UiButton type="submit" :loading="saving">{{ $t("pages.settings.saveSettings") }}</UiButton>
        </div>
      </form>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getSystemSettings, updateSystemSettings } from "@/api";
import UiButton from "@/components/ui/UiButton.vue";
import UiCard from "@/components/ui/UiCard.vue";
import UiInput from "@/components/ui/UiInput.vue";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const saving = ref(false);
const settings = ref<Record<string, any>>({});

const settingsFields = computed(() => {
  return Object.keys(settings.value ?? {}).filter((k) => typeof settings.value[k] !== "object");
});

async function fetchData() {
  loading.value = true;
  try {
    const res = await getSystemSettings();
    settings.value = res?.data ?? {};
  } catch {
    settings.value = {};
  } finally {
    loading.value = false;
  }
}

async function handleSave() {
  saving.value = true;
  try {
    await updateSystemSettings(settings.value);
  } catch {
    // error handling
  } finally {
    saving.value = false;
  }
}

onMounted(fetchData);
</script>
