<template>
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)] mb-6">{{ $t("dashboard.title") }}</h1>

    <UiSpinner v-if="loading" :fullPage="true" />

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <UiCard v-for="card in statCards" :key="card.label">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-[var(--color-text-secondary)]">{{ card.label }}</p>
            <p class="text-2xl font-bold text-[var(--color-text)] mt-1">{{ card.value }}</p>
          </div>
          <div :class="['w-12 h-12 rounded-lg flex items-center justify-center', card.bgClass]">
            <span class="text-lg" v-html="card.icon"></span>
          </div>
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getOverview } from "@/api";
import UiCard from "@/components/ui/UiCard.vue";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const overview = ref<any>(null);

const statCards = computed(() => [
  {
    label: t("dashboard.totalRequests"),
    value: overview.value?.totalRequests ?? overview.value?.requestCount ?? 0,
    bgClass: "bg-blue-100 dark:bg-blue-900/30",
    icon: '<svg class="w-6 h-6 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" /></svg>',
  },
  {
    label: t("dashboard.totalTokens"),
    value: formatNumber(overview.value?.totalTokens ?? overview.value?.tokenCount ?? 0),
    bgClass: "bg-green-100 dark:bg-green-900/30",
    icon: '<svg class="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" /></svg>',
  },
  {
    label: t("dashboard.activeUsers"),
    value: overview.value?.activeUsers ?? overview.value?.userCount ?? 0,
    bgClass: "bg-purple-100 dark:bg-purple-900/30",
    icon: '<svg class="w-6 h-6 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" /></svg>',
  },
  {
    label: t("dashboard.activeProviders"),
    value: overview.value?.activeProviders ?? overview.value?.providerCount ?? 0,
    bgClass: "bg-orange-100 dark:bg-orange-900/30",
    icon: '<svg class="w-6 h-6 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" /></svg>',
  },
]);

function formatNumber(n: any): string {
  const num = Number(n) || 0;
  if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + "M";
  if (num >= 1_000) return (num / 1_000).toFixed(1) + "K";
  return String(num);
}

onMounted(async () => {
  try {
    const res = await getOverview();
    overview.value = res?.data ?? {};
  } catch {
    overview.value = {};
  } finally {
    loading.value = false;
  }
});
</script>
