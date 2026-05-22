<template>
  <div class="min-h-screen flex items-center justify-center bg-[var(--color-bg-secondary)]">
    <div class="text-center">
      <h1 class="text-6xl font-bold text-[var(--color-text-tertiary)] mb-4">404</h1>
      <p class="text-xl text-[var(--color-text-secondary)] mb-6">{{ $t("pages.notFound.message") }}</p>
      <router-link
        to="/dashboard"
        class="inline-flex items-center px-4 py-2 rounded-md bg-[var(--color-primary)] text-white hover:bg-[var(--color-primary-hover)] transition-colors"
      >
        {{ $t("pages.notFound.goHome") }}
      </router-link>
    </div>
  </div>
</template>
