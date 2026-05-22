<template>
  <div
    :class="[
      'rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)]',
      'shadow-sm',
      $attrs.class,
    ]"
  >
    <div v-if="$slots.header" class="px-6 py-4 border-b border-[var(--color-border)]">
      <slot name="header" />
    </div>
    <div :class="['px-6 py-4', noPadding ? 'p-0' : '']">
      <slot />
    </div>
    <div v-if="$slots.footer" class="px-6 py-4 border-t border-[var(--color-border)]">
      <slot name="footer" />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    noPadding?: boolean;
  }>(),
  {
    noPadding: false,
  }
);
</script>
