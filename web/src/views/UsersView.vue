<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-[var(--color-text)]">{{ $t("pages.users.title") }}</h1>
      <UiButton @click="openCreate">{{ $t("pages.users.createUser") }}</UiButton>
    </div>

    <UiSpinner v-if="loading" :fullPage="true" />

    <UiCard v-else :noPadding="true">
      <UiTable :columns="columns" :rows="items">
        <template #cell-id="{ value }">
          <span class="font-mono text-xs">{{ value }}</span>
        </template>
        <template #cell-role="{ value }">
          <UiBadge :variant="value === 'admin' ? 'info' : 'default'">{{ value ?? "-" }}</UiBadge>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex gap-2">
            <UiButton size="sm" variant="ghost" @click="openEdit(row)">{{ $t("common.edit") }}</UiButton>
            <UiButton size="sm" variant="danger" @click="handleDelete(row)">{{ $t("common.delete") }}</UiButton>
          </div>
        </template>
      </UiTable>
    </UiCard>

    <UiDialog v-model="dialogOpen" :title="isEditing ? $t('pages.users.editUser') : $t('pages.users.createUser')">
      <form @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.users.userName") }}</label>
          <UiInput v-model="form.name" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.users.userRole") }}</label>
          <UiInput v-model="form.role" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.users.email") }}</label>
          <UiInput v-model="form.email" type="email" />
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
import { getUsers, createUser, updateUser, deleteUser } from "@/api";
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
const form = ref({ name: "", role: "", email: "" });

const columns = computed(() => [
  { key: "id", label: t("common.id") },
  { key: "name", label: t("common.name") },
  { key: "role", label: t("common.role") },
  { key: "email", label: t("pages.users.email") },
  { key: "createdAt", label: t("common.createdAt") },
  { key: "actions", label: t("common.actions") },
]);

async function fetchData() {
  loading.value = true;
  try {
    const res = await getUsers();
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
  form.value = { name: "", role: "", email: "" };
  dialogOpen.value = true;
}

function openEdit(row: any) {
  isEditing.value = true;
  editId.value = row?.id;
  form.value = { name: row?.name ?? "", role: row?.role ?? "", email: row?.email ?? "" };
  dialogOpen.value = true;
}

async function handleSave() {
  saving.value = true;
  try {
    if (isEditing.value) {
      await updateUser(editId.value, form.value);
    } else {
      await createUser(form.value);
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
    await deleteUser(row?.id);
    await fetchData();
  } catch {
    // error handling
  }
}

onMounted(fetchData);
</script>
