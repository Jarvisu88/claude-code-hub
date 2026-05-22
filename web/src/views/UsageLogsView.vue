<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("pages.usage.title") }}</h1>
      <UiButton variant="secondary" @click="fetchData">{{ $t("common.refresh") }}</UiButton>
    </div>

    <UiSpinner v-if="loading" :fullPage="true" />

    <div v-else>
      <UiCard :noPadding="true">
        <UiTable :columns="columns" :rows="items">
          <template #cell-inputTokens="{ value }">
            {{ formatNumber(value) }}
          </template>
          <template #cell-outputTokens="{ value }">
            {{ formatNumber(value) }}
          </template>
          <template #cell-cost="{ value }">
            {{ value != null ? ("$" + Number(value).toFixed(4)) : "-" }}
          </template>
          <template #cell-duration="{ value }">
            {{ value != null ? (Number(value) / 1000).toFixed(2) + "s" : "-" }}
          </template>
          <template #cell-createdAt="{ value }">
            {{ formatDate(value) }}
          </template>
        </UiTable>
      </UiCard>

      <!-- Pagination -->
      <div class="flex items-center justify-between mt-4">
        <span class="text-sm text-[var(--color-text-secondary)]">
          {{ $t("common.total") }}: {{ total }}
        </span>
        <div class="flex gap-2">
          <UiButton size="sm" variant="secondary" :disabled="page <= 1" @click="goPage(page - 1)">
            {{ $t("common.previous") }}
          </UiButton>
          <span class="flex items-center text-sm text-[var(--color-text-secondary)] px-2">
            {{ $t("common.page") }} {{ page }} {{ $t("common.of") }} {{ totalPages }}
          </span>
          <UiButton size="sm" variant="secondary" :disabled="page >= totalPages" @click="goPage(page + 1)">
            {{ $t("common.next") }}
          </UiButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getUsageLogs } from "@/api";
import UiButton from "@/components/ui/UiButton.vue";
import UiCard from "@/components/ui/UiCard.vue";
import UiTable from "@/components/ui/UiTable.vue";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const items = ref<any[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)));

const columns = computed(() => [
  { key: "userName", label: t("pages.usage.user") },
  { key: "model", label: t("pages.usage.model") },
  { key: "inputTokens", label: t("pages.usage.inputTokens") },
  { key: "outputTokens", label: t("pages.usage.outputTokens") },
  { key: "cost", label: t("pages.usage.cost") },
  { key: "duration", label: t("pages.usage.duration") },
  { key: "createdAt", label: t("pages.usage.timestamp") },
]);

function formatNumber(val: any): string {
  const n = Number(val) || 0;
  return n.toLocaleString();
}

function formatDate(val: any): string {
  if (!val) return "-";
  try {
    return new Date(val).toLocaleString();
  } catch {
    return String(val);
  }
}

async function fetchData() {
  loading.value = true;
  try {
    const res = await getUsageLogs({ page: page.value, pageSize: pageSize.value });
    const data = res?.data ?? {};
    items.value = (data?.logs ?? data ?? []) as any[];
    total.value = data?.total ?? 0;
  } catch {
    items.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function goPage(p: number) {
  page.value = p;
  fetchData();
}

onMounted(fetchData);
</script>
