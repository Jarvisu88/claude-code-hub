<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("pages.providers.title") }}</h1>
      <UiButton @click="openCreate">{{ $t("pages.providers.createProvider") }}</UiButton>
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

    <UiDialog v-model="dialogOpen" :title="isEditing ? $t('pages.providers.editProvider') : $t('pages.providers.createProvider')" size="lg">
      <form @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.providers.providerName") }}</label>
          <UiInput v-model="form.name" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.providers.providerType") }}</label>
          <select
            v-model="form.providerType"
            class="w-full rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)]"
          >
            <option value="claude">claude</option>
            <option value="openai-compatible">openai-compatible</option>
            <option value="codex">codex</option>
            <option value="gemini">gemini</option>
            <option value="gemini-cli">gemini-cli</option>
            <option value="claude-auth">claude-auth</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.providers.url") }}</label>
          <UiInput v-model="form.url" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.providers.key") }}</label>
          <UiInput v-model="form.key" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.providers.weight") }}</label>
          <UiInput v-model="form.weight" type="number" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.providers.model") }}</label>
          <UiInput v-model="form.model" />
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
import { getProviders, createProvider, updateProvider, deleteProvider } from "@/api";
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
const form = ref({ name: "", providerType: "claude", url: "", key: "", weight: 1, model: "" });

const columns = computed(() => [
  { key: "id", label: t("common.id") },
  { key: "name", label: t("common.name") },
  { key: "providerType", label: t("pages.providers.providerType") },
  { key: "url", label: t("pages.providers.url") },
  { key: "weight", label: t("pages.providers.weight") },
  { key: "isEnabled", label: t("common.status") },
  { key: "actions", label: t("common.actions") },
]);

async function fetchData() {
  loading.value = true;
  try {
    const res = await getProviders();
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
  form.value = { name: "", providerType: "claude", url: "", key: "", weight: 1, model: "" };
  dialogOpen.value = true;
}

function openEdit(row: any) {
  isEditing.value = true;
  editId.value = row?.id;
  form.value = {
    name: row?.name ?? "",
    providerType: row?.providerType ?? "claude",
    url: row?.url ?? "",
    key: row?.key ?? "",
    weight: row?.weight ?? 1,
    model: row?.model ?? "",
  };
  dialogOpen.value = true;
}

async function handleSave() {
  saving.value = true;
  try {
    if (isEditing.value) {
      await updateProvider(editId.value, form.value);
    } else {
      await createProvider(form.value);
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
    await deleteProvider(row?.id);
    await fetchData();
  } catch {
    // error handling
  }
}

onMounted(fetchData);
</script>
