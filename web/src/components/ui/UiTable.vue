<template>
  <div class="overflow-x-auto">
    <table class="w-full text-sm text-left">
      <thead class="text-xs uppercase bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)]">
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3 font-medium whitespace-nowrap"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(row, idx) in rows"
          :key="idx"
          class="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-secondary)] transition-colors"
        >
          <td
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3 whitespace-nowrap"
          >
            <slot :name="`cell-${col.key}`" :row="row" :value="row?.[col.key]">
              {{ row?.[col.key] ?? "-" }}
            </slot>
          </td>
        </tr>
        <tr v-if="(rows ?? []).length === 0">
          <td :colspan="columns.length" class="px-4 py-8 text-center text-[var(--color-text-tertiary)]">
            {{ resolvedEmptyText }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { computed } from "vue";

const { t } = useI18n();

const props = withDefaults(
  defineProps<{
    columns: Array<{ key: string; label: string }>;
    rows: any[];
    emptyText?: string;
  }>(),
  {
    rows: () => [],
  }
);

const resolvedEmptyText = computed(() => props.emptyText ?? t("common.noData"));
</script>
