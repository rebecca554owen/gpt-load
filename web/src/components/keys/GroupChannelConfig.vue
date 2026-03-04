<script setup lang="ts">
import type { GroupFormData } from "@/types/models";
import { HelpCircleOutline } from "@vicons/ionicons5";
import { NFormItem, NIcon, NInput, NSelect, NTooltip, type SelectOption } from "naive-ui";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupChannelConfig",
});

interface Props {
  formData: GroupFormData;
  channelTypeOptions: SelectOption[];
  testModelPlaceholder: string;
  validationEndpointPlaceholder: string;
  isEditMode: boolean;
}

interface Emits {
  (e: "update:formData", value: GroupFormData): void;
  (e: "fetchChannelTypes"): void;
  (e: "testModelModified"): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();

function updateFormData(key: keyof GroupFormData, value: unknown) {
  emit("update:formData", { ...props.formData, [key]: value as never });
}

function handleChannelTypeChange(value: string) {
  updateFormData("channel_type", value);
  emit("fetchChannelTypes");
}

function handleTestModelInputChange(value: string) {
  updateFormData("test_model", value);
  if (!props.isEditMode) {
    emit("testModelModified");
  }
}

function handleValidationEndpointChange(value: string) {
  updateFormData("validation_endpoint", value);
}
</script>

<template>
  <div class="form-row">
    <n-form-item :label="t('keys.channelType')" path="channel_type" class="form-item-half">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.channelType") }}
          <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon" />
            </template>
            {{ t("keys.channelTypeTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <n-select
        :value="formData.channel_type"
        :options="channelTypeOptions"
        :placeholder="t('keys.selectChannelType')"
        @update:value="handleChannelTypeChange"
      />
    </n-form-item>
  </div>

  <div class="form-row">
    <n-form-item :label="t('keys.testModel')" path="test_model" class="form-item-half">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.testModel") }}
          <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon" />
            </template>
            {{ t("keys.testModelTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <n-input
        :value="formData.test_model"
        :placeholder="testModelPlaceholder"
        autocomplete="off"
        @update:value="handleTestModelInputChange"
      />
    </n-form-item>

    <n-form-item
      :label="t('keys.testPath')"
      path="validation_endpoint"
      class="form-item-half"
      v-if="formData.channel_type !== 'gemini'"
    >
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.testPath") }}
          <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon" />
            </template>
            <div>
              {{ t("keys.testPathTooltip1") }}
              <br />
              • OpenAI: /v1/chat/completions
              <br />
              • Anthropic: /v1/messages
              <br />
              {{ t("keys.testPathTooltip2") }}
            </div>
          </n-tooltip>
        </div>
      </template>
      <n-input
        :value="formData.validation_endpoint"
        :placeholder="validationEndpointPlaceholder || t('keys.optionalCustomValidationPath')"
        autocomplete="off"
        @update:value="handleValidationEndpointChange"
      />
    </n-form-item>

    <div v-else class="form-item-half" />
  </div>
</template>

<style scoped>
.form-row {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

.form-item-half {
  flex: 1;
  width: 50%;
}

.form-label-with-tooltip {
  display: flex;
  align-items: center;
  gap: 6px;
}

.help-icon {
  color: var(--text-tertiary);
  font-size: 14px;
  cursor: help;
  transition: color 0.2s ease;
}

.help-icon:hover {
  color: var(--primary-color);
}

@media (max-width: 768px) {
  .form-row {
    flex-direction: column;
    gap: 0;
  }

  .form-item-half {
    width: 100%;
  }
}
</style>
