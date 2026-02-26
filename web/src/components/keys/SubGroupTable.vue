<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group, SubGroupInfo } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import {
  Add,
  ChevronDownOutline,
  ChevronUpOutline,
  CreateOutline,
  EyeOutline,
  InformationCircleOutline,
  Search,
  Trash,
} from "@vicons/ionicons5";
import {
  NButton,
  NButtonGroup,
  NEmpty,
  NIcon,
  NInput,
  NSelect,
  NSpin,
  NTag,
  NTooltip,
  useDialog,
} from "naive-ui";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import AddSubGroupModal from "./AddSubGroupModal.vue";
import EditSubGroupWeightModal from "./EditSubGroupWeightModal.vue";

const { t } = useI18n();

// Get sub-group status
function getSubGroupStatus(subGroup: SubGroupInfo): {
  status: "active" | "disabled" | "unavailable";
  text: string;
  type: "success" | "warning" | "error";
} {
  if (subGroup.weight === 0) {
    return { status: "disabled", text: t("subGroups.statusDisabled"), type: "warning" };
  }
  if (subGroup.weight > 0 && subGroup.active_keys === 0) {
    return { status: "unavailable", text: t("subGroups.statusUnavailable"), type: "error" };
  }
  return { status: "active", text: t("subGroups.statusActive"), type: "success" };
}

interface SubGroupRow extends SubGroupInfo {
  percentage: number;
}

interface Props {
  selectedGroup: Group | null;
  subGroups?: SubGroupInfo[];
  groups?: Group[];
  loading?: boolean;
}

interface Emits {
  (e: "refresh"): void;
  (e: "group-select", groupId: number): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const dialog = useDialog();

const addModalShow = ref(false);
const editModalShow = ref(false);
const editingSubGroup = ref<SubGroupInfo | null>(null);

// Search and filter state
const searchText = ref("");
const statusFilter = ref<"all" | "active" | "disabled" | "unavailable">("all");

// Collapse state - default collapsed
const isCollapsed = ref(true);

// Status filter options
const statusOptions = [
  { label: t("common.all"), value: "all" },
  { label: t("subGroups.statusActive"), value: "active" },
  { label: t("subGroups.statusDisabled"), value: "disabled" },
  { label: t("subGroups.statusUnavailable"), value: "unavailable" },
];

// Calculate sub-group data with percentage and sort by weight
const sortedSubGroupsWithPercentage = computed<SubGroupRow[]>(() => {
  if (!props.subGroups) {
    return [];
  }
  const total = props.subGroups.reduce((sum, sg) => sum + sg.weight, 0);
  const withPercentage = props.subGroups.map(sg => ({
    ...sg,
    percentage: total > 0 ? Math.round((sg.weight / total) * 100) : 0,
  }));

  // Sort by weight descending
  return withPercentage.sort((a, b) => b.weight - a.weight);
});

// Filtered sub-groups (apply search and status filtering)
const filteredSubGroups = computed<SubGroupRow[]>(() => {
  let filtered = sortedSubGroupsWithPercentage.value;

  // Name search filter (case-insensitive)
  if (searchText.value.trim()) {
    const searchLower = searchText.value.trim().toLowerCase();
    filtered = filtered.filter(sg => {
      const name = sg.group.name?.toLowerCase() || "";
      const displayName = sg.group.display_name?.toLowerCase() || "";
      return name.includes(searchLower) || displayName.includes(searchLower);
    });
  }

  // Status filter
  if (statusFilter.value !== "all") {
    filtered = filtered.filter(sg => {
      const status = getSubGroupStatus(sg).status;
      return status === statusFilter.value;
    });
  }

  return filtered;
});

function openEditModal(subGroup: SubGroupInfo) {
  editingSubGroup.value = subGroup;
  editModalShow.value = true;
}

async function deleteSubGroup(subGroup: SubGroupInfo) {
  if (!props.selectedGroup?.id) {
    return;
  }

  const d = dialog.warning({
    title: t("subGroups.removeSubGroup"),
    content: t("subGroups.confirmRemoveSubGroup", { name: getGroupDisplayName(subGroup) }),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      d.loading = true;
      try {
        const groupId = subGroup.group.id;
        if (!groupId) {
          return;
        }
        await keysApi.deleteSubGroup(props.selectedGroup.id, groupId);
        emit("refresh");
      } finally {
        d.loading = false;
      }
    },
  });
}

// Handle success after modal operations
function handleSuccess() {
  emit("refresh");
}

// Navigate to group info
function goToGroupInfo(groupId: number) {
  emit("group-select", groupId);
}

// Format number with K suffix
function formatNumber(num: number): string {
  if (num >= 1000) {
    return `${(num / 1000).toFixed(1)}K`;
  }
  return num.toString();
}

// Toggle collapse state
function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value;
}
</script>

<template>
  <div class="key-table-container">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <n-button class="btn-create" size="small" @click="addModalShow = true">
          <template #icon>
            <n-icon :component="Add" />
          </template>
          {{ t("subGroups.addSubGroup") }}
        </n-button>
      </div>
      <div class="toolbar-right">
        <n-select
          v-model:value="statusFilter"
          :options="statusOptions"
          size="small"
          style="width: 120px"
          :placeholder="t('keys.allStatus')"
        />
        <n-input
          v-model:value="searchText"
          :placeholder="t('keys.searchByName')"
          size="small"
          style="width: 200px"
          clearable
        >
          <template #prefix>
            <n-icon :component="Search" />
          </template>
        </n-input>
      </div>
    </div>

    <!-- Sub-group card grid -->
    <div class="keys-grid-container" v-show="!isCollapsed">
      <n-spin :show="props.loading || false">
        <div v-if="!props.subGroups || props.subGroups.length === 0" class="empty-container">
          <n-empty :description="t('subGroups.noSubGroups')" />
        </div>
        <div v-else-if="filteredSubGroups.length === 0" class="empty-container">
          <n-empty :description="t('keys.noMatchingKeys')" />
        </div>
        <div v-else class="keys-grid">
          <div
            v-for="subGroup in filteredSubGroups"
            :key="subGroup.group.id"
            class="key-card status-sub-group"
            :class="{ disabled: subGroup.weight === 0 || subGroup.active_keys === 0 }"
          >
            <!-- Main info row: display name + group name -->
            <div class="key-main">
              <div class="key-section">
                <div class="sub-group-names">
                  <span class="display-name">{{ getGroupDisplayName(subGroup) }}</span>
                </div>
                <div class="quick-actions">
                  <span class="group-name">#{{ subGroup.group.name }}</span>
                </div>
              </div>
            </div>

            <!-- Weight display -->
            <div class="weight-display">
              <div class="weight-bar-container">
                <span class="weight-label">
                  {{ t("subGroups.weight") }}
                  <strong>{{ subGroup.weight }}</strong>
                </span>
                <div class="weight-bar">
                  <div
                    class="weight-fill"
                    :class="{
                      'weight-fill-active': subGroup.weight > 0 && subGroup.active_keys > 0,
                      'weight-fill-unavailable': subGroup.weight > 0 && subGroup.active_keys === 0,
                    }"
                    :style="{ width: `${subGroup.percentage}%` }"
                  />
                </div>
                <span class="weight-text">{{ subGroup.percentage }}%</span>
              </div>
            </div>

            <!-- Key statistics -->
            <div class="key-stats-row">
              <div class="stats-left">
                <span class="stat-item">
                  <span class="stat-value">{{ formatNumber(subGroup.total_keys) }}</span>
                </span>
                <n-divider vertical />
                <span class="stat-item stat-success">
                  {{ formatNumber(subGroup.active_keys) }}
                </span>
                <n-divider vertical />
                <span class="stat-item stat-error">
                  {{ formatNumber(subGroup.invalid_keys) }}
                </span>
              </div>
              <n-tag :type="getSubGroupStatus(subGroup).type" size="small">
                {{ getSubGroupStatus(subGroup).text }}
              </n-tag>
            </div>

            <!-- Action buttons row -->
            <div class="key-bottom">
              <div class="key-stats">
                <n-tooltip trigger="hover" placement="right">
                  <template #trigger>
                    <n-button
                      round
                      tertiary
                      type="default"
                      size="tiny"
                      :aria-label="t('common.viewDetails')"
                    >
                      <template #icon>
                        <n-icon :component="InformationCircleOutline" />
                      </template>
                    </n-button>
                  </template>
                  <div class="sub-group-info-tooltip">
                    <!-- Group name and status -->
                    <div class="info-header">
                      <div class="info-title">{{ getGroupDisplayName(subGroup) }}</div>
                      <n-tag :type="getSubGroupStatus(subGroup).type" size="small">
                        {{ getSubGroupStatus(subGroup).text }}
                      </n-tag>
                    </div>

                    <!-- Detailed information -->
                    <div class="info-details">
                      <div class="info-row">
                        <span class="info-label">{{ t("keys.testModel") }}:</span>
                        <span class="info-value">{{ subGroup.group.test_model || "-" }}</span>
                      </div>
                      <div class="info-row" v-if="subGroup.group.channel_type !== 'gemini'">
                        <span class="info-label">{{ t("keys.testPath") }}:</span>
                        <span class="info-value">
                          {{ subGroup.group.validation_endpoint || "-" }}
                        </span>
                      </div>

                      <!-- Upstream addresses -->
                      <div
                        class="info-row"
                        v-if="subGroup.group.upstreams && subGroup.group.upstreams.length > 0"
                      >
                        <span class="info-label">{{ t("keys.upstreamAddresses") }}:</span>
                        <div class="info-value upstream-list">
                          <input
                            v-for="(upstream, index) in subGroup.group.upstreams"
                            :key="index"
                            class="upstream-input"
                            :value="upstream.url"
                            readonly
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </n-tooltip>
              </div>
              <n-button-group class="key-actions">
                <n-button
                  round
                  tertiary
                  class="btn-view"
                  size="tiny"
                  @click="subGroup.group.id && goToGroupInfo(subGroup.group.id)"
                  :title="t('subGroups.viewSubGroup')"
                >
                  <template #icon>
                    <n-icon :component="EyeOutline" />
                  </template>
                  {{ t("common.view") }}
                </n-button>
                <n-button
                  round
                  tertiary
                  class="btn-edit"
                  size="tiny"
                  @click="openEditModal(subGroup)"
                  :title="t('subGroups.editWeight')"
                >
                  <template #icon>
                    <n-icon :component="CreateOutline" />
                  </template>
                  {{ t("common.edit") }}
                </n-button>
                <n-button
                  round
                  tertiary
                  class="btn-delete"
                  size="tiny"
                  @click="deleteSubGroup(subGroup)"
                  :title="t('subGroups.removeSubGroup')"
                >
                  <template #icon>
                    <n-icon :component="Trash" />
                  </template>
                  {{ t("subGroups.remove") }}
                </n-button>
              </n-button-group>
            </div>
          </div>
        </div>
      </n-spin>
    </div>

    <!-- Footer information -->
    <div class="pagination-container">
      <div class="pagination-info">
        <span>
          {{ t("subGroups.totalSubGroups", { count: filteredSubGroups.length }) }}
          <template v-if="filteredSubGroups.length !== (props.subGroups?.length || 0)">
            / {{ props.subGroups?.length || 0 }}
          </template>
        </span>
        <n-icon
          :component="isCollapsed ? ChevronDownOutline : ChevronUpOutline"
          size="18"
          style="cursor: pointer; color: var(--text-secondary); margin-left: 8px"
          @click="toggleCollapse"
        />
      </div>
      <div class="pagination-controls">
        <span class="page-info">
          {{ t("subGroups.sortedByWeight") }}
        </span>
      </div>
    </div>

    <!-- Add sub-group modal -->
    <add-sub-group-modal
      v-if="selectedGroup?.id"
      v-model:show="addModalShow"
      :aggregate-group="selectedGroup"
      :existing-sub-groups="subGroups || []"
      :groups="groups || []"
      @success="handleSuccess"
    />

    <!-- Edit weight modal -->
    <edit-sub-group-weight-modal
      v-if="editingSubGroup && selectedGroup?.id"
      v-model:show="editModalShow"
      :aggregate-group="selectedGroup"
      :sub-group="editingSubGroup"
      :sub-groups="subGroups || []"
      @success="handleSuccess"
      @update:show="
        show => {
          if (!show) editingSubGroup = null;
        }
      "
    />
  </div>
</template>

<style scoped>
/* Component-specific styles only - shared styles in components.css */

/* Sub-group specific - group name tag style override */
.group-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--sub-group-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  background: var(--sub-group-bg);
  padding: 2px 6px;
  border-radius: 4px;
  white-space: nowrap;
  flex-shrink: 0;
}

/* Key stats row - component specific layout */
.key-stats-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.stat-label {
  color: var(--text-secondary);
}

.stat-value {
  color: var(--text-primary);
  font-weight: 500;
}

.stat-divider {
  color: var(--text-secondary);
  opacity: 0.5;
}

/* Disabled state styles */
.key-card.disabled .display-name,
.key-card.disabled .group-name,
.key-card.disabled .weight-label {
  color: var(--text-disabled);
}

.key-card.disabled .weight-fill {
  background: var(--color-disabled);
}

/* Tooltip override */
.sub-group-info-tooltip {
  min-width: 450px;
}
</style>
