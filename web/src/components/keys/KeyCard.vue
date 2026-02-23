<script setup lang="ts">
import type { APIKey, KeyStatus } from "@/types/models";
import {
  AlertCircleOutline,
  CheckmarkCircle,
  CopyOutline,
  EyeOffOutline,
  EyeOutline,
  Pencil,
} from "@vicons/ionicons5";
import { NButton, NButtonGroup, NIcon, NInput, NTag } from "naive-ui";
import { useI18n } from "vue-i18n";
import { maskKey } from "@/utils/display";
import { formatRelativeTime } from "@/utils/format";

export interface KeyRow extends APIKey {
  is_visible: boolean;
}

interface Props {
  keyData: KeyRow;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  "edit-notes": [key: KeyRow];
  "toggle-visibility": [key: KeyRow];
  copy: [key: KeyRow];
  test: [key: KeyRow];
  restore: [key: KeyRow];
  delete: [key: KeyRow];
}>();

const { t } = useI18n();

function getStatusClass(status: KeyStatus): string {
  switch (status) {
    case "active":
      return "status-valid";
    case "invalid":
      return "status-invalid";
    default:
      return "status-unknown";
  }
}

function getDisplayValue(key: KeyRow): string {
  if (key.notes && !key.is_visible) {
    return key.notes;
  }
  return key.is_visible ? key.key_value : maskKey(key.key_value);
}

function handleEditNotes() {
  emit("edit-notes", props.keyData);
}

function handleToggleVisibility() {
  emit("toggle-visibility", props.keyData);
}

function handleCopy() {
  emit("copy", props.keyData);
}

function handleTest() {
  emit("test", props.keyData);
}

function handleRestore() {
  emit("restore", props.keyData);
}

function handleDelete() {
  emit("delete", props.keyData);
}
</script>

<template>
  <div class="key-card" :class="getStatusClass(keyData.status)">
    <div class="key-main">
      <div class="key-section">
        <n-tag v-if="keyData.status === 'active'" type="success" :bordered="false" round>
          <template #icon>
            <n-icon :component="CheckmarkCircle" />
          </template>
          {{ t("keys.validShort") }}
        </n-tag>
        <n-tag v-else :bordered="false" round>
          <template #icon>
            <n-icon :component="AlertCircleOutline" />
          </template>
          {{ t("keys.invalidShort") }}
        </n-tag>
        <n-input class="key-text" :value="getDisplayValue(keyData)" readonly size="small" />
        <div class="quick-actions">
          <n-button
            size="tiny"
            text
            @click="handleEditNotes"
            :title="t('keys.editNotes')"
            :aria-label="t('keys.editNotes')"
          >
            <template #icon>
              <n-icon :component="Pencil" />
            </template>
          </n-button>
          <n-button
            size="tiny"
            text
            @click="handleToggleVisibility"
            :title="t('keys.showHide')"
            :aria-label="t('keys.showHide')"
          >
            <template #icon>
              <n-icon :component="keyData.is_visible ? EyeOffOutline : EyeOutline" />
            </template>
          </n-button>
          <n-button
            size="tiny"
            text
            @click="handleCopy"
            :title="t('common.copy')"
            :aria-label="t('common.copy')"
          >
            <template #icon>
              <n-icon :component="CopyOutline" />
            </template>
          </n-button>
        </div>
      </div>
    </div>

    <div class="key-bottom">
      <div class="key-stats">
        <span class="stat-item">
          {{ t("keys.requestsShort") }}
          <strong>{{ keyData.request_count }}</strong>
        </span>
        <span class="stat-item">
          {{ t("keys.failuresShort") }}
          <strong>{{ keyData.failure_count }}</strong>
        </span>
        <span class="stat-item">
          {{
            keyData.last_used_at ? formatRelativeTime(keyData.last_used_at, t) : t("keys.unused")
          }}
        </span>
      </div>
      <n-button-group class="key-actions">
        <n-button
          class="btn-test"
          size="tiny"
          @click="handleTest"
          :title="t('keys.testKey')"
          :aria-label="t('keys.testKey')"
        >
          {{ t("keys.testShort") }}
        </n-button>
        <n-button
          v-if="keyData.status !== 'active'"
          class="btn-view"
          size="tiny"
          @click="handleRestore"
          :title="t('keys.restoreKey')"
          :aria-label="t('keys.restoreKey')"
        >
          {{ t("keys.restoreShort") }}
        </n-button>
        <n-button
          class="btn-delete"
          size="tiny"
          @click="handleDelete"
          :title="t('keys.deleteKey')"
          :aria-label="t('keys.deleteKey')"
        >
          {{ t("common.deleteShort") }}
        </n-button>
      </n-button-group>
    </div>
  </div>
</template>

<style scoped>
/* Component-specific styles only - all card styles are in components.css */
.key-card {
  /* Use .item-card base styles from components.css */
}

.key-card.status-valid {
  /* Use .item-card.status-valid from components.css */
}

.key-card.status-invalid {
  /* Use .item-card.status-invalid from components.css */
}

.key-card.status-error {
  /* Use .item-card.status-error from components.css */
}

.key-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-weight: 500;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

:root:not(.dark) .key-text {
  color: var(--text-primary);
  background: var(--bg-secondary);
}

:root.dark .key-text {
  color: var(--text-primary);
  background: var(--bg-tertiary);
}

.key-text :deep(.n-input__input-el) {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 13px;
}
</style>
