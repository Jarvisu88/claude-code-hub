<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("pages.keys.title") }}</h1>
      <UiButton @click="openCreate">{{ $t("pages.keys.createKey") }}</UiButton>
    </div>

    <UiSpinner v-if="loading" :fullPage="true" />

    <UiCard v-else :noPadding="true">
      <UiTable :columns="columns" :rows="items">
        <template #cell-id="{ value }">
          <span class="font-mono text-xs">{{ value }}</span>
        </template>
        <template #cell-key="{ value }">
          <span class="font-mono text-xs">{{ maskKey(value) }}</span>
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

    <UiDialog v-model="dialogOpen" :title="isEditing ? $t('pages.keys.editKey') : $t('pages.keys.createKey')">
      <form @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.keys.keyName") }}</label>
          <UiInput v-model="form.name" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.keys.userId") }}</label>
          <UiInput v-model="form.userId" type="number" />
        </div>
      </form>
      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ $t("common.cancel") }}</UiButton>
        <UiButton @click="handleSave" :loading="saving">{{ $t("common.save") }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog v-model="keyResultOpen" :title="$t('pages.keys.keyCreated')">
      <div class="space-y-3">
        <p class="text-sm text-[var(--color-text-secondary)]">{{ $t("pages.keys.keyCreatedHint") }}</p>
        <div class="flex items-center gap-2">
          <code class="flex-1 block p-3 rounded bg-[var(--color-bg-secondary)] font-mono text-sm break-all select-all">{{ createdKeyValue }}</code>
          <UiButton size="sm" @click="copyKey">{{ $t("pages.keys.copyKey") }}</UiButton>
        </div>
      </div>
      <template #footer>
        <UiButton @click="keyResultOpen = false">{{ $t("common.confirm") }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import { getKeys, createKey, updateKey, deleteKey } from "@/api";
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
const form = ref({ name: "", userId: "" });
const keyResultOpen = ref(false);
const createdKeyValue = ref("");

function generateKey(): string {
  return "sk-" + crypto.randomUUID().replace(/-/g, "").slice(0, 32);
}

const columns = computed(() => [
  { key: "id", label: t("common.id") },
  { key: "name", label: t("common.name") },
  { key: "key", label: t("pages.keys.keyValue") },
  { key: "isEnabled", label: t("common.status") },
  { key: "createdAt", label: t("common.createdAt") },
  { key: "actions", label: t("common.actions") },
]);

function maskKey(key: any): string {
  const s = String(key ?? "");
  if (s.length <= 8) return s;
  return s.slice(0, 4) + "..." + s.slice(-4);
}

async function fetchData() {
  loading.value = true;
  try {
    const res = await getKeys();
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
  form.value = { name: "", userId: "" };
  dialogOpen.value = true;
}

function openEdit(row: any) {
  isEditing.value = true;
  editId.value = row?.id;
  form.value = { name: row?.name ?? "", userId: String(row?.userId ?? "") };
  dialogOpen.value = true;
}

async function handleSave() {
  saving.value = true;
  try {
    if (isEditing.value) {
      await updateKey(editId.value, { name: form.value.name });
    } else {
      const newKey = generateKey();
      await createKey({
        name: form.value.name,
        key: newKey,
        userId: Number(form.value.userId),
      });
      createdKeyValue.value = newKey;
      dialogOpen.value = false;
      keyResultOpen.value = true;
      await fetchData();
      return;
    }
    dialogOpen.value = false;
    await fetchData();
  } catch {
    // error handling
  } finally {
    saving.value = false;
  }
}

function copyKey() {
  navigator.clipboard.writeText(createdKeyValue.value);
}

async function handleDelete(row: any) {
  if (!confirm(t("common.deleteConfirm"))) return;
  try {
    await deleteKey(row?.id);
    await fetchData();
  } catch {
    // error handling
  }
}

onMounted(fetchData);
</script>
