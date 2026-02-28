<script setup lang="ts">
import { keysApi } from "@/api/keys";

defineOptions({
  name: "KeyTable",
});
import type { Group, KeyRow, KeyStatus } from "@/types/models";
import { useAppStateStore } from "@/stores/appState";
import { useCopy } from "@/composables/useCopy";
import { getGroupDisplayName, maskKey } from "@/utils/display";
import { formatDuration } from "@/utils/format";
import { PAGINATION } from "@/constants/chart";
import { NButton, NEmpty, NInput, NModal, NSpin, useDialog, type MessageReactive } from "naive-ui";
import { h, ref, watch, nextTick, computed } from "vue";
import { useI18n } from "vue-i18n";
import KeyCreateDialog from "./KeyCreateDialog.vue";
import KeyDeleteDialog from "./KeyDeleteDialog.vue";
import KeyCard from "./KeyCard.vue";
import KeyToolbar from "./KeyToolbar.vue";
import KeyPagination from "./KeyPagination.vue";
import { useVirtualGrid } from "@/composables/useVirtualList";

const { t } = useI18n();
const { copyWithFeedback } = useCopy();

const appState = useAppStateStore();

interface Props {
  selectedGroup: Group | null;
}

const props = defineProps<Props>();

const keys = ref<KeyRow[]>([]);
const loading = ref(false);
const searchText = ref("");
const statusFilter = ref<"all" | "active" | "invalid">("all");
const currentPage = ref(1);
const pageSize = ref<12 | 24 | 60 | 120>(PAGINATION.defaultPageSize);
const total = ref(0);
const totalPages = ref(0);
const dialog = useDialog();
const confirmInput = ref("");

const testingMsg = ref<MessageReactive | null>(null);
const isDeling = ref(false);
const isRestoring = ref(false);

const createDialogShow = ref(false);
const deleteDialogShow = ref(false);

const notesDialogShow = ref(false);
const editingKey = ref<KeyRow | null>(null);
const editingNotes = ref("");

// Virtual scrolling container
const gridContainerRef = ref<HTMLElement | null>(null);
const containerHeight = computed(() => {
  if (!gridContainerRef.value) {
    return 600;
  }
  // Use the actual container height minus toolbar and pagination
  const containerHeight = gridContainerRef.value.clientHeight;
  return Math.max(400, containerHeight - 150); // Reserve space for toolbar and pagination
});

// Use virtual scrolling for large datasets
const shouldUseVirtualScroll = computed(() => {
  return keys.value.length > 24; // Only use virtual scroll for more than 24 items
});

// Set up virtual list
const {
  list: virtualList,
  containerProps,
  wrapperProps,
} = useVirtualGrid(keys, containerHeight.value);

watch(
  () => props.selectedGroup,
  async newGroup => {
    if (newGroup) {
      const willWatcherTrigger = currentPage.value !== 1 || statusFilter.value !== "all";
      resetPage();
      if (!willWatcherTrigger) {
        await loadKeys();
      }
    }
  },
  { immediate: true }
);

watch([currentPage, pageSize], async () => {
  await loadKeys();
});

watch(statusFilter, async () => {
  if (currentPage.value !== 1) {
    currentPage.value = 1;
  } else {
    await loadKeys();
  }
});

// Watch for task completion events (e.g., bulk validation, import, delete)
// When a task completes for the current group, refresh the key list
watch(
  () => appState.groupDataRefreshTrigger,
  async () => {
    if (props.selectedGroup) {
      const isCurrentGroupTask =
        appState.lastCompletedTask &&
        appState.lastCompletedTask.groupName === props.selectedGroup.name;

      const isRelevantTaskType =
        isCurrentGroupTask &&
        ["KEY_VALIDATION", "KEY_IMPORT", "KEY_DELETE"].includes(
          appState.lastCompletedTask?.taskType || ""
        );

      if (isRelevantTaskType) {
        await loadKeys();
      }
    }
  }
);

// Watch for sync operation events (e.g., single key test, restore, delete)
// When a sync operation completes for the current group, refresh the key list
watch(
  () => appState.syncOperationTrigger,
  async () => {
    if (props.selectedGroup) {
      const isCurrentGroupSync =
        appState.lastSyncOperation &&
        appState.lastSyncOperation.groupName === props.selectedGroup.name;

      if (isCurrentGroupSync) {
        await loadKeys();
      }
    }
  }
);

function handleSearchInput() {
  if (currentPage.value !== 1) {
    currentPage.value = 1;
  } else {
    loadKeys();
  }
}

function handleMoreAction(key: string) {
  switch (key) {
    case "copyAll":
      copyAllKeys();
      break;
    case "copyValid":
      copyValidKeys();
      break;
    case "copyInvalid":
      copyInvalidKeys();
      break;
    case "restoreAll":
      restoreAllInvalid();
      break;
    case "validateAll":
      validateKeys("all");
      break;
    case "validateActive":
      validateKeys("active");
      break;
    case "validateInvalid":
      validateKeys("invalid");
      break;
    case "clearInvalid":
      clearAllInvalid();
      break;
    case "clearAll":
      clearAll();
      break;
  }
}

async function loadKeys() {
  if (!props.selectedGroup?.id) {
    return;
  }

  try {
    loading.value = true;
    const result = await keysApi.getGroupKeys({
      group_id: props.selectedGroup.id,
      page: currentPage.value,
      page_size: pageSize.value,
      status: statusFilter.value === "all" ? undefined : (statusFilter.value as KeyStatus),
      key_value: searchText.value.trim() || undefined,
    });
    keys.value = result.items.map(key => ({ ...key, is_visible: false }));
    total.value = result.pagination.total_items;
    totalPages.value = result.pagination.total_pages;

    // Reset virtual scroll position when data changes
    if (gridContainerRef.value) {
      gridContainerRef.value.scrollTop = 0;
    }
  } finally {
    loading.value = false;
  }
}

async function handleBatchDeleteSuccess() {
  await loadKeys();
  if (props.selectedGroup) {
    appState.triggerSyncOperationRefresh(props.selectedGroup.name, "BATCH_DELETE");
  }
}

async function copyKey(key: KeyRow) {
  await copyWithFeedback(key.key_value);
}

async function testKey(_key: KeyRow) {
  if (!props.selectedGroup?.id || !_key.key_value || testingMsg.value) {
    return;
  }

  testingMsg.value = window.$message.info(t("keys.testingKeyWithEllipsis"), {
    duration: 0,
  });

  try {
    const response = await keysApi.testKeys(props.selectedGroup.id, _key.key_value);
    const curValid = response.results?.[0] || {};
    if (curValid.is_valid) {
      window.$message.success(
        t("keys.testSuccess", { duration: formatDuration(response.total_duration) })
      );
    } else {
      window.$message.error(curValid.error || t("keys.testFailed"), {
        keepAliveOnHover: true,
        duration: 5000,
        closable: true,
      });
    }
    appState.triggerSyncOperationRefresh(props.selectedGroup.name, "TEST_SINGLE");
    await nextTick();
    await loadKeys();
  } catch (_error) {
    console.error("Test failed");
  } finally {
    testingMsg.value?.destroy();
    testingMsg.value = null;
  }
}

function toggleKeyVisibility(key: KeyRow) {
  const index = keys.value.findIndex(k => k.id === key.id);
  if (index !== -1) {
    keys.value[index] = { ...keys.value[index], is_visible: !keys.value[index].is_visible };
  }
}

function editKeyNotes(key: KeyRow) {
  editingKey.value = key;
  editingNotes.value = key.notes || "";
  notesDialogShow.value = true;
}

async function saveKeyNotes() {
  if (!editingKey.value) {
    return;
  }

  try {
    const trimmed = editingNotes.value.trim();
    await keysApi.updateKeyNotes(editingKey.value.id, trimmed);
    editingKey.value.notes = trimmed;
    window.$message.success(t("keys.notesUpdated"));
    notesDialogShow.value = false;
  } catch (error) {
    console.error("Update notes failed", error);
  }
}

async function restoreKey(key: KeyRow) {
  if (!props.selectedGroup?.id || !key.key_value || isRestoring.value) {
    return;
  }

  const d = dialog.warning({
    title: t("keys.restoreKey"),
    content: t("keys.confirmRestoreKey", { key: maskKey(key.key_value) }),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      isRestoring.value = true;
      d.loading = true;

      try {
        await keysApi.restoreKeys(props.selectedGroup.id, key.key_value);
        await loadKeys();
        appState.triggerSyncOperationRefresh(props.selectedGroup.name, "RESTORE_SINGLE");
      } catch (_error) {
        console.error("Restore failed");
      } finally {
        d.loading = false;
        isRestoring.value = false;
      }
    },
  });
}

async function deleteKey(key: KeyRow) {
  if (!props.selectedGroup?.id || !key.key_value || isDeling.value) {
    return;
  }

  const d = dialog.warning({
    title: t("keys.deleteKey"),
    content: t("keys.confirmDeleteKey", { key: maskKey(key.key_value) }),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      d.loading = true;
      isDeling.value = true;

      try {
        await keysApi.deleteKeys(props.selectedGroup.id, key.key_value);
        await loadKeys();
        appState.triggerSyncOperationRefresh(props.selectedGroup.name, "DELETE_SINGLE");
      } catch (_error) {
        console.error("Delete failed");
      } finally {
        d.loading = false;
        isDeling.value = false;
      }
    },
  });
}

async function copyAllKeys() {
  if (!props.selectedGroup?.id) {
    return;
  }

  keysApi.exportKeys(props.selectedGroup.id, "all");
}

async function copyValidKeys() {
  if (!props.selectedGroup?.id) {
    return;
  }

  keysApi.exportKeys(props.selectedGroup.id, "active");
}

async function copyInvalidKeys() {
  if (!props.selectedGroup?.id) {
    return;
  }

  keysApi.exportKeys(props.selectedGroup.id, "invalid");
}

async function restoreAllInvalid() {
  if (!props.selectedGroup?.id || isRestoring.value) {
    return;
  }

  const d = dialog.warning({
    title: t("keys.restoreKeys"),
    content: t("keys.confirmRestoreAllInvalid"),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      isRestoring.value = true;
      d.loading = true;
      try {
        await keysApi.restoreAllInvalidKeys(props.selectedGroup.id);
        await loadKeys();
        appState.triggerSyncOperationRefresh(props.selectedGroup.name, "RESTORE_ALL_INVALID");
      } catch (_error) {
        console.error("Restore failed");
      } finally {
        d.loading = false;
        isRestoring.value = false;
      }
    },
  });
}

async function validateKeys(status: "all" | "active" | "invalid") {
  if (!props.selectedGroup?.id || testingMsg.value) {
    return;
  }

  let statusText = t("common.all");
  if (status === "active") {
    statusText = t("keys.valid");
  } else if (status === "invalid") {
    statusText = t("keys.invalid");
  }

  testingMsg.value = window.$message.info(t("keys.validatingKeysMsg", { type: statusText }), {
    duration: 0,
  });

  try {
    await keysApi.validateGroupKeys(props.selectedGroup.id, status === "all" ? undefined : status);
    localStorage.removeItem("last_closed_task");
    appState.triggerTaskPolling();
  } catch (_error) {
    console.error("Test failed");
  } finally {
    testingMsg.value?.destroy();
    testingMsg.value = null;
  }
}

async function clearAllInvalid() {
  if (!props.selectedGroup?.id || isDeling.value) {
    return;
  }

  const d = dialog.warning({
    title: t("keys.clearKeys"),
    content: t("keys.confirmClearInvalidKeys"),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      isDeling.value = true;
      d.loading = true;
      try {
        await keysApi.clearAllInvalidKeys(props.selectedGroup.id);
        window.$message.success(t("keys.clearSuccess"));
        await loadKeys();
        appState.triggerSyncOperationRefresh(props.selectedGroup.name, "CLEAR_ALL_INVALID");
      } catch (_error) {
        console.error("Delete failed");
      } finally {
        d.loading = false;
        isDeling.value = false;
      }
    },
  });
}

async function clearAll() {
  if (!props.selectedGroup?.id || isDeling.value) {
    return;
  }

  dialog.warning({
    title: t("keys.clearAllKeys"),
    content: t("keys.confirmClearAllKeys"),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: () => {
      confirmInput.value = "";
      dialog.create({
        title: t("keys.enterGroupNameToConfirm"),
        content: () =>
          h("div", null, [
            h("p", null, [
              t("keys.dangerousOperationWarning1"),
              h("strong", null, t("common.all")),
              t("keys.dangerousOperationWarning2"),
              h("strong", { style: { color: "var(--error-color)" } }, props.selectedGroup?.name),
              t("keys.toConfirm"),
            ]),
            h(NInput, {
              value: confirmInput.value,
              "onUpdate:value": v => {
                confirmInput.value = v;
              },
              placeholder: t("keys.enterGroupName"),
            }),
          ]),
        positiveText: t("keys.confirmClear"),
        negativeText: t("common.cancel"),
        onPositiveClick: async () => {
          if (confirmInput.value !== props.selectedGroup?.name) {
            window.$message.error(t("keys.incorrectGroupName"));
            return false;
          }

          if (!props.selectedGroup?.id) {
            return;
          }

          isDeling.value = true;
          try {
            await keysApi.clearAllKeys(props.selectedGroup.id);
            window.$message.success(t("keys.clearAllKeysSuccess"));
            await loadKeys();
            appState.triggerSyncOperationRefresh(props.selectedGroup.name, "CLEAR_ALL");
          } catch (_error) {
            console.error("Clear all failed", _error);
          } finally {
            isDeling.value = false;
          }
        },
      });
    },
  });
}

function resetPage() {
  currentPage.value = 1;
  searchText.value = "";
  statusFilter.value = "all";
}
</script>

<template>
  <div class="key-table-container" ref="gridContainerRef">
    <key-toolbar
      :loading="loading"
      v-model:status-filter="statusFilter"
      v-model:search-text="searchText"
      @handle-search="handleSearchInput"
      @create-click="createDialogShow = true"
      @delete-click="deleteDialogShow = true"
      @more-action="handleMoreAction"
    />

    <div class="keys-grid-container">
      <n-spin :show="loading">
        <div v-if="keys.length === 0 && !loading" class="empty-container">
          <n-empty :description="t('keys.noMatchingKeys')" />
        </div>

        <!-- Virtual scrolling for large datasets -->
        <div
          v-else-if="shouldUseVirtualScroll"
          v-bind="containerProps"
          class="virtual-grid-container"
        >
          <div v-bind="wrapperProps" class="virtual-grid-wrapper">
            <div class="keys-grid keys-grid-virtual">
              <key-card
                v-for="item in virtualList"
                :key="item.data.id"
                :key-data="item.data"
                @edit-notes="editKeyNotes"
                @toggle-visibility="toggleKeyVisibility"
                @copy="copyKey"
                @test="testKey"
                @restore="restoreKey"
                @delete="deleteKey"
              />
            </div>
          </div>
        </div>

        <!-- Regular grid for small datasets -->
        <div v-else class="keys-grid">
          <key-card
            v-for="key in keys"
            :key="key.id"
            :key-data="key"
            @edit-notes="editKeyNotes"
            @toggle-visibility="toggleKeyVisibility"
            @copy="copyKey"
            @test="testKey"
            @restore="restoreKey"
            @delete="deleteKey"
          />
        </div>
      </n-spin>
    </div>

    <key-pagination
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      :total-pages="totalPages"
      :total="total"
    />

    <key-create-dialog
      v-if="selectedGroup?.id"
      v-model:show="createDialogShow"
      :group-id="selectedGroup.id"
      :group-name="getGroupDisplayName(selectedGroup!)"
      @success="loadKeys"
    />

    <key-delete-dialog
      v-if="selectedGroup?.id"
      v-model:show="deleteDialogShow"
      :group-id="selectedGroup.id"
      :group-name="getGroupDisplayName(selectedGroup!)"
      @success="handleBatchDeleteSuccess"
    />
  </div>

  <n-modal v-model:show="notesDialogShow" preset="dialog" :title="t('keys.editKeyNotes')">
    <n-input
      v-model:value="editingNotes"
      type="textarea"
      :placeholder="t('keys.enterNotes')"
      :rows="3"
      maxlength="255"
      show-count
    />
    <template #action>
      <n-button @click="notesDialogShow = false" class="btn-cancel">
        {{ t("common.cancel") }}
      </n-button>
      <n-button @click="saveKeyNotes" class="btn-confirm">
        {{ t("common.save") }}
      </n-button>
    </template>
  </n-modal>
</template>

<style scoped>
.key-table-container {
  background: var(--card-bg-solid);
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  overflow: hidden;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.keys-grid-container {
  flex: 1;
  overflow: hidden;
  padding: 16px;
  display: flex;
  flex-direction: column;
}

.keys-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

/* Virtual grid container */
.virtual-grid-container {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px;
}

.virtual-grid-wrapper {
  position: relative;
  width: 100%;
}

.keys-grid-virtual {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  padding: 8px;
}

.empty-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 200px;
}
</style>
