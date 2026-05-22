<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("pages.errorRules.title") }}</h1>
      <UiButton @click="openCreate">{{ $t("pages.errorRules.createRule") }}</UiButton>
    </div>

    <UiSpinner v-if="loading" :fullPage="true" />

    <UiCard v-else :noPadding="true">
      <UiTable :columns="columns" :rows="items">
        <template #cell-id="{ value }">
          <span class="font-mono text-xs">{{ value }}</span>
        </template>
        <template #cell-isEnabled="{ value }">
          <UiBadge :variant="value ? 'success' : 'default'">
            {{ value ? $t("common.enabled") : $t("common.disabled") }}
          </UiBadge>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex gap-2">
            <UiButton size="sm" variant="ghost" @click="openEdit(row)">{{ $t("common.edit") }}</UiButton>
            <UiButton size="sm" variant="danger" @click="handleDelete(row)">{{ $t("common.delete") }}</UiButton>
          </div>
        </template>
      </UiTable>
    </UiCard>

    <UiDialog v-model="dialogOpen" :title="isEditing ? $t('pages.errorRules.editRule') : $t('pages.errorRules.createRule')">
      <form @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("common.name") }}</label>
          <UiInput v-model="form.name" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.errorRules.pattern") }}</label>
          <UiInput v-model="form.pattern" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.errorRules.action") }}</label>
          <UiInput v-model="form.action" />
        </div>
      </form>
      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ $t("common.cancel") }}</UiButton>
        <UiButton @click="handleSave" :loading="saving">{{ $t("common.save") }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getErrorRules, createErrorRule, updateErrorRule, deleteErrorRule } from "@/api";
import UiButton from "@/components/ui/UiButton.vue";
import UiCard from "@/components/ui/UiCard.vue";
import UiTable from "@/components/ui/UiTable.vue";
import UiDialog from "@/components/ui/UiDialog.vue";
import UiInput from "@/components/ui/UiInput.vue";
import UiBadge from "@/components/ui/UiBadge.vue";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const saving = ref(false);
const items = ref<any[]>([]);
const dialogOpen = ref(false);
const isEditing = ref(false);
const editId = ref<any>(null);
const form = ref({ name: "", pattern: "", action: "" });

const columns = computed(() => [
  { key: "id", label: t("common.id") },
  { key: "name", label: t("common.name") },
  { key: "pattern", label: t("pages.errorRules.pattern") },
  { key: "action", label: t("pages.errorRules.action") },
  { key: "isEnabled", label: t("common.status") },
  { key: "actions", label: t("common.actions") },
]);

async function fetchData() {
  loading.value = true;
  try {
    const res = await getErrorRules();
    items.value = (res?.data ?? []) as any[];
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  isEditing.value = false;
  editId.value = null;
  form.value = { name: "", pattern: "", action: "" };
  dialogOpen.value = true;
}

function openEdit(row: any) {
  isEditing.value = true;
  editId.value = row?.id;
  form.value = { name: row?.name ?? "", pattern: row?.pattern ?? "", action: row?.action ?? "" };
  dialogOpen.value = true;
}

async function handleSave() {
  saving.value = true;
  try {
    if (isEditing.value) {
      await updateErrorRule(editId.value, form.value);
    } else {
      await createErrorRule(form.value);
    }
    dialogOpen.value = false;
    await fetchData();
  } catch {
    // error handling
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: any) {
  if (!confirm(t("common.deleteConfirm"))) return;
  try {
    await deleteErrorRule(row?.id);
    await fetchData();
  } catch {
    // error handling
  }
}

onMounted(fetchData);
</script>
