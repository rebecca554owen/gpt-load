<script setup lang="ts">
import { keysApi } from "@/api/keys";
import { useModelTestStatus } from "@/composables/useModelTestStatus";
import { useCopy } from "@/composables/useCopy";
import type { Group, ModelMapping, ModelMappingTarget, SubGroupInfo } from "@/types/models";
import { formatDuration } from "@/utils/format";
import {
  CheckmarkCircle,
  CopyOutline,
  CreateOutline,
  InformationCircleOutline,
  PlayCircleOutline,
  Trash,
} from "@vicons/ionicons5";
import { NButton, NButtonGroup, NIcon, NTag, NTooltip, useMessage } from "naive-ui";
import { useI18n } from "vue-i18n";

interface ModelMappingRow extends ModelMapping {
  totalWeight: number;
}

interface Props {
  mapping: ModelMappingRow;
  selectedGroup: Group | null;
  subGroups?: SubGroupInfo[];
  groups?: Group[];
}

interface Emits {
  (e: "edit", mapping: ModelMapping): void;
  (e: "duplicate", mapping: ModelMapping): void;
  (e: "delete", mapping: ModelMapping): void;
  (e: "refresh"): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();
const message = useMessage();
const { copyWithFeedback } = useCopy();

const {
  startTesting,
  setSuccess,
  setFailure,
  finishTesting,
  isTesting,
  hasFailed,
  hasSucceeded,
  getResult,
} = useModelTestStatus();

function getTestKey(target: ModelMappingTarget): string {
  return `${props.mapping.model}_${target.model}_${target.sub_group_id}`;
}

function getSubGroupName(subGroupId: number): string {
  const subGroup = props.subGroups?.find(sg => sg.group.id === subGroupId);
  if (subGroup) {
    return subGroup.group.display_name || subGroup.group.name;
  }
  const group = props.groups?.find(g => g.id === subGroupId);
  return group?.display_name || group?.name || `#${subGroupId}`;
}

function getTargetPercentage(target: ModelMappingTarget, mapping: ModelMappingRow): number {
  const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
  if (!subGroup || subGroup.active_keys === 0) {
    return 0;
  }
  const totalWeight = mapping.totalWeight;
  if (totalWeight === 0) {
    return 0;
  }
  return Math.round((target.weight / totalWeight) * 100);
}

async function testSubGroupModel(target: ModelMappingTarget) {
  const testKey = getTestKey(target);

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
    if (!startTesting(testKey)) {
      return;
    }

    const loadingMessage = message.loading(
      t("modelMappings.testingModel", { model: target.model }),
      { duration: 0 }
    );

    const response = await keysApi.testNextKey(target.sub_group_id, target.model);
    const result = response.result;

    loadingMessage.destroy();

    if (result.is_valid) {
      setSuccess(testKey, response.total_duration || 0);
      message.success(
        t("modelMappings.testSuccess", {
          model: target.model,
          duration: formatDuration(response.total_duration || 0),
        })
      );
    } else {
      setFailure(testKey);
      message.error(
        t("modelMappings.testFailed", {
          model: target.model,
          error: result.error || t("modelMappings.testFailedGeneric"),
        }),
        { keepAliveOnHover: true, duration: 5000, closable: true }
      );
    }
  } catch (error) {
    console.error(t("modelMappings.testModelFailed"), error);
    setFailure(testKey);
    message.error(
      t("modelMappings.testFailed", {
        model: target.model,
        error: error instanceof Error ? error.message : t("modelMappings.unknownError"),
      })
    );
  } finally {
    finishTesting(testKey);
  }
}

function getTestButtonType(testKey: string): "default" | "info" | "warning" | "success" | "error" {
  if (hasFailed(testKey)) {
    return "error";
  }

  const result = getResult(testKey);
  if (!result) {
    return "default";
  }

  const duration = result.duration;
  if (duration < 200) {
    return "success";
  }
  if (duration < 500) {
    return "info";
  }
  if (duration < 1000) {
    return "warning";
  }
  if (duration < 2000) {
    return "default";
  }
  if (duration < 3000) {
    return "warning";
  }
  if (duration < 4000) {
    return "error";
  }
  if (duration < 5000) {
    return "error";
  }
  return "default";
}

function getTestButtonClass(testKey: string): string {
  if (hasFailed(testKey)) {
    return "test-failed";
  }

  const result = getResult(testKey);
  if (!result) {
    return "";
  }

  const duration = result.duration;
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

function getTestButtonText(testKey: string): string {
  if (hasFailed(testKey)) {
    return t("modelMappings.failed");
  }

  const result = getResult(testKey);
  if (!result) {
    return t("modelMappings.test");
  }

  return formatDuration(result.duration);
}

async function copyModelAlias() {
  await copyWithFeedback(props.mapping.model);
}

function handleEdit() {
  emit("edit", props.mapping);
}

function handleDuplicate() {
  emit("duplicate", props.mapping);
}

function handleDelete() {
  emit("delete", props.mapping);
}
</script>

<template>
  <div class="key-card status-model-mapping">
    <div class="key-main">
      <div class="key-section">
        <div class="quick-actions">
          <n-tag type="info" size="small">模型别名</n-tag>
        </div>
        <div class="model-mapping-names">
          <n-tooltip trigger="hover">
            <template #trigger>
              <span class="group-url-wrapper" @click="copyModelAlias">
                <code class="group-url">
                  {{ mapping.model }}
                </code>
              </span>
            </template>
            {{ t("keys.clickToCopy") }}
          </n-tooltip>
        </div>
      </div>
    </div>

    <div class="weight-display">
      <div class="subgroup-progress-bars">
        <div v-for="(target, index) in mapping.targets" :key="index" class="target-progress-item">
          <div class="target-model-row">
            <div class="model-name-chain">
              <span class="subgroup-part">{{ getSubGroupName(target.sub_group_id) }}</span>
              <span class="arrow-separator">→</span>
              <span class="model-part">{{ target.model }}</span>
            </div>
            <n-button
              size="tiny"
              :type="getTestButtonType(getTestKey(target))"
              :class="['target-test-button', getTestButtonClass(getTestKey(target))]"
              ghost
              @click="testSubGroupModel(target)"
              :disabled="
                (subGroups?.find(sg => sg.group.id === target.sub_group_id)?.active_keys || 0) ===
                  0 || isTesting(getTestKey(target))
              "
              :loading="isTesting(getTestKey(target))"
            >
              <template #icon v-if="!isTesting(getTestKey(target))">
                <n-icon
                  :component="
                    hasSucceeded(getTestKey(target)) ? CheckmarkCircle : PlayCircleOutline
                  "
                />
              </template>
              {{
                isTesting(getTestKey(target))
                  ? t("modelMappings.testInProgress")
                  : getTestButtonText(getTestKey(target))
              }}
            </n-button>
          </div>
          <div class="target-progress-bar">
            <div
              class="weight-fill"
              :class="{
                'weight-fill-active':
                  (subGroups?.find(sg => sg.group.id === target.sub_group_id)?.active_keys || 0) >
                  0,
                'weight-fill-unavailable':
                  (subGroups?.find(sg => sg.group.id === target.sub_group_id)?.active_keys || 0) ===
                  0,
              }"
              :style="{ width: `${Math.min(getTargetPercentage(target, mapping), 100)}%` }"
            />
          </div>
        </div>
      </div>
    </div>

    <div class="key-bottom">
      <n-tooltip trigger="hover" placement="right">
        <template #trigger>
          <n-button round tertiary type="default" size="tiny">
            <template #icon>
              <n-icon :component="InformationCircleOutline" />
            </template>
          </n-button>
        </template>
        <div class="model-mapping-info-tooltip">
          <div class="info-header">
            <div class="info-title">{{ mapping.model }}</div>
            <n-tag type="info" size="small">模型映射</n-tag>
          </div>
          <div class="info-details">
            <div class="info-row">
              <span class="info-label">总权重:</span>
              <span class="info-value">{{ mapping.totalWeight }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">目标数量:</span>
              <span class="info-value">{{ mapping.targets.length }}</span>
            </div>
            <div v-for="(target, index) in mapping.targets" :key="index" class="target-detail">
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

      <n-button-group class="key-actions">
        <n-button
          round
          tertiary
          class="btn-edit"
          size="tiny"
          @click="handleEdit"
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
          @click="handleDuplicate"
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
          @click="handleDelete"
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
</template>

<style scoped>
.key-card {
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

.key-card.status-model-mapping {
  border-color: rgba(114, 46, 209, 0.5);
  background: rgba(114, 46, 209, 0.06);
  border-width: 1.5px;
}

html.dark .key-card.status-model-mapping {
  border-color: rgba(179, 127, 235, 0.4);
  background: rgba(179, 127, 235, 0.1);
}

.key-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.key-section {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.quick-actions {
  flex-shrink: 0;
}

.model-mapping-names {
  display: flex;
  align-items: baseline;
  flex: 1;
  min-width: 0;
}

.group-url-wrapper {
  cursor: pointer;
}

.group-url {
  font-size: 0.8rem;
  color: var(--primary-color);
  font-family: monospace;
  background: var(--bg-secondary);
  border-radius: 4px;
  padding: 2px 6px;
  border: 1px solid var(--border-color);
  transition: all 0.2s ease;
}

.weight-display {
  margin: 4px 0;
  overflow-y: auto;
  max-height: 300px;
  flex: 1;
  min-height: 0;
  scrollbar-width: thin;
  scrollbar-color: var(--border-color) transparent;
}

.subgroup-progress-bars {
  display: flex;
  flex-direction: column;
  gap: 3px;
  height: auto;
  min-height: 0;
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

.target-test-button {
  margin-left: 8px;
  padding: 0 6px;
  height: 18px;
  font-size: 10px;
  flex-shrink: 0;
  font-weight: 500;
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
  transition: width 0.3s ease;
}

.weight-fill-active {
  background: linear-gradient(90deg, #0e7a43, #18a058, #36ad6a, #5fd299);
}

html.dark .weight-fill-active {
  background: linear-gradient(90deg, #4aba7d, #63e2b7, #7fe7c4, #a3f5d0);
}

.weight-fill-unavailable {
  background: var(--border-color);
}

.key-bottom {
  margin-top: auto;
  padding-top: 8px;
  flex-shrink: 0;
  background: inherit;
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.key-actions {
  display: flex;
  gap: 4px;
}

.btn-edit,
.btn-view,
.btn-delete {
  transition: all 0.2s ease;
}

.model-mapping-info-tooltip {
  min-width: 300px;
  max-width: 90vw;
}

.info-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.info-title {
  font-weight: 600;
  font-size: 14px;
}

.info-details {
  font-size: 12px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 2px 0;
}

.info-label {
  color: var(--text-secondary);
  min-width: 80px;
}

.info-value {
  color: var(--text-primary);
  text-align: right;
}

.target-detail {
  margin-bottom: 4px;
  padding-top: 4px;
  border-top: 1px solid var(--border-color);
}

.target-model-detail {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 13px;
  opacity: 0.8;
  font-weight: 400;
}

@media (max-width: 768px) {
  .subgroup-part,
  .model-part,
  .arrow-separator {
    font-size: 9px;
  }

  .target-test-button {
    margin-left: 4px;
    padding: 0 4px;
    height: 16px;
    font-size: 9px;
  }

  .target-progress-bar {
    height: 6px;
  }

  .model-mapping-info-tooltip {
    min-width: 280px;
    max-width: 85vw;
    font-size: 12px;
  }
}
</style>
