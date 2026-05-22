<template>
  <div class="min-h-screen bg-gray-950 text-gray-100 p-6">
    <div class="flex items-center justify-between mb-8">
      <h1 class="text-3xl font-bold">{{ $t("pages.bigScreen.title") }}</h1>
      <div class="flex gap-4 items-center">
        <select
          v-model="period"
          class="bg-gray-800 border border-gray-700 rounded-md px-3 py-1.5 text-sm text-gray-200"
          @change="fetchData"
        >
          <option value="1d">1D</option>
          <option value="7d">7D</option>
          <option value="30d">30D</option>
        </select>
        <router-link
          to="/dashboard"
          class="text-sm text-gray-400 hover:text-gray-200 underline"
        >
          {{ $t("common.back") }}
        </router-link>
      </div>
    </div>

    <UiSpinner v-if="loading" :fullPage="true" />

    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
      <div
        v-for="card in statCards"
        :key="card.label"
        class="rounded-xl border border-gray-800 bg-gray-900 p-6"
      >
        <p class="text-sm text-gray-400 mb-2">{{ card.label }}</p>
        <p class="text-3xl font-bold">{{ card.value }}</p>
      </div>
    </div>

    <!-- Leaderboard -->
    <div class="rounded-xl border border-gray-800 bg-gray-900 p-6">
      <h2 class="text-lg font-semibold mb-4">{{ $t("pages.bigScreen.leaderboard") }}</h2>
      <div v-if="(leaderboard ?? []).length === 0" class="text-gray-500 text-center py-8">
        {{ $t("dashboard.noData") }}
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="(entry, idx) in (leaderboard ?? [])"
          :key="idx"
          class="flex items-center justify-between py-2 px-3 rounded-lg hover:bg-gray-800 transition-colors"
        >
          <div class="flex items-center gap-3">
            <span class="w-8 h-8 rounded-full bg-gray-800 flex items-center justify-center text-sm font-bold">
              {{ idx + 1 }}
            </span>
            <span class="text-sm">{{ entry?.name ?? entry?.userName ?? "-" }}</span>
          </div>
          <span class="text-sm font-mono text-gray-400">
            {{ formatNumber(entry?.totalTokens ?? entry?.tokens ?? 0) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getOverview, getLeaderboard } from "@/api";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const period = ref("7d");
const overview = ref<any>(null);
const leaderboard = ref<any[]>([]);

const statCards = computed(() => [
  {
    label: t("dashboard.totalRequests"),
    value: overview.value?.totalRequests ?? overview.value?.requestCount ?? 0,
  },
  {
    label: t("dashboard.totalTokens"),
    value: formatNumber(overview.value?.totalTokens ?? overview.value?.tokenCount ?? 0),
  },
  {
    label: t("dashboard.activeUsers"),
    value: overview.value?.activeUsers ?? overview.value?.userCount ?? 0,
  },
  {
    label: t("dashboard.activeProviders"),
    value: overview.value?.activeProviders ?? overview.value?.providerCount ?? 0,
  },
]);

function formatNumber(n: any): string {
  const num = Number(n) || 0;
  if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + "M";
  if (num >= 1_000) return (num / 1_000).toFixed(1) + "K";
  return String(num);
}

async function fetchData() {
  loading.value = true;
  try {
    const [overviewRes, leaderboardRes] = await Promise.all([
      getOverview().catch(() => ({ data: {} })),
      getLeaderboard(period.value).catch(() => ({ data: [] })),
    ]);
    overview.value = overviewRes?.data ?? {};
    leaderboard.value = (leaderboardRes?.data ?? []) as any[];
  } catch {
    overview.value = {};
    leaderboard.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(fetchData);
</script>
