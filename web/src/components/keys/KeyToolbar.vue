<script setup lang="ts">
import { AddCircleOutline, RemoveCircleOutline, Search } from "@vicons/ionicons5";
import { NButton, NDropdown, NIcon, NInput, NInputGroup, NSelect, NSpace } from "naive-ui";
import { useI18n } from "vue-i18n";

type StatusFilter = "all" | "active" | "invalid";

interface Props {
  loading: boolean;
  statusFilter: StatusFilter;
  searchText: string;
}

defineProps<Props>();

const emit = defineEmits<{
  "update:statusFilter": [value: StatusFilter];
  "update:searchText": [value: string];
  "handle-search": [];
  "create-click": [];
  "delete-click": [];
  "more-action": [key: string];
}>();

const { t } = useI18n();

const statusOptions = [
  { label: t("common.all"), value: "all" },
  { label: t("keys.valid"), value: "active" },
  { label: t("keys.invalid"), value: "invalid" },
];

const moreOptions = [
  { label: t("keys.exportAllKeys"), key: "copyAll" },
  { label: t("keys.exportValidKeys"), key: "copyValid" },
  { label: t("keys.exportInvalidKeys"), key: "copyInvalid" },
  { type: "divider" },
  { label: t("keys.restoreAllInvalidKeys"), key: "restoreAll" },
  {
    label: t("keys.clearAllInvalidKeys"),
    key: "clearInvalid",
    props: { style: { color: "var(--error-color)" } },
  },
  {
    label: t("keys.clearAllKeys"),
    key: "clearAll",
    props: { style: { color: "var(--error-color)", fontWeight: "bold" } },
  },
  { type: "divider" },
  { label: t("keys.validateAllKeys"), key: "validateAll" },
  { label: t("keys.validateValidKeys"), key: "validateActive" },
  { label: t("keys.validateInvalidKeys"), key: "validateInvalid" },
];

function handleStatusChange(value: StatusFilter) {
  emit("update:statusFilter", value);
}

function handleSearchTextChange(value: string) {
  emit("update:searchText", value);
}

function handleSearch() {
  emit("handle-search");
}

function handleMoreActionSelect(key: string) {
  emit("more-action", key);
}
</script>

<template>
  <div class="toolbar">
    <div class="toolbar-left">
      <n-button class="btn-create" size="small" @click="emit('create-click')">
        <template #icon>
          <n-icon :component="AddCircleOutline" />
        </template>
        {{ t("keys.addKey") }}
      </n-button>
      <n-button class="btn-delete" size="small" @click="emit('delete-click')">
        <template #icon>
          <n-icon :component="RemoveCircleOutline" />
        </template>
        {{ t("keys.deleteKey") }}
      </n-button>
    </div>
    <div class="toolbar-right">
      <n-space :size="12" align="center">
        <n-select
          :value="statusFilter"
          :options="statusOptions"
          size="small"
          style="width: 120px"
          :placeholder="t('keys.allStatus')"
          @update:value="handleStatusChange"
        />
        <n-input-group>
          <n-input
            :value="searchText"
            :placeholder="t('keys.keyExactMatch')"
            size="small"
            style="width: 200px"
            clearable
            @update:value="handleSearchTextChange"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <n-icon :component="Search" />
            </template>
          </n-input>
          <n-button type="primary" ghost size="small" :disabled="loading" @click="handleSearch">
            {{ t("common.search") }}
          </n-button>
        </n-input-group>
        <n-dropdown :options="moreOptions" trigger="click" @select="handleMoreActionSelect">
          <n-button size="small" tertiary>
            <template #icon>
              <span style="font-size: 16px; font-weight: bold">⋯</span>
            </template>
          </n-button>
        </n-dropdown>
      </n-space>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: var(--card-bg-solid);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  gap: 16px;
  min-height: 64px;
}

.toolbar :deep(.n-button) {
  font-weight: 500;
}

.toolbar-left {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.toolbar-right {
  display: flex;
  gap: 12px;
  align-items: center;
  flex: 1;
  justify-content: flex-end;
  min-width: 0;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    justify-content: space-between;
  }

  .toolbar-right :deep(.n-space) {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
