<script setup lang="ts">
import type { GroupFormData } from "@/types/models";
import { HelpCircleOutline } from "@vicons/ionicons5";
import { NFormItem, NIcon, NInput, NInputNumber, NTooltip } from "naive-ui";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupBasicInfo",
});

interface Props {
  formData: GroupFormData;
}

interface Emits {
  (e: "update:formData", value: GroupFormData): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();

function updateFormData(key: keyof GroupFormData, value: unknown) {
  emit("update:formData", { ...props.formData, [key]: value as never });
}
</script>

<template>
  <div class="form-section">
    <h4 class="section-title">{{ t("keys.basicInfo") }}</h4>

    <div class="form-row">
      <n-form-item :label="t('keys.groupName')" path="name" class="form-item-half">
        <template #label>
          <div class="form-label-with-tooltip">
            {{ t("keys.groupName") }}
            <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
              <template #trigger>
                <n-icon :component="HelpCircleOutline" class="help-icon" />
              </template>
              {{ t("keys.groupNameTooltip") }}
            </n-tooltip>
          </div>
        </template>
        <n-input
          :value="formData.name"
          autocomplete="off"
          placeholder="gemini"
          @update:value="(value: string) => updateFormData('name', value)"
        />
      </n-form-item>

      <n-form-item :label="t('keys.displayName')" path="display_name" class="form-item-half">
        <template #label>
          <div class="form-label-with-tooltip">
            {{ t("keys.displayName") }}
            <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
              <template #trigger>
                <n-icon :component="HelpCircleOutline" class="help-icon" />
              </template>
              {{ t("keys.displayNameTooltip") }}
            </n-tooltip>
          </div>
        </template>
        <n-input
          :value="formData.display_name"
          autocomplete="off"
          placeholder="Google Gemini"
          @update:value="(value: string) => updateFormData('display_name', value)"
        />
      </n-form-item>
    </div>

    <div class="form-row">
      <n-form-item :label="t('keys.sortOrder')" path="sort" class="form-item-half">
        <template #label>
          <div class="form-label-with-tooltip">
            {{ t("keys.sortOrder") }}
            <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
              <template #trigger>
                <n-icon :component="HelpCircleOutline" class="help-icon" />
              </template>
              {{ t("keys.sortOrderTooltip") }}
            </n-tooltip>
          </div>
        </template>
        <n-input-number
          :value="formData.sort"
          :min="0"
          :placeholder="t('keys.sortValue')"
          style="width: 100%"
          @update:value="(value: number | null) => updateFormData('sort', value || 0)"
        />
      </n-form-item>
    </div>

    <n-form-item :label="t('common.description')" path="description">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("common.description") }}
          <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon" />
            </template>
            {{ t("keys.descriptionTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <n-input
        :value="formData.description"
        type="textarea"
        placeholder=""
        :rows="1"
        :autosize="{ minRows: 1, maxRows: 5 }"
        style="resize: none"
        @update:value="(value: string) => updateFormData('description', value)"
      />
    </n-form-item>
  </div>
</template>

<style scoped>
.form-section {
  margin-bottom: 24px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--n-text-color);
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--n-border-color);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

:deep(.n-form-item-label) {
  font-weight: 500;
}

:deep(.n-form-item-blank) {
  flex-grow: 1;
}

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

  .section-title {
    font-size: 0.9rem;
  }
}
</style>
