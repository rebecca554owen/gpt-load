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
.key-card {
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 14px;
  transition:
    transform 0.2s,
    box-shadow 0.2s,
    border-color 0.2s;
  display: flex;
  flex-direction: column;
  gap: 10px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.key-card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-1px);
}

.key-card.status-valid {
  border-color: var(--success-border);
  background: var(--success-bg);
  border-width: 1.5px;
}

.key-card.status-invalid {
  border-color: var(--invalid-border);
  background: var(--card-bg-solid);
  opacity: 0.85;
}

.key-card.status-error {
  border-color: var(--error-border);
  background: var(--error-bg);
}

.key-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.key-section {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.key-bottom {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.key-stats {
  display: flex;
  gap: 8px;
  font-size: 12px;
  overflow: hidden;
  color: var(--text-secondary);
  flex: 1;
  min-width: 0;
}

.stat-item {
  white-space: nowrap;
  color: var(--text-secondary);
}

.stat-item strong {
  color: var(--text-primary);
  font-weight: 600;
}

.key-actions {
  flex-shrink: 0;
  &:deep(.n-button) {
    padding: 0 4px;
  }
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

:deep(.n-input__input-el) {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 13px;
}

.quick-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
</style>
