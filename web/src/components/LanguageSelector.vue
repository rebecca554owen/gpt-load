<script setup lang="ts">
import { NDropdown, NButton, NIcon } from "naive-ui";
import { computed } from "vue";
import { SUPPORTED_LOCALES, setLocale, getCurrentLocaleLabel, type Locale } from "@/locales";
import { Language } from "@vicons/ionicons5";

// Current language label
const currentLabel = computed(() => getCurrentLocaleLabel());

// Dropdown options
const options = computed(() =>
  SUPPORTED_LOCALES.map(locale => ({
    label: locale.label,
    key: locale.key,
  }))
);

// Switch language
const handleSelect = (key: string) => {
  setLocale(key as Locale);
  // Page will auto refresh, no need to prompt
};
</script>

<template>
  <n-dropdown :options="options" @select="handleSelect" trigger="click">
    <n-button quaternary size="medium" class="language-selector-btn">
      <template #icon>
        <n-icon :component="Language" />
      </template>
      {{ currentLabel }}
    </n-button>
  </n-dropdown>
</template>

<style scoped>
.language-selector-btn {
  min-width: 100px;
}

/* Ensure good contrast in dark mode */
:global(.dark) .language-selector-btn {
  color: var(--n-text-color);
}

.language-selector-btn:hover {
  color: var(--n-primary-color);
}
</style>
