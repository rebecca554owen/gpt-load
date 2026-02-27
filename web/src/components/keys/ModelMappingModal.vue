<script setup lang="ts">
import { keysApi } from "@/api/keys";
import useApi from "@/composables/useApi";
import useLoading from "@/composables/useLoading";
import type { Group, ModelMapping, ModelMappingTarget, SubGroupInfo } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import { Add, Close, RemoveCircleOutline } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSpace,
  NTag,
  NText,
  type FormRules,
} from "naive-ui";
import { computed, nextTick, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  mode: "add" | "edit";
  aggregateGroup: Group | null;
  modelMapping?: ModelMapping | null; // Passed in edit mode
  existingModelMappings: ModelMapping[];
  subGroups: SubGroupInfo[];
  groups: Group[];
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success"): void;
  (e: "group-updated", group: Group): void;
}

interface TargetItem extends ModelMappingTarget {
  id: string;
  editingModel?: string | null; // Model name being edited
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();
const { handleApiError, handleApiSuccess } = useApi();
const { loading, withLoading } = useLoading();
const formRef = ref();

// Form data
const formData = reactive<{
  model: string;
  targets: TargetItem[];
}>({
  model: "",
  targets: [{ id: "1", sub_group_id: 0, weight: 1, model: "" }],
});

// Original model alias (for duplicate check)
const originalModelName = ref("");

// Compute modal title
const modalTitle = computed(() => {
  return props.mode === "edit"
    ? t("modelMappings.editModelMapping")
    : t("modelMappings.addModelMapping");
});

// Compute available sub-group options
const getAvailableSubGroups = computed(() => {
  if (!props.subGroups) {
    return [];
  }

  return props.subGroups.map(subGroup => ({
    label: getGroupDisplayName(subGroup.group),
    value: subGroup.group.id,
  }));
});

// Get options for specified index (remove unique selection limit, allow duplicate sub-group selection)
function getOptionsForTarget(_index: number) {
  return getAvailableSubGroups.value;
}

// Form validation rules
const rules: FormRules = {
  model: {
    required: true,
    message: t("modelMappings.modelAliasRequired"),
    trigger: ["blur", "input"],
    validator: (_rule, value) => {
      if (!value || !value.trim()) {
        return new Error(t("modelMappings.modelAliasRequired"));
      }

      // Check if duplicate with existing model mappings
      const isDuplicate = props.existingModelMappings.some(mapping => {
        // Edit mode: exclude current mapping being edited
        if (props.mode === "edit" && mapping.model === originalModelName.value) {
          return false;
        }
        return mapping.model.toLowerCase() === value.trim().toLowerCase();
      });

      if (isDuplicate) {
        return new Error(t("modelMappings.duplicateModelAlias"));
      }

      return true;
    },
  },
  targets: {
    type: "array",
    required: true,
    validator: (_rule, value: TargetItem[]) => {
      // Check if at least one valid target exists
      const validTargets = value.filter(target => {
        const hasValidSubGroup = target.sub_group_id > 0;
        const hasValidModel = target.model && target.model.trim() !== "";
        return hasValidSubGroup && hasValidModel;
      });
      if (validTargets.length === 0) {
        return new Error(t("modelMappings.atLeastOneTarget"));
      }

      // Check if weight is valid
      for (const target of validTargets) {
        if (target.weight < 0) {
          return new Error(t("modelMappings.invalidWeight"));
        }
      }

      return true;
    },
    trigger: ["blur", "change"],
  },
};

// Watch modal display state and data changes
watch(
  [() => props.show, () => props.modelMapping, () => props.mode],
  ([show, modelMapping, mode]) => {
    if (show) {
      if (
        modelMapping &&
        (mode === "edit" ||
          (mode === "add" && modelMapping.model && modelMapping.targets.length > 0))
      ) {
        // Load data in edit mode or add mode with pre-filled data
        loadModelMappingData();
      } else {
        resetForm();
      }
    }
  }
);

// Reset form
function resetForm() {
  formData.model = "";
  // Use first sub-group's weight as default, or 1 if none exists
  const defaultWeight =
    props.subGroups && props.subGroups.length > 0 ? props.subGroups[0].weight : 1;
  formData.targets = [{ id: "1", sub_group_id: 0, weight: defaultWeight, model: "" }];
  originalModelName.value = "";
}

// Load model mapping data to form (edit mode)
function loadModelMappingData() {
  if (!props.modelMapping) {
    return;
  }

  originalModelName.value = props.modelMapping.model;
  formData.model = props.modelMapping.model;
  formData.targets = props.modelMapping.targets.map((target, index) => ({
    ...target,
    id: (index + 1).toString(),
  }));
}

// Add target item
function addTargetItem() {
  const newId = (Math.max(...formData.targets.map(t => parseInt(t.id))) + 1).toString();
  // Use first sub-group's weight as default, or 1 if none exists
  const defaultWeight =
    props.subGroups && props.subGroups.length > 0 ? props.subGroups[0].weight : 1;
  formData.targets.push({
    id: newId,
    sub_group_id: 0,
    weight: defaultWeight,
    model: "",
  });
}

// Remove target item
function removeTargetItem(id: string) {
  if (formData.targets.length > 1) {
    formData.targets = formData.targets.filter(t => t.id !== id);
  }
}

// Close modal
function handleClose() {
  emit("update:show", false);
}

// Cancel operation
function handleCancel() {
  emit("update:show", false);
}

// Submit form
async function handleSubmit() {
  if (loading.value || !props.aggregateGroup?.id) {
    return;
  }

  await withLoading(async () => {
    await formRef.value?.validate();

    if (!props.aggregateGroup?.id) {
      throw new Error("Aggregate group not found");
    }

    // Filter out valid target configurations
    const validTargets = formData.targets
      .filter(target => {
        const hasValidSubGroup = target.sub_group_id > 0;
        const hasValidModel = target.model && target.model.trim() !== "";
        return hasValidSubGroup && hasValidModel;
      })
      .map(target => ({
        sub_group_id: target.sub_group_id,
        weight: target.weight,
        model: target.model.trim(),
      }));

    if (validTargets.length === 0) {
      handleApiError(t("modelMappings.targetsRequired"));
      throw new Error(t("modelMappings.targetsRequired"));
    }

    // Build model mapping
    const newMapping: ModelMapping = {
      model: formData.model.trim(),
      targets: validTargets,
    };

    let updatedMappings: ModelMapping[];

    if (props.mode === "edit") {
      // Edit mode: update existing mapping
      updatedMappings = props.existingModelMappings.map(mapping =>
        mapping.model === props.modelMapping?.model ? newMapping : mapping
      );
    } else {
      // Add mode: add new mapping
      updatedMappings = [...props.existingModelMappings, newMapping];
    }

    const response = await keysApi.updateGroup(props.aggregateGroup.id, {
      model_mappings: updatedMappings,
    });

    // Check response format, if array take first element
    let updatedGroup: Group | null = null;
    if (Array.isArray(response)) {
      updatedGroup = response[0];
    } else {
      updatedGroup = response;
    }

    // Emit group-updated event if successfully obtained updated group data
    if (updatedGroup) {
      emit("group-updated", updatedGroup);
    }

    handleApiSuccess();
    emit("success");
    handleClose();
  });
}

// Auto-fill default model name and weight based on sub-group ID
function handleSubGroupChange(targetId: string, subGroupId: number) {
  const target = formData.targets.find(t => t.id === targetId);
  if (target) {
    // Sync sub-group weight
    const subGroup = props.subGroups.find(sg => sg.group.id === subGroupId);
    if (subGroup) {
      target.weight = subGroup.weight;
      // If model name is empty, auto-fill sub-group's test model
      if (!target.model.trim() && subGroup.group.test_model) {
        target.model = subGroup.group.test_model;
      }
    }
  }
}

// Edit model
function editModel(target: TargetItem, event: MouseEvent) {
  target.editingModel = target.model;
  // Use nextTick to ensure DOM updates before focusing
  nextTick(() => {
    // Use event target element to find corresponding input
    const targetElement = event.currentTarget as HTMLElement;
    const input = targetElement?.querySelector(".model-config-input") as HTMLInputElement;
    if (input) {
      input.focus();
      // Select all text for easy editing
      input.select();
    }
  });
}

// Finish editing
function finishEdit(target: TargetItem) {
  if (target.editingModel !== undefined && target.editingModel !== null) {
    const newModelName = target.editingModel.trim();
    target.model = newModelName;
  }
  target.editingModel = null;
}

// Cancel editing
function cancelEdit(target: TargetItem) {
  target.editingModel = null;
}
</script>

<template>
  <n-modal
    :show="show"
    @update:show="handleClose"
    class="model-mapping-modal modal-mask"
    :mask-closable="true"
    :closable="false"
  >
    <n-card
      class="model-mapping-card modal-card modal-extra-wide"
      :title="modalTitle"
      :bordered="false"
      size="huge"
      role="dialog"
      aria-modal="true"
    >
      <template #header-extra>
        <n-button quaternary circle @click="handleClose" class="modal-close">
          <template #icon>
            <n-icon :component="Close" />
          </template>
        </n-button>
      </template>

      <n-form
        ref="formRef"
        :model="formData"
        :rules="rules"
        label-placement="left"
        label-width="100px"
      >
        <div class="form-section">
          <n-form-item
            :label="t('modelMappings.modelAlias')"
            path="model"
            class="model-alias-form-item"
          >
            <n-input
              v-model:value="formData.model"
              :placeholder="t('modelMappings.modelAliasPlaceholder')"
              clearable
            />
            <template #feedback>
              <n-space vertical :size="4">
                <n-tag type="info" size="small" :bordered="false">
                  {{ t("modelMappings.wildcardHint") }}
                </n-tag>
                <n-text depth="3" style="font-size: 11px">
                  {{ t("modelMappings.wildcardExample1") }}
                </n-text>
                <n-text depth="3" style="font-size: 11px">
                  {{ t("modelMappings.wildcardExample2") }}
                </n-text>
              </n-space>
            </template>
          </n-form-item>
        </div>

        <div class="form-section">
          <h4 class="section-title">
            {{ t("modelMappings.targetConfig") }}
          </h4>

          <div class="targets-list">
            <div
              v-for="target in formData.targets"
              :key="target.id"
              class="target-item"
              :class="{ disabled: target.weight === 0 }"
            >
              <!-- Single row layout -->
              <div class="target-row">
                <span class="target-label">
                  {{ t("modelMappings.targetSubGroup") }} {{ target.id }}
                </span>

                <n-form-item
                  class="item-select"
                  :path="`targets[${formData.targets.indexOf(target)}].sub_group_id`"
                  :show-feedback="false"
                >
                  <n-select
                    :value="target.sub_group_id || null"
                    :options="getOptionsForTarget(formData.targets.indexOf(target))"
                    :placeholder="t('subGroups.selectSubGroup')"
                    clearable
                    @update:value="
                      value => {
                        target.sub_group_id = value || 0;
                        handleSubGroupChange(target.id, value || 0);
                      }
                    "
                  />
                </n-form-item>

                <n-form-item
                  class="item-weight"
                  :path="`targets[${formData.targets.indexOf(target)}].weight`"
                  :show-feedback="false"
                >
                  <n-input-number
                    v-model:value="target.weight"
                    :min="0"
                    :max="100"
                    :step="1"
                    :placeholder="t('keys.weight')"
                    size="small"
                  />
                </n-form-item>

                <!-- Actual model configuration area -->
                <n-form-item class="item-models" :show-feedback="false">
                  <div class="model-config-container">
                    <div
                      class="model-config-tag"
                      :class="{
                        editing: target.editingModel !== undefined && target.editingModel !== null,
                      }"
                      @click="editModel(target, $event)"
                    >
                      <input
                        v-if="target.editingModel !== undefined && target.editingModel !== null"
                        v-model="target.editingModel"
                        @blur="finishEdit(target)"
                        @keyup.enter="finishEdit(target)"
                        @keyup.esc="cancelEdit(target)"
                        class="model-config-input"
                        ref="modelInput"
                      />
                      <span v-else>{{ target.model || t("modelMappings.clickToEditModel") }}</span>
                    </div>
                  </div>
                </n-form-item>

                <span v-if="target.weight === 0" class="disabled-badge">
                  {{ t("modelMappings.disabled") }}
                </span>

                <n-button
                  @click="removeTargetItem(target.id)"
                  size="small"
                  class="btn-delete"
                  :style="{ visibility: formData.targets.length > 1 ? 'visible' : 'hidden' }"
                >
                  <template #icon>
                    <n-icon :component="RemoveCircleOutline" />
                  </template>
                </n-button>
              </div>
            </div>
          </div>

          <div class="add-item-section">
            <n-button @click="addTargetItem" dashed class="full-width-button btn-create">
              <template #icon>
                <n-icon :component="Add" />
              </template>
              {{ t("modelMappings.addTarget") }}
            </n-button>
          </div>
        </div>
      </n-form>

      <template #footer>
        <div class="modal-footer">
          <n-button @click="handleCancel" class="btn-cancel">{{ t("common.cancel") }}</n-button>
          <n-button @click="handleSubmit" :loading="loading" class="btn-confirm">
            {{ t("common.confirm") }}
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.model-mapping-modal {
  /* 继承 modal-mask 样式 */
}

.model-mapping-card {
  /* 继承 modal-card 和 modal-extra-wide 样式 */
}

.form-section {
  margin-bottom: 16px;
}

.form-section:first-child {
  margin-bottom: 12px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--n-text-color);
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--n-border-color);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.model-alias-form-item {
  margin-bottom: 8px;
}

.targets-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 20px;
}

.target-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius-md);
}

.target-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.target-label {
  flex-shrink: 0;
  min-width: 100px;
  font-weight: 500;
  color: var(--text-primary);
  font-size: 0.9rem;
}

.item-select {
  flex: 1;
  min-width: 200px;
}

.item-weight {
  width: 100px;
  flex-shrink: 0;
}

.item-models {
  flex: 2;
  min-width: 300px;
}

.model-config-container {
  display: flex;
  align-items: center;
  width: 100%;
}

.model-config-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 32px;
  padding: 0 8px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  font-size: 0.85rem;
  font-weight: 500;
  cursor: text;
  min-width: 300px;
  max-width: 100%;
  flex: 1;
  transition: all 0.2s ease;
}

.model-config-tag.editing {
  height: auto;
  min-height: 32px;
  padding: 8px 12px;
  background: var(--bg-secondary);
  border-color: var(--border-color);
}

.model-config-tag:hover {
  background: var(--bg-secondary);
  border-color: var(--border-color);
}

.model-config-input {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 0.85rem;
  font-weight: 500;
  outline: none;
  width: 100%;
  min-width: 280px;
  padding: 2px 8px;
}

.model-config-input::placeholder {
  color: var(--text-tertiary);
}

.item-delete {
  flex-shrink: 0;
}

.target-item.disabled {
  opacity: 0.6;
  background: var(--bg-tertiary);
}

.disabled-badge {
  margin-left: auto;
  padding: 2px 8px;
  background: var(--error-color);
  color: white;
  border-radius: 4px;
  font-size: 0.75rem;
}

.add-item-section {
  margin-top: 16px;
}

.full-width-button {
  width: 100%;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.btn-create {
  background: var(--btn-create-bg) !important;
  color: white !important;
  border: none !important;
}

.btn-delete {
  background: var(--btn-delete-bg) !important;
  color: white !important;
  border: none !important;
}

.btn-edit {
  background: var(--btn-edit-bg) !important;
  color: white !important;
  border: none !important;
}

/* Responsive adaptation */
@media (max-width: 768px) {
  .mobile-form {
    padding: 8px;
  }

  .mobile-form .form-section {
    margin-bottom: 20px;
  }

  .mobile-form .model-alias-form-item {
    margin-bottom: 16px;
  }

  .target-item {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
    padding: 16px;
    margin-bottom: 12px;
  }

  .target-row {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .target-label {
    width: 100%;
    text-align: left;
    margin-bottom: 4px;
    font-weight: 600;
  }

  .item-select,
  .item-weight,
  .item-models {
    width: 100%;
    min-width: auto;
    flex: none;
  }

  .item-delete {
    align-self: flex-end;
    margin-top: 8px;
  }

  .model-config-container {
    width: 100%;
    justify-content: stretch;
  }

  .model-config-tag {
    width: 100%;
    justify-content: stretch;
    min-height: 40px;
    padding: 8px 16px;
    min-width: auto;
  }

  .model-config-tag.editing {
    min-height: 48px;
    padding: 12px 16px;
  }

  .model-config-input {
    text-align: left;
    min-width: auto;
    width: 100%;
    min-height: 32px;
    padding: 6px 8px;
    font-size: 0.9rem;
  }

  .add-item-section {
    margin-top: 20px;
  }
}
</style>
