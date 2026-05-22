<template>
  <div class="flex items-center justify-center" :class="fullPage ? 'min-h-[200px]' : ''">
    <div class="w-8 h-8 border-3 border-[var(--color-primary)] border-t-transparent rounded-full animate-spin"></div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    fullPage?: boolean;
  }>(),
  {
    fullPage: false,
  }
);
</script>
