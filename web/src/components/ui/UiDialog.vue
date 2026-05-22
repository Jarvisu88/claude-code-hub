<template>
  <Teleport to="body">
    <Transition name="dialog">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center"
        @click.self="$emit('update:modelValue', false)"
      >
        <div class="fixed inset-0 bg-black/50" />
        <div
          :class="[
            'relative z-10 w-full max-h-[90vh] overflow-y-auto rounded-lg',
            'bg-[var(--color-bg)] border border-[var(--color-border)] shadow-xl',
            sizeClass,
          ]"
        >
          <div class="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)]">
            <h3 class="text-lg font-semibold text-[var(--color-text)]">
              {{ title }}
            </h3>
            <button
              class="p-1 rounded hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)]"
              @click="$emit('update:modelValue', false)"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="px-6 py-4">
            <slot />
          </div>
          <div v-if="$slots.footer" class="px-6 py-4 border-t border-[var(--color-border)] flex justify-end gap-3">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    title?: string;
    size?: "sm" | "md" | "lg";
  }>(),
  {
    title: "",
    size: "md",
  }
);

defineEmits<{
  "update:modelValue": [value: boolean];
}>();

const sizeClass = computed(() => {
  const map: Record<string, string> = {
    sm: "max-w-sm mx-4",
    md: "max-w-lg mx-4",
    lg: "max-w-2xl mx-4",
  };
  return map[props.size] ?? map.md;
});
</script>

<style scoped>
.dialog-enter-active,
.dialog-leave-active {
  transition: opacity 0.2s ease;
}
.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
}
</style>
