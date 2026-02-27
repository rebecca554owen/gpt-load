<script setup lang="ts">
import type {
  Group,
  GroupConfigOption,
  GroupStatsResponse,
  ParentAggregateGroup,
  SubGroupInfo,
} from "@/types/models";
import { keysApi } from "@/api/keys";
import { copy } from "@/utils/clipboard";
import { useAppStateStore } from "@/stores/appState";
import { getGroupDisplayName } from "@/utils/display";
import { CopyOutline, Pencil, Trash } from "@vicons/ionicons5";
import { NButton, NCard, NDivider, NIcon, NInput, NTooltip, useDialog } from "naive-ui";
import { computed, h, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import AggregateGroupModal from "./AggregateGroupModal.vue";
import GroupCopyModal from "./GroupCopyModal.vue";
import GroupDetailsSection from "./GroupDetailsSection.vue";
import GroupFormModal from "./GroupFormModal.vue";
import GroupStatsSection from "./GroupStatsSection.vue";

const { t } = useI18n();
const appState = useAppStateStore();

interface Props {
  group: Group | null;
  groups?: Group[];
  subGroups?: SubGroupInfo[];
}

interface Emits {
  (e: "refresh", value: Group): void;
  (e: "delete", value: Group): void;
  (e: "copy-success", group: Group): void;
  (e: "navigate-to-group", groupId: number): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const stats = ref<GroupStatsResponse | null>(null);
const loading = ref(false);
const dialog = useDialog();
const showEditModal = ref(false);
const showCopyModal = ref(false);
const showAggregateEditModal = ref(false);
const delLoading = ref(false);
const confirmInput = ref("");
const configOptions = ref<GroupConfigOption[]>([]);
const parentAggregateGroups = ref<ParentAggregateGroup[]>([]);

const isAggregateGroup = computed(() => props.group?.group_type === "aggregate");

onMounted(() => {
  loadStats();
  loadConfigOptions();
  loadParentAggregateGroups();
});

watch(
  () => props.group,
  () => {
    resetPage();
    loadStats();
    loadParentAggregateGroups();
  }
);

watch(
  () => appState.groupDataRefreshTrigger,
  () => {
    if (!props.group) {
      return;
    }
    const isCurrentGroupTask =
      appState.lastCompletedTask && appState.lastCompletedTask.groupName === props.group.name;
    const isRelevantTaskType =
      isCurrentGroupTask &&
      ["KEY_VALIDATION", "KEY_IMPORT", "KEY_DELETE"].includes(
        appState.lastCompletedTask?.taskType || ""
      );
    if (isRelevantTaskType) {
      loadStats();
    }
  }
);

watch(
  () => appState.syncOperationTrigger,
  () => {
    if (!props.group) {
      return;
    }
    const isCurrentGroupSync =
      appState.lastSyncOperation && appState.lastSyncOperation.groupName === props.group.name;
    if (isCurrentGroupSync) {
      loadStats();
    }
  }
);

async function loadStats() {
  if (!props.group?.id) {
    stats.value = null;
    return;
  }
  try {
    loading.value = true;
    stats.value = await keysApi.getGroupStats(props.group.id);
  } finally {
    loading.value = false;
  }
}

async function loadConfigOptions() {
  try {
    configOptions.value = (await keysApi.getGroupConfigOptions()) || [];
  } catch (error) {
    console.error("Failed to load config options:", error);
  }
}

async function loadParentAggregateGroups() {
  if (!props.group?.id || props.group.group_type === "aggregate") {
    parentAggregateGroups.value = [];
    return;
  }
  try {
    parentAggregateGroups.value = (await keysApi.getParentAggregateGroups(props.group.id)) || [];
  } catch (error) {
    console.error("Failed to load parent aggregate groups:", error);
    parentAggregateGroups.value = [];
  }
}

function handleEdit() {
  if (!props.group) {
    return;
  }
  if (props.group.group_type === "aggregate") {
    showAggregateEditModal.value = true;
    return;
  }
  showEditModal.value = true;
}

function handleCopy() {
  showCopyModal.value = true;
}

function handleNavigateToGroup(groupId: number) {
  emit("navigate-to-group", groupId);
}

function handleGroupEdited(newGroup: Group) {
  showEditModal.value = false;
  if (newGroup) {
    emit("refresh", newGroup);
  }
}

function handleAggregateGroupEdited(newGroup: Group) {
  showAggregateEditModal.value = false;
  if (newGroup) {
    emit("refresh", newGroup);
  }
}

function handleGroupCopied(newGroup: Group) {
  showCopyModal.value = false;
  if (newGroup) {
    emit("copy-success", newGroup);
  }
}

async function handleDelete() {
  if (!props.group || delLoading.value) {
    return;
  }

  dialog.warning({
    title: t("keys.deleteGroup"),
    content: t("keys.confirmDeleteGroup", { name: getGroupDisplayName(props.group) }),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: () => {
      confirmInput.value = "";
      dialog.create({
        title: t("keys.enterGroupNameToConfirm"),
        content: () =>
          h("div", null, [
            h("p", null, [
              t("keys.dangerousOperation"),
              h("strong", { style: { color: "#d03050" } }, props.group?.name),
              t("keys.toConfirmDeletion"),
            ]),
            h(NInput, {
              value: confirmInput.value,
              "onUpdate:value": (v: string) => {
                confirmInput.value = v;
              },
              placeholder: t("keys.enterGroupName"),
            }),
          ]),
        positiveText: t("keys.confirmDelete"),
        negativeText: t("common.cancel"),
        onPositiveClick: async () => {
          if (confirmInput.value !== props.group?.name) {
            window.$message.error(t("keys.incorrectGroupName"));
            return false;
          }
          delLoading.value = true;
          try {
            if (props.group?.id) {
              await keysApi.deleteGroup(props.group.id);
              emit("delete", props.group);
            }
          } finally {
            delLoading.value = false;
          }
        },
      });
    },
  });
}

async function copyUrl(url: string) {
  if (!url) {
    return;
  }
  const success = await copy(url);
  if (success) {
    window.$message.success(t("keys.urlCopied"));
  } else {
    window.$message.error(t("keys.copyFailed"));
  }
}

function resetPage() {
  showEditModal.value = false;
  showCopyModal.value = false;
}
</script>

<template>
  <div class="group-info-container">
    <n-card :bordered="false" class="group-info-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <h3 class="group-title">
              {{ group ? getGroupDisplayName(group) : t("keys.selectGroup") }}
              <n-tooltip trigger="hover" v-if="group && group.endpoint">
                <template #trigger>
                  <code class="group-url" @click="copyUrl(group.endpoint)">
                    {{ group.endpoint }}
                  </code>
                </template>
                {{ t("keys.clickToCopy") }}
              </n-tooltip>
            </h3>
          </div>
          <div class="header-actions">
            <n-button
              v-if="group?.group_type !== 'aggregate'"
              quaternary
              circle
              size="small"
              @click="handleCopy"
              :title="t('keys.copyGroup')"
              :disabled="!group"
            >
              <template #icon>
                <n-icon :component="CopyOutline" />
              </template>
            </n-button>
            <n-button
              quaternary
              circle
              size="small"
              @click="handleEdit"
              :title="t('keys.editGroup')"
            >
              <template #icon>
                <n-icon :component="Pencil" />
              </template>
            </n-button>
            <n-button
              quaternary
              circle
              size="small"
              @click="handleDelete"
              :title="t('keys.deleteGroup')"
              type="error"
              :disabled="!group"
            >
              <template #icon>
                <n-icon :component="Trash" />
              </template>
            </n-button>
          </div>
        </div>
      </template>

      <n-divider style="margin: 0; margin-bottom: 12px" />

      <group-stats-section
        :stats="stats"
        :loading="loading"
        :is-aggregate-group="isAggregateGroup"
        :sub-groups="props.subGroups"
      />

      <n-divider style="margin: 0" />

      <group-details-section
        :group="group"
        :config-options="configOptions"
        :parent-aggregate-groups="parentAggregateGroups"
        @navigate-to-group="handleNavigateToGroup"
      />
    </n-card>

    <group-form-modal v-model:show="showEditModal" :group="group" @success="handleGroupEdited" />
    <aggregate-group-modal
      v-model:show="showAggregateEditModal"
      :group="group"
      :groups="props.groups"
      @success="handleAggregateGroupEdited"
    />
    <group-copy-modal
      v-model:show="showCopyModal"
      :source-group="group"
      @success="handleGroupCopied"
    />
  </div>
</template>

<style scoped>
.group-info-container {
  width: 100%;
}

:deep(.n-card-header) {
  padding: 12px 24px;
}

.group-info-card {
  background: var(--card-bg-solid);
  border-radius: var(--border-radius-md);
  border: 1px solid var(--border-color);
  animation: fadeInUp 0.2s ease-out;
  box-shadow: var(--shadow-sm);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.header-left {
  flex: 1;
}

.group-title {
  font-size: 1.2rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-url {
  font-size: 0.8rem;
  color: var(--primary-color);
  margin-left: 8px;
  font-family: monospace;
  background: var(--bg-secondary);
  border-radius: 4px;
  padding: 2px 6px;
  margin-right: 4px;
  border: 1px solid var(--border-color);
  cursor: pointer;
  transition: all 0.2s ease;
}

.header-actions {
  display: flex;
  gap: 8px;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
