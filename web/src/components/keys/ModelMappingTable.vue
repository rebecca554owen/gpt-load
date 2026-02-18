<script setup lang="ts">
import { keysApi } from "@/api/keys";
import { useModelTestStatus } from "@/composables/useModelTestStatus";
import type { Group, ModelMapping, ModelMappingTarget, SubGroupInfo } from "@/types/models";
import { copy } from "@/utils/clipboard";
import { formatDuration } from "@/utils/format";
import {
  Add,
  CheckmarkCircle,
  CopyOutline,
  CreateOutline,
  InformationCircleOutline,
  PlayCircleOutline,
  Search,
  Trash,
} from "@vicons/ionicons5";
import {
  NButton,
  NButtonGroup,
  NEmpty,
  NIcon,
  NInput,
  NSpin,
  NTag,
  NTooltip,
  useDialog,
  useMessage,
} from "naive-ui";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import ModelMappingModal from "./ModelMappingModal.vue";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();

interface ModelMappingRow extends ModelMapping {
  totalWeight: number;
}

interface Props {
  selectedGroup: Group | null;
  modelMappings?: ModelMapping[];
  subGroups?: SubGroupInfo[];
  groups?: Group[];
  loading?: boolean;
}

interface Emits {
  (e: "refresh"): void;
  (e: "group-select", groupId: number): void;
  (e: "group-updated", group: Group): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const modalShow = ref(false);
const modalMode = ref<"add" | "edit">("add");
const editingModelMapping = ref<ModelMapping | null>(null);

// Search and filter state
const searchText = ref("");

// Test status tracking
const {
  clearAll,
  startTesting,
  setSuccess,
  setFailure,
  finishTesting,
  isTesting,
  hasFailed,
  hasSucceeded,
  getResult,
} = useModelTestStatus();

// Generate lightweight signature for detecting substantial changes
const getDataSignature = (mappings: typeof props.modelMappings, groups: typeof props.subGroups) => {
  if (!mappings || !groups) {
    return "";
  }
  return `${mappings.length}:${groups.length}:${mappings.map(m => m.model).join(",")}`;
};

// Watch for data changes, but don't clear test status too frequently
let lastSignature = "";
watch(
  [() => props.modelMappings, () => props.subGroups],
  () => {
    const signature = getDataSignature(props.modelMappings, props.subGroups);
    if (lastSignature && signature !== lastSignature) {
      clearAll();
    }
    lastSignature = signature;
  },
  { immediate: true }
);

// Compute model mapping data with total weight, sorted alphabetically
const sortedModelMappings = computed<ModelMappingRow[]>(() => {
  if (!props.modelMappings) {
    return [];
  }

  return props.modelMappings
    .map(mapping => {
      // Calculate valid weight (excluding sub-groups with no valid keys)
      const validWeight = mapping.targets.reduce((sum, target) => {
        const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
        // If sub-group has no valid keys, weight becomes zero
        if (subGroup && subGroup.active_keys > 0) {
          return sum + target.weight;
        }
        return sum;
      }, 0);

      return {
        ...mapping,
        totalWeight: validWeight,
      };
    })
    .sort((a, b) => a.model.localeCompare(b.model)); // Sort alphabetically by model name
});

// Filtered model mappings (apply search)
const filteredModelMappings = computed<ModelMappingRow[]>(() => {
  let filtered = sortedModelMappings.value;

  // Model alias search filter (case-insensitive)
  if (searchText.value.trim()) {
    const searchLower = searchText.value.trim().toLowerCase();
    filtered = filtered.filter(mapping => {
      return mapping.model.toLowerCase().includes(searchLower);
    });
  }

  return filtered;
});

function openAddModal() {
  editingModelMapping.value = null;
  modalMode.value = "add";
  modalShow.value = true;
}

function openEditModal(modelMapping: ModelMapping) {
  editingModelMapping.value = modelMapping;
  modalMode.value = "edit";
  // Use nextTick to ensure editingModelMapping is updated before showing modal
  nextTick(() => {
    modalShow.value = true;
  });
}

async function deleteModelMapping(modelMapping: ModelMapping) {
  if (!props.selectedGroup?.id) {
    return;
  }

  const d = dialog.warning({
    title: t("modelMappings.removeModelMapping"),
    content: t("modelMappings.confirmRemoveModelMapping", { model: modelMapping.model }),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      d.loading = true;
      try {
        // Remove specified model mapping
        const remainingMappings =
          props.modelMappings?.filter(m => m.model !== modelMapping.model) || [];

        await keysApi.updateGroup(props.selectedGroup.id, {
          model_mappings: remainingMappings,
        });

        message.success(t("common.operationSuccess"));
        emit("refresh");
      } catch (error) {
        console.error("Failed to delete model mapping:", error);
        message.error(t("common.operationFailed"));
      } finally {
        d.loading = false;
      }
    },
  });
}

// Handle success after modal operations
function handleSuccess() {
  emit("refresh");
  // Only clear test status when adding/editing model mapping, as model mapping may have changed
  clearAll();
}

// Get sub group name by ID
function getSubGroupName(subGroupId: number): string {
  const subGroup = props.subGroups?.find(sg => sg.group.id === subGroupId);
  if (subGroup) {
    return subGroup.group.display_name || subGroup.group.name;
  }

  // If not found in sub-groups, try to find from all groups
  const group = props.groups?.find(g => g.id === subGroupId);
  return group?.display_name || group?.name || `#${subGroupId}`;
}

// Calculate target percentage
function getTargetPercentage(target: ModelMappingTarget, mapping: ModelMappingRow): number {
  // Check if sub-group has valid keys
  const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
  if (!subGroup || subGroup.active_keys === 0) {
    return 0; // Sub-group has no valid keys, weight becomes zero
  }

  const totalWeight = mapping.totalWeight;
  if (totalWeight === 0) {
    return 0;
  }
  return Math.round((target.weight / totalWeight) * 100);
}

// Test sub group model availability using round-robin key selection
async function testSubGroupModel(target: ModelMappingTarget, mapping: ModelMapping) {
  const testKey = `${mapping.model}_${target.model}_${target.sub_group_id}`;

  // Prevent duplicate testing
  if (isTesting(testKey)) {
    return;
  }

  const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
  if (!subGroup || subGroup.active_keys === 0) {
    message.warning(
      t("modelMappings.subGroupNoAvailableKeys", { name: getSubGroupName(target.sub_group_id) })
    );
    return;
  }

  if (!props.selectedGroup?.id) {
    message.error(t("keys.selectGroup"));
    return;
  }

  try {
    // Mark as testing
    if (!startTesting(testKey)) {
      return;
    }

    // Create test loading state
    const loadingMessage = message.loading(
      t("modelMappings.testingModel", { model: target.model }),
      {
        duration: 0,
      }
    );

    // Test next key using round-robin mechanism
    const response = await keysApi.testNextKey(target.sub_group_id, target.model);
    const result = response.result;

    loadingMessage.destroy();

    if (result.is_valid) {
      // Mark as test success and store result
      setSuccess(testKey, response.total_duration || 0);

      message.success(
        t("modelMappings.testSuccess", {
          model: target.model,
          duration: formatDuration(response.total_duration || 0),
        })
      );
    } else {
      // Mark as test failure, remove success status and result
      setFailure(testKey);

      message.error(
        t("modelMappings.testFailed", {
          model: target.model,
          error: result.error || t("modelMappings.testFailedGeneric"),
        }),
        {
          keepAliveOnHover: true,
          duration: 5000,
          closable: true,
        }
      );
    }

    // Model testing does not need data refresh, as testing operation does not modify any data
    // Remove auto-refresh after test completion to avoid triggering global loading animation
  } catch (error) {
    console.error(t("modelMappings.testModelFailed"), error);
    // Mark as failure status
    setFailure(testKey);

    message.error(
      t("modelMappings.testFailed", {
        model: target.model,
        error: error instanceof Error ? error.message : t("modelMappings.unknownError"),
      })
    );
  } finally {
    // Remove test status regardless of success or failure
    finishTesting(testKey);
  }
}

// Get test button type (set color based on latency) - using natural color scheme
function getTestButtonType(testKey: string): "default" | "info" | "warning" | "success" | "error" {
  // 优先检查失败状态
  if (hasFailed(testKey)) {
    return "error";
  }

  const result = getResult(testKey);
  if (!result) {
    return "default";
  }

  const duration = result.duration;

  // Natural color scheme: forest green -> sky blue -> lemon yellow -> orange -> brown -> red -> purple -> gray
  // < 200ms: success (green) - extremely fast response
  // 200-500ms: info (blue) - very fast response
  // 500-1000ms: warning (yellow) - fast response
  // 1000-2000ms: default (orange) - normal response
  // 2000-3000ms: warning (brown) - slow response
  // 3000-4000ms: error (red) - very slow response
  // 4000-5000ms: error (purple) - extremely slow response
  // > 5000ms: default (gray) - timeout response
  if (duration < 200) {
    return "success";
  } // 森林绿
  if (duration < 500) {
    return "info";
  } // 天蓝
  if (duration < 1000) {
    return "warning";
  } // 柠檬黄
  if (duration < 2000) {
    return "default";
  } // 橙色
  if (duration < 3000) {
    return "warning";
  } // 棕色
  if (duration < 4000) {
    return "error";
  } // 红色
  if (duration < 5000) {
    return "error";
  } // 紫色
  return "default"; // 灰色
}

// Get custom CSS class for test button (for finer color control)
function getTestButtonClass(testKey: string): string {
  // 优先检查失败状态
  if (hasFailed(testKey)) {
    return "test-failed";
  }

  const result = getResult(testKey);
  if (!result) {
    return "";
  }

  const duration = result.duration;

  // Green gradient color scheme:
  // < 200ms: ultra fast - bright green (light tones)
  // 200-500ms: very fast - deep green
  // 500-1000ms: fast - yellow-green
  // 1000-2000ms: normal - golden yellow
  // 2000-3000ms: slow - orange-yellow
  // 3000-4000ms: very slow - orange
  // 4000-5000ms: extremely slow - red
  // > 5000ms: timeout - deep red
  if (duration < 200) {
    return "test-very-fast";
  }
  if (duration < 500) {
    return "test-excellent";
  }
  if (duration < 1000) {
    return "test-fast";
  }
  if (duration < 2000) {
    return "test-normal";
  }
  if (duration < 3000) {
    return "test-slow";
  }
  if (duration < 4000) {
    return "test-very-slow";
  }
  if (duration < 5000) {
    return "test-extremely-slow";
  }
  return "test-timeout";
}

// Get test button text (display specific duration or status)
function getTestButtonText(testKey: string): string {
  // If test failed, display "Failed"
  if (hasFailed(testKey)) {
    return t("modelMappings.failed");
  }

  const result = getResult(testKey);
  if (!result) {
    return t("modelMappings.test");
  }

  const duration = result.duration;
  const formattedDuration = formatDuration(duration);

  // Return corresponding duration based on latency
  return formattedDuration;
}

// Copy model alias function
async function copyModelAlias(modelAlias: string) {
  const success = await copy(modelAlias);

  if (success) {
    message.success(t("modelMappings.copySuccess", { alias: modelAlias }));
  } else {
    message.error(t("modelMappings.copyFailed"));
  }
}

// Generate unique model alias
function generateUniqueAlias(originalAlias: string): string {
  let counter = 1;
  let newAlias = `${originalAlias}-${counter}`;

  while (props.modelMappings?.some((m: ModelMapping) => m.model === newAlias)) {
    counter++;
    newAlias = `${originalAlias}-${counter}`;
  }

  return newAlias;
}

// Duplicate model mapping card function
async function duplicateModelMapping(mapping: ModelMapping) {
  if (!props.selectedGroup?.id) {
    message.error(t("keys.selectGroup"));
    return;
  }

  // Generate new model alias
  const newModelAlias = generateUniqueAlias(mapping.model);

  // Create duplicated mapping data
  const duplicatedMapping: ModelMapping = {
    model: newModelAlias,
    targets: mapping.targets.map(target => ({
      ...target,
      // Ensure creating new target object to avoid reference issues
    })),
  };

  // Set to add mode and pre-fill data
  editingModelMapping.value = duplicatedMapping;
  modalMode.value = "add";

  // Use nextTick to ensure data is updated before showing modal
  await nextTick();
  modalShow.value = true;

  message.info(t("modelMappings.duplicatedMappingInfo"));
}
</script>

<template>
  <div class="model-mapping-table-container">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <n-button
          class="btn-create"
          size="small"
          @click="openAddModal"
          :disabled="!subGroups || subGroups.length === 0"
        >
          <template #icon>
            <n-icon :component="Add" />
          </template>
          {{ t("modelMappings.addModelMapping") }}
        </n-button>
      </div>
      <div class="toolbar-right">
        <n-input
          v-model:value="searchText"
          :placeholder="t('modelMappings.modelAlias')"
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

    <!-- Model mapping card grid -->
    <div class="model-mappings-grid-container">
      <n-spin :show="props.loading || false">
        <div
          v-if="!props.modelMappings || props.modelMappings.length === 0"
          class="empty-container"
        >
          <n-empty :description="t('modelMappings.noModelMappings')" />
        </div>
        <div v-else-if="filteredModelMappings.length === 0" class="empty-container">
          <n-empty :description="t('keys.noMatchingKeys')" />
        </div>
        <div v-else class="model-mappings-grid">
          <div
            v-for="mapping in filteredModelMappings"
            :key="mapping.model"
            class="key-card status-model-mapping"
          >
            <!-- Main title row -->
            <div class="key-main">
              <div class="key-section">
                <div class="quick-actions">
                  <n-tag type="info" size="small">模型别名</n-tag>
                </div>
                <div class="model-mapping-names">
                  <n-tooltip trigger="hover">
                    <template #trigger>
                      <code class="group-url" @click="copyModelAlias(mapping.model)">
                        {{ mapping.model }}
                      </code>
                    </template>
                    {{ t("keys.clickToCopy") }}
                  </n-tooltip>
                </div>
              </div>
            </div>

            <!-- Sub-group progress bar display (show all sub-groups) -->
            <div class="weight-display">
              <div class="subgroup-progress-bars">
                <div
                  v-for="(target, index) in mapping.targets"
                  :key="index"
                  class="target-progress-item"
                >
                  <!-- Model name row -->
                  <div class="target-model-row">
                    <div class="model-name-chain">
                      <span class="subgroup-part">{{ getSubGroupName(target.sub_group_id) }}</span>
                      <span class="arrow-separator">→</span>
                      <span class="model-part">{{ target.model }}</span>
                    </div>
                    <n-button
                      size="tiny"
                      :type="
                        getTestButtonType(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                      "
                      :class="[
                        'target-test-button',
                        getTestButtonClass(
                          `${mapping.model}_${target.model}_${target.sub_group_id}`
                        ),
                      ]"
                      ghost
                      @click="testSubGroupModel(target, mapping)"
                      :disabled="
                        (subGroups?.find(sg => sg.group.id === target.sub_group_id)?.active_keys ||
                          0) === 0 ||
                        isTesting(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                      "
                      :loading="
                        isTesting(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                      "
                    >
                      <template
                        #icon
                        v-if="!isTesting(`${mapping.model}_${target.model}_${target.sub_group_id}`)"
                      >
                        <n-icon
                          :component="
                            hasSucceeded(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                              ? CheckmarkCircle
                              : PlayCircleOutline
                          "
                        />
                      </template>
                      {{
                        isTesting(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                          ? t("modelMappings.testInProgress")
                          : getTestButtonText(
                              `${mapping.model}_${target.model}_${target.sub_group_id}`
                            )
                      }}
                    </n-button>
                  </div>
                  <!-- Progress bar row -->
                  <div class="target-progress-bar">
                    <div
                      class="weight-fill"
                      :class="{
                        'weight-fill-active':
                          (subGroups?.find(sg => sg.group.id === target.sub_group_id)
                            ?.active_keys || 0) > 0,
                        'weight-fill-unavailable':
                          (subGroups?.find(sg => sg.group.id === target.sub_group_id)
                            ?.active_keys || 0) === 0,
                      }"
                      :style="{ width: `${Math.min(getTargetPercentage(target, mapping), 100)}%` }"
                    />
                  </div>
                </div>
              </div>
            </div>

            <!-- Action buttons row -->
            <div class="key-bottom">
              <!-- Left floating view details button -->
              <n-tooltip trigger="hover" placement="top">
                <template #trigger>
                  <n-button round tertiary type="default" size="tiny">
                    <template #icon>
                      <n-icon :component="InformationCircleOutline" />
                    </template>
                  </n-button>
                </template>
                <div class="model-mapping-info-tooltip">
                  <!-- Model mapping details -->
                  <div class="info-header">
                    <div class="info-title">{{ mapping.model }}</div>
                    <n-tag type="info" size="small">模型映射</n-tag>
                  </div>

                  <!-- Detailed information -->
                  <div class="info-details">
                    <div class="info-row">
                      <span class="info-label">总权重:</span>
                      <span class="info-value">{{ mapping.totalWeight }}</span>
                    </div>
                    <div class="info-row">
                      <span class="info-label">目标数量:</span>
                      <span class="info-value">{{ mapping.targets.length }}</span>
                    </div>

                    <!-- Detailed information for each target -->
                    <div
                      v-for="(target, index) in mapping.targets"
                      :key="index"
                      class="target-detail"
                    >
                      <div class="info-row">
                        <span class="info-label">子分组 {{ index + 1 }}:</span>
                        <span class="info-value">{{ getSubGroupName(target.sub_group_id) }}</span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">目标模型:</span>
                        <span class="info-value target-model-detail">{{ target.model }}</span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">权重:</span>
                        <span class="info-value">
                          {{ target.weight }} ({{ getTargetPercentage(target, mapping) }}%)
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </n-tooltip>

              <!-- Right edit, copy and delete button group -->
              <n-button-group class="key-actions">
                <n-button
                  round
                  tertiary
                  class="btn-edit"
                  size="tiny"
                  @click="openEditModal(mapping)"
                  :title="t('modelMappings.editModelMapping')"
                >
                  <template #icon>
                    <n-icon :component="CreateOutline" />
                  </template>
                  {{ t("common.edit") }}
                </n-button>
                <n-button
                  round
                  tertiary
                  class="btn-view"
                  size="tiny"
                  @click="duplicateModelMapping(mapping)"
                  :title="t('modelMappings.duplicateModelMapping')"
                >
                  <template #icon>
                    <n-icon :component="CopyOutline" />
                  </template>
                  {{ t("modelMappings.duplicate") }}
                </n-button>
                <n-button
                  round
                  tertiary
                  class="btn-delete"
                  size="tiny"
                  @click="deleteModelMapping(mapping)"
                  :title="t('modelMappings.removeModelMapping')"
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
          {{ t("modelMappings.totalMappings", { total: filteredModelMappings.length }) }}
          <template v-if="filteredModelMappings.length !== (props.modelMappings?.length || 0)">
            / {{ props.modelMappings?.length || 0 }}
          </template>
        </span>
      </div>
      <div class="pagination-controls">
        <span class="page-info">{{ t("modelMappings.sortedByModelName") }}</span>
      </div>
    </div>

    <!-- Unified model mapping modal -->
    <model-mapping-modal
      v-if="selectedGroup?.id"
      v-model:show="modalShow"
      :mode="modalMode"
      :aggregate-group="selectedGroup"
      :model-mapping="editingModelMapping"
      :existing-model-mappings="modelMappings || []"
      :sub-groups="subGroups || []"
      :groups="groups || []"
      @success="handleSuccess"
      @group-updated="group => emit('group-updated', group)"
      @update:show="
        show => {
          if (!show) editingModelMapping = null;
        }
      "
    />
  </div>
</template>

<style scoped>
.model-mapping-table-container {
  background: var(--card-bg-solid);
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  overflow: hidden;
  min-height: 100%;
  height: auto;
  display: flex;
  flex-direction: column;
  contain: layout;
}

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

.model-mappings-grid-container {
  flex: 1;
  /* Let container expand naturally based on card height */
  overflow-y: visible; /* Changed to visible, don't limit content */
  overflow-x: hidden; /* Prevent horizontal overflow */
  padding: 16px;
  min-height: 0;
  /* Ensure container can grow with content */
  height: auto;
  /* Allow container to grow within parent */
  max-height: none;
}

.model-mappings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  /* Ensure grid handles cards with different heights correctly */
  grid-auto-rows: min-content;
  /* Let grid grow naturally */
  align-items: start;
  /* Ensure content is not truncated */
  overflow: visible;
}

.key-card {
  /* Liquid glass style */
  background: rgba(250, 252, 255, 0.75);
  backdrop-filter: blur(20px) saturate(150%);
  -webkit-backdrop-filter: blur(20px) saturate(150%);
  border: 1px solid rgba(255, 255, 255, 0.45);
  border-radius: var(--radius-md, 12px);
  padding: 16px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.3),
    0 2px 8px rgba(0, 0, 0, 0.04),
    inset 0 1px 0 rgba(255, 255, 255, 0.4);
  width: 100%;
  min-height: auto;
  height: auto;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  contain: layout;
}

.key-card:hover {
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.4),
    0 4px 16px rgba(0, 0, 0, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.5);
  transform: translateY(-2px);
}

/* Dark mode card */
html.dark .key-card {
  background: rgba(35, 40, 55, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.12);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.15),
    0 2px 8px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

html.dark .key-card:hover {
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.2),
    0 4px 16px rgba(0, 0, 0, 0.3),
    inset 0 1px 0 rgba(255, 255, 255, 0.15);
}

.key-card.status-valid {
  border-color: rgba(124, 185, 135, 0.5);
  background: rgba(124, 185, 135, 0.08);
  border-width: 1.5px;
}

html.dark .key-card.status-valid {
  border-color: rgba(124, 185, 135, 0.4);
  background: rgba(124, 185, 135, 0.12);
}

/* Model mapping specific styles - purple theme (distinct from sub-groups) */
.key-card.status-model-mapping {
  border-color: rgba(114, 46, 209, 0.5);
  background: rgba(114, 46, 209, 0.06);
  border-width: 1.5px;
}

/* Model mapping style in dark mode */
html.dark .key-card.status-model-mapping {
  border-color: rgba(179, 127, 235, 0.4);
  background: rgba(179, 127, 235, 0.1);
}

/* Model mapping name styles */
.model-mapping-names {
  display: flex;
  align-items: baseline;
  flex: 1;
  min-width: 0;
}

/* Reuse group-url style from GroupInfoCard */
.group-url {
  font-size: 0.8rem;
  color: var(--primary-color);
  font-family: monospace;
  background: var(--bg-secondary);
  border-radius: 4px;
  padding: 2px 6px;
  border: 1px solid var(--border-color);
  cursor: pointer;
  transition: all 0.2s ease;
}

/* Weight display styles */
.weight-display {
  margin: 4px 0;
  /* Allow content to grow within card, but with reasonable height limit */
  overflow-y: auto; /* Show scrollbar when content is excessive */
  max-height: 300px; /* Set reasonable max height to ensure edit buttons are visible */
  /* Ensure correct display within card */
  flex: 1;
  min-height: 0; /* Allow shrinking */
  /* Beautify scrollbar */
  scrollbar-width: thin;
  scrollbar-color: var(--border-color) transparent;
}

/* Webkit scrollbar styles */
.weight-display::-webkit-scrollbar {
  width: 6px;
}

.weight-display::-webkit-scrollbar-track {
  background: transparent;
}

.weight-display::-webkit-scrollbar-thumb {
  background-color: var(--border-color);
  border-radius: 3px;
  border: 2px solid transparent;
}

.weight-display::-webkit-scrollbar-thumb:hover {
  background-color: var(--text-tertiary);
}

/* Sub-group progress bar container styles */
.subgroup-progress-bars {
  display: flex;
  flex-direction: column;
  gap: 3px;
  /* Ensure content expands correctly */
  height: auto;
  min-height: 0; /* Allow shrinking */
  /* Ensure content doesn't overflow */
  overflow-y: auto;
}

.target-progress-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  min-height: 22px;
}

.target-model-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  gap: 8px;
}

.model-name-chain {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 4px;
}

.subgroup-part {
  font-size: 10px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  flex-shrink: 0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

.arrow-separator {
  font-size: 10px;
  color: var(--text-tertiary);
  flex-shrink: 0;
  opacity: 0.6;
}

.model-part {
  font-size: 10px;
  color: var(--text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

/* Test button styles - using softer color scheme */
.target-test-button {
  margin-left: 8px;
  padding: 0 6px;
  height: 18px;
  font-size: 10px;
  flex-shrink: 0;
  font-weight: 500;
}

/* Test success state - soft green (low saturation) */
.target-test-button.n-button--success-type {
  border-color: rgba(16, 185, 129, 0.25);
  background: rgba(16, 185, 129, 0.05);
  color: #0d9488;
  font-weight: 500;
}

.target-test-button.n-button--success-type:hover {
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.35);
}

/* Test info state - soft blue (low saturation) */
.target-test-button.n-button--info-type {
  border-color: rgba(14, 165, 233, 0.25);
  background: rgba(14, 165, 233, 0.05);
  color: #0891b2;
  font-weight: 500;
}

.target-test-button.n-button--info-type:hover {
  background: rgba(14, 165, 233, 0.12);
  border-color: rgba(14, 165, 233, 0.35);
}

/* Test warning state - soft orange (low saturation) */
.target-test-button.n-button--warning-type {
  border-color: rgba(251, 146, 60, 0.25);
  background: rgba(251, 146, 60, 0.05);
  color: #c2410c;
  font-weight: 500;
}

.target-test-button.n-button--warning-type:hover {
  background: rgba(251, 146, 60, 0.12);
  border-color: rgba(251, 146, 60, 0.35);
}

/* Test error state - soft red (low saturation) */
.target-test-button.n-button--error-type {
  border-color: rgba(239, 68, 68, 0.25);
  background: rgba(239, 68, 68, 0.05);
  color: #b91c1c;
  font-weight: 500;
}

.target-test-button.n-button--error-type:hover {
  background: rgba(239, 68, 68, 0.12);
  border-color: rgba(239, 68, 68, 0.35);
}

.target-progress-bar {
  width: 100%;
  height: 8px;
  background: var(--bg-tertiary);
  border-radius: 4px;
  overflow: hidden;
}

.weight-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
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
  margin-top: auto;
  padding-top: 8px;
  /* Ensure action buttons are always visible */
  flex-shrink: 0;
  background: inherit;
  position: relative;
  z-index: 1;
}

.key-actions {
  flex-shrink: 0;
}

.key-actions :deep(.n-button) {
  padding: 0 4px;
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

/* Active state - 绿色渐变（与子分组保持一致） */
.key-card .weight-fill-active {
  background: linear-gradient(90deg, #0e7a43, #18a058, #36ad6a, #5fd299) !important;
}

html.dark .key-card .weight-fill-active {
  background: linear-gradient(90deg, #4aba7d, #63e2b7, #7fe7c4, #a3f5d0) !important;
}

/* Unavailable state - striped pattern (red/orange warning) */
.key-card .weight-fill-unavailable {
  background: repeating-linear-gradient(
    45deg,
    #f5a9a9,
    #f5a9a9 8px,
    #e88592 8px,
    #e88592 16px
  ) !important;
  opacity: 0.85;
}

html.dark .key-card .weight-fill-unavailable {
  background: repeating-linear-gradient(
    45deg,
    #8b3a3a,
    #8b3a3a 8px,
    #a04848 8px,
    #a04848 16px
  ) !important;
  opacity: 0.8;
}

.weight-text {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 14px;
  min-width: 40px;
  text-align: right;
}

.pagination-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--card-bg-solid);
  border-top: 1px solid var(--border-color);
  flex-shrink: 0;
  border-radius: 0 0 8px 8px;
}

.pagination-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--text-secondary);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-info {
  font-size: 12px;
  color: var(--text-secondary);
}

.empty-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 200px;
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

  .model-mappings-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .model-mapping-names {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
  }

  .quick-actions {
    flex-shrink: 0;
  }

  .key-actions {
    flex-shrink: 0;
  }

  .subgroup-part {
    font-size: 9px;
  }

  .model-part {
    font-size: 9px;
  }

  .arrow-separator {
    font-size: 9px;
  }

  .target-test-button {
    margin-left: 4px;
    padding: 0 4px;
    height: 16px;
    font-size: 9px;
  }

  .target-test-button :deep(.n-icon) {
    font-size: 9px;
  }

  .target-progress-bar {
    height: 6px;
  }

  .subgroup-progress-bars {
    gap: 2px;
  }

  .weight-display {
    max-height: 200px;
    overflow-y: auto;
  }

  .target-progress-item {
    min-height: 20px;
  }
}

/* Tooltip styles */
.model-mapping-info-tooltip {
  min-width: 300px;
  max-width: 90vw;
  width: auto;
  padding: 8px;
  max-height: 70vh;
  overflow-y: auto;
}

/* Mobile adaptation */
@media (max-width: 768px) {
  .model-mapping-info-tooltip {
    min-width: 280px;
    max-width: 85vw;
    font-size: 12px;
  }

  .info-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .info-title {
    font-size: 13px;
  }

  .info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .info-label {
    min-width: auto;
    font-size: 11px;
  }

  .info-value {
    text-align: left;
    font-size: 12px;
  }

  .target-model-detail {
    font-size: 12px;
  }
}

.info-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 10px;
  margin-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.15);
}

:root:not(.dark) .info-header {
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.info-title {
  font-size: 14px;
  font-weight: 600;
  color: inherit;
}

.info-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  line-height: 1.5;
  gap: 12px;
}

.info-label {
  color: inherit;
  opacity: 0.7;
  flex-shrink: 0;
  min-width: 100px;
  font-weight: 500;
}

.info-value {
  color: inherit;
  font-weight: 500;
  text-align: right;
  word-break: break-word;
  flex: 1;
}

.target-detail {
  margin-bottom: 4px;
  font-size: 12px;
}

.target-model-detail {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 13px;
  opacity: 0.8;
  font-weight: 400;
}
</style>
