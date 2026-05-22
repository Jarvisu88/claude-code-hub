<template>
  <div>
    <h1 class="text-2xl font-bold text-[var(--color-text)] mb-6">{{ $t("pages.notifications.title") }}</h1>

    <UiSpinner v-if="loading" :fullPage="true" />

    <div v-else class="space-y-6">
      <!-- Notification Settings -->
      <UiCard>
        <template #header>
          <h2 class="text-lg font-semibold text-[var(--color-text)]">{{ $t("pages.notifications.settings") }}</h2>
        </template>
        <form @submit.prevent="handleSaveSettings" class="space-y-4 max-w-2xl">
          <div v-for="field in notifFields" :key="field">
            <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ field }}</label>
            <UiInput
              :modelValue="String(notifSettings[field] ?? '')"
              @update:modelValue="notifSettings[field] = $event"
            />
          </div>
          <UiButton type="submit" :loading="savingSettings">{{ $t("common.save") }}</UiButton>
        </form>
      </UiCard>

      <!-- Webhook Targets -->
      <UiCard :noPadding="true">
        <template #header>
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold text-[var(--color-text)]">{{ $t("pages.notifications.webhookTargets") }}</h2>
            <UiButton size="sm" @click="openCreate">{{ $t("pages.notifications.createTarget") }}</UiButton>
          </div>
        </template>
        <UiTable :columns="whColumns" :rows="webhookTargets">
          <template #cell-id="{ value }">
            <span class="font-mono text-xs">{{ value }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex gap-2">
              <UiButton size="sm" variant="ghost" @click="openEdit(row)">{{ $t("common.edit") }}</UiButton>
              <UiButton size="sm" variant="danger" @click="handleDeleteTarget(row)">{{ $t("common.delete") }}</UiButton>
            </div>
          </template>
        </UiTable>
      </UiCard>
    </div>

    <UiDialog v-model="dialogOpen" :title="isEditing ? $t('pages.notifications.editTarget') : $t('pages.notifications.createTarget')">
      <form @submit.prevent="handleSaveTarget" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("common.name") }}</label>
          <UiInput v-model="form.name" />
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.notifications.providerType") }}</label>
          <select
            v-model="form.providerType"
            class="w-full rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)]"
          >
            <option value="wechat">wechat</option>
            <option value="feishu">feishu</option>
            <option value="dingtalk">dingtalk</option>
            <option value="telegram">telegram</option>
            <option value="custom">custom</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-[var(--color-text)] mb-1">{{ $t("pages.notifications.webhookUrl") }}</label>
          <UiInput v-model="form.webhookUrl" />
        </div>
      </form>
      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ $t("common.cancel") }}</UiButton>
        <UiButton @click="handleSaveTarget" :loading="savingTarget">{{ $t("common.save") }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  getNotificationSettings,
  updateNotificationSettings,
  getWebhookTargets,
  createWebhookTarget,
  updateWebhookTarget,
  deleteWebhookTarget,
} from "@/api";
import UiButton from "@/components/ui/UiButton.vue";
import UiCard from "@/components/ui/UiCard.vue";
import UiTable from "@/components/ui/UiTable.vue";
import UiDialog from "@/components/ui/UiDialog.vue";
import UiInput from "@/components/ui/UiInput.vue";
import UiSpinner from "@/components/ui/UiSpinner.vue";

const { t } = useI18n();

const loading = ref(true);
const savingSettings = ref(false);
const savingTarget = ref(false);
const notifSettings = ref<Record<string, any>>({});
const webhookTargets = ref<any[]>([]);
const dialogOpen = ref(false);
const isEditing = ref(false);
const editId = ref<any>(null);
const form = ref({ name: "", webhookUrl: "", providerType: "custom" });

const notifFields = computed(() => {
  return Object.keys(notifSettings.value ?? {}).filter(
    (k) => typeof notifSettings.value[k] !== "object"
  );
});

const whColumns = computed(() => [
  { key: "id", label: t("common.id") },
  { key: "name", label: t("common.name") },
  { key: "providerType", label: t("pages.notifications.providerType") },
  { key: "webhookUrl", label: t("pages.notifications.webhookUrl") },
  { key: "actions", label: t("common.actions") },
]);

async function fetchData() {
  loading.value = true;
  try {
    const [settingsRes, targetsRes] = await Promise.all([
      getNotificationSettings().catch(() => ({ data: {} })),
      getWebhookTargets().catch(() => ({ data: [] })),
    ]);
    notifSettings.value = settingsRes?.data ?? {};
    webhookTargets.value = (targetsRes?.data ?? []) as any[];
  } catch {
    notifSettings.value = {};
    webhookTargets.value = [];
  } finally {
    loading.value = false;
  }
}

async function handleSaveSettings() {
  savingSettings.value = true;
  try {
    await updateNotificationSettings(notifSettings.value);
  } catch {
    // error handling
  } finally {
    savingSettings.value = false;
  }
}

function openCreate() {
  isEditing.value = false;
  editId.value = null;
  form.value = { name: "", webhookUrl: "", providerType: "custom" };
  dialogOpen.value = true;
}

function openEdit(row: any) {
  isEditing.value = true;
  editId.value = row?.id;
  form.value = {
    name: row?.name ?? "",
    webhookUrl: row?.webhookUrl ?? "",
    providerType: row?.providerType ?? "custom",
  };
  dialogOpen.value = true;
}

async function handleSaveTarget() {
  savingTarget.value = true;
  try {
    if (isEditing.value) {
      await updateWebhookTarget(editId.value, form.value);
    } else {
      await createWebhookTarget(form.value);
    }
    dialogOpen.value = false;
    await fetchData();
  } catch {
    // error handling
  } finally {
    savingTarget.value = false;
  }
}

async function handleDeleteTarget(row: any) {
  if (!confirm(t("common.deleteConfirm"))) return;
  try {
    await deleteWebhookTarget(row?.id);
    await fetchData();
  } catch {
    // error handling
  }
}

onMounted(fetchData);
</script>
