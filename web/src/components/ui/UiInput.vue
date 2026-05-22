<template>
  <input
    :class="[
      'w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)]',
      'text-[var(--color-text)] placeholder-[var(--color-text-tertiary)]',
      'px-3 py-2 text-sm transition-colors',
      'focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent',
      'disabled:opacity-50 disabled:cursor-not-allowed',
    ]"
    :value="modelValue"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement)?.value)"
    v-bind="$attrs"
  />
</template>

<script setup lang="ts">
defineProps<{
  modelValue?: string | number;
}>();

defineEmits<{
  "update:modelValue": [value: string];
}>();
</script>
