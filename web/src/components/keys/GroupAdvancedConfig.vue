<script setup lang="ts">
import type { GroupFormData } from "@/types/models";
import { HelpCircleOutline } from "@vicons/ionicons5";
import { NFormItem, NIcon, NSwitch, NTooltip } from "naive-ui";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupAdvancedConfig",
});

interface Props {
  formData: GroupFormData;
  modelRedirectTip: string;
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

const showModelRedirect = computed(() => props.formData.group_type !== "aggregate");
const showModelMappingStrict = computed(() => props.formData.group_type === "aggregate");
</script>

<template>
  <div v-if="showModelRedirect" class="config-section">
    <n-form-item path="model_redirect_strict">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.modelRedirectPolicy") }}
          <n-tooltip trigger="hover" placement="top">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
            </template>
            {{ t("keys.modelRedirectPolicyTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <div style="display: flex; align-items: center; gap: 12px">
        <n-switch
          :value="formData.model_redirect_strict"
          @update:value="(value: boolean) => updateFormData('model_redirect_strict', value)"
        />
        <span style="font-size: 14px; color: var(--text-hint)">
          {{
            formData.model_redirect_strict
              ? t("keys.modelRedirectStrictMode")
              : t("keys.modelRedirectLooseMode")
          }}
        </span>
      </div>
      <template #feedback>
        <div style="font-size: 12px; color: var(--text-placeholder); margin: 4px 0">
          <div v-if="formData.model_redirect_strict" style="color: var(--warning-color)">
            ⚠️ {{ t("keys.modelRedirectStrictWarning") }}
          </div>
          <div v-else style="color: var(--success-color)">
            ✅ {{ t("keys.modelRedirectLooseInfo") }}
          </div>
        </div>
      </template>
    </n-form-item>

    <n-form-item path="model_redirect_rules">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.modelRedirectRules") }}
          <n-tooltip trigger="hover" placement="top">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
            </template>
            {{ t("keys.modelRedirectRulesTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <n-input
        :value="formData.model_redirect_rules"
        type="textarea"
        :placeholder="modelRedirectTip"
        :rows="4"
        @update:value="(value: string) => updateFormData('model_redirect_rules', value)"
      />
      <template #feedback>
        <div style="font-size: 14px; color: var(--text-placeholder)">
          {{ t("keys.modelRedirectRulesDescription") }}
        </div>
      </template>
    </n-form-item>
  </div>

  <div v-if="showModelMappingStrict" class="config-section">
    <n-form-item path="model_mapping_strict">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("groups.modelMappingStrict") }}
          <n-tooltip trigger="hover" placement="top">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
            </template>
            {{ t("groups.modelMappingStrictTip") }}
          </n-tooltip>
        </div>
      </template>
      <div style="display: flex; align-items: center; gap: 12px">
        <n-switch
          :value="formData.model_mapping_strict"
          @update:value="(value: boolean) => updateFormData('model_mapping_strict', value)"
        />
        <span style="font-size: 14px; color: var(--text-hint)">
          {{
            formData.model_mapping_strict
              ? t("groups.modelMappingStrictEnabled")
              : t("groups.modelMappingStrictDisabled")
          }}
        </span>
      </div>
      <template #feedback>
        <div style="font-size: 12px; color: var(--text-placeholder); margin: 4px 0">
          <div v-if="formData.model_mapping_strict" style="color: var(--warning-color)">
            ⚠️ {{ t("groups.modelMappingStrictWarning") }}
          </div>
          <div v-else style="color: var(--success-color)">
            ✅ {{ t("groups.modelMappingLooseInfo") }}
          </div>
        </div>
      </template>
    </n-form-item>
  </div>

  <div class="config-section">
    <n-form-item path="param_overrides">
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.paramOverrides") }}
          <n-tooltip trigger="hover" placement="top">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
            </template>
            {{ t("keys.paramOverridesTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <n-input
        :value="formData.param_overrides"
        type="textarea"
        placeholder='{"temperature": 0.7}'
        :rows="4"
        @update:value="(value: string) => updateFormData('param_overrides', value)"
      />
    </n-form-item>
  </div>
</template>

<style scoped>
.config-section {
  margin-top: 16px;
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

.config-help {
  font-size: 13px;
}
</style>
