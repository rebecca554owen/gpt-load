<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group, ModelMapping, SubGroupInfo } from "@/types/models";
import { Add, Search } from "@vicons/ionicons5";
import { NButton, NEmpty, NIcon, NInput, NSpin, useDialog, useMessage } from "naive-ui";
import { computed, nextTick, ref } from "vue";
import { useI18n } from "vue-i18n";
import ModelMappingCard from "./ModelMappingCard.vue";
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
const searchText = ref("");

const sortedModelMappings = computed<ModelMappingRow[]>(() => {
  if (!props.modelMappings) {
    return [];
  }

  return props.modelMappings
    .map(mapping => {
      const validWeight = mapping.targets.reduce((sum, target) => {
        const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
        if (subGroup && subGroup.active_keys > 0) {
          return sum + target.weight;
        }
        return sum;
      }, 0);

      return { ...mapping, totalWeight: validWeight };
    })
    .sort((a, b) => a.model.localeCompare(b.model));
});

const filteredModelMappings = computed<ModelMappingRow[]>(() => {
  let filtered = sortedModelMappings.value;
  if (searchText.value.trim()) {
    const searchLower = searchText.value.trim().toLowerCase();
    filtered = filtered.filter(mapping => mapping.model.toLowerCase().includes(searchLower));
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
        const remainingMappings =
          props.modelMappings?.filter(m => m.model !== modelMapping.model) || [];
        await keysApi.updateGroup(props.selectedGroup.id, { model_mappings: remainingMappings });
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

function handleCardEdit(mapping: ModelMapping) {
  openEditModal(mapping);
}

function handleCardDuplicate(mapping: ModelMapping) {
  if (!props.selectedGroup?.id) {
    message.error(t("keys.selectGroup"));
    return;
  }

  let counter = 1;
  let newAlias = `${mapping.model}-${counter}`;
  while (props.modelMappings?.some(m => m.model === newAlias)) {
    counter++;
    newAlias = `${mapping.model}-${counter}`;
  }

  const duplicatedMapping: ModelMapping = {
    model: newAlias,
    targets: mapping.targets.map(target => ({ ...target })),
  };

  editingModelMapping.value = duplicatedMapping;
  modalMode.value = "add";
  nextTick(() => {
    modalShow.value = true;
  });
  message.info(t("modelMappings.duplicatedMappingInfo"));
}

function handleCardDelete(mapping: ModelMapping) {
  deleteModelMapping(mapping);
}

function handleModalSuccess() {
  emit("refresh");
}

function handleCardRefresh() {
  emit("refresh");
}
</script>

<template>
  <div class="model-mapping-table-container">
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
          <model-mapping-card
            v-for="mapping in filteredModelMappings"
            :key="mapping.model"
            :mapping="mapping"
            :selected-group="selectedGroup"
            :sub-groups="subGroups"
            :groups="groups"
            @edit="handleCardEdit"
            @duplicate="handleCardDuplicate"
            @delete="handleCardDelete"
            @refresh="handleCardRefresh"
          />
        </div>
      </n-spin>
    </div>

    <div class="pagination-container">
      <div class="pagination-info">
        <span>
          {{ t("modelMappings.totalMappings", { count: filteredModelMappings.length }) }}
          <template v-if="filteredModelMappings.length !== (props.modelMappings?.length || 0)">
            / {{ props.modelMappings?.length || 0 }}
          </template>
        </span>
      </div>
      <div class="pagination-controls">
        <span class="page-info">{{ t("modelMappings.sortedByModelName") }}</span>
      </div>
    </div>

    <model-mapping-modal
      v-if="selectedGroup?.id"
      v-model:show="modalShow"
      :mode="modalMode"
      :aggregate-group="selectedGroup"
      :model-mapping="editingModelMapping"
      :existing-model-mappings="modelMappings || []"
      :sub-groups="subGroups || []"
      :groups="groups || []"
      @success="handleModalSuccess"
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
  gap: 12px;
}

.toolbar-left {
  display: flex;
  gap: 8px;
}

.toolbar-right {
  display: flex;
  gap: 8px;
}

.model-mappings-grid-container {
  flex: 1;
  overflow-y: visible;
  overflow-x: hidden;
  padding: 16px;
  min-height: 0;
  height: auto;
  max-height: none;
}

.model-mappings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  grid-auto-rows: min-content;
  align-items: start;
  overflow: visible;
}

.empty-container {
  padding: 48px 24px;
  text-align: center;
}

.pagination-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-top: 1px solid var(--border-color);
  background: var(--bg-secondary);
}

.pagination-info {
  font-size: 13px;
  color: var(--text-secondary);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-info {
  font-size: 12px;
  color: var(--text-tertiary);
}

.btn-create {
  background: var(--btn-create-bg);
  color: white;
  border: none;
  transition: all 0.2s;
}

.btn-create:hover:not(:disabled) {
  background: var(--btn-create-hover);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
}

.btn-create:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
  }

  .toolbar-right > * {
    width: 100%;
  }

  .model-mappings-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .pagination-container {
    flex-direction: column;
    gap: 8px;
    text-align: center;
  }
}
</style>
