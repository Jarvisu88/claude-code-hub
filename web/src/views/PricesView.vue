<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("pages.prices.title") }}</h1>
      <UiButton variant="secondary" @click="fetchData">{{ $t("common.refresh") }}</UiButton>
    </div>

    <UiSpinner v-if="loading" :fullPage="true" />

    <UiCard v-else :noPadding="true">
      <UiTable :columns="columns" :rows="items">
        <template #cell-inputPrice="{ value }">
          {{ value != null ? ("$" + Number(value).toFixed(6)) : "-" }}
        </template>
        <template #cell-outputPrice="{ value }">
          {{ value != null ? ("$" + Number(value).toFixed(6)) : "-" }}
        </template>
      </UiTable>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getPrices } from "@/api";
import UiButton from "@/components/ui/UiButton.vue";
import UiCard from "@/components/ui/UiCard.vue";
import UiTable from "@/components/ui/UiTable.vue";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const items = ref<any[]>([]);

const columns = computed(() => [
  { key: "model", label: t("pages.prices.model") },
  { key: "inputPrice", label: t("pages.prices.inputPrice") },
  { key: "outputPrice", label: t("pages.prices.outputPrice") },
]);

async function fetchData() {
  loading.value = true;
  try {
    const res = await getPrices();
    items.value = (res?.data ?? []) as any[];
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(fetchData);
</script>
