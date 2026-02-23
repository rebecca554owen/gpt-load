<script setup lang="ts">
import type { GroupFormData, HeaderRuleItem } from "@/types/models";
import { Add, HelpCircleOutline, Remove } from "@vicons/ionicons5";
import { NButton, NFormItem, NIcon, NInput, NSwitch, NTooltip } from "naive-ui";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupHeaderRules",
});

interface Props {
  formData: GroupFormData;
}

interface Emits {
  (e: "update:formData", value: GroupFormData): void;
  (e: "addHeaderRule"): void;
  (e: "removeHeaderRule", index: number): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();

function updateFormData(key: keyof GroupFormData, value: unknown) {
  emit("update:formData", { ...props.formData, [key]: value as never });
}

function updateHeaderRule(index: number, key: string, value: unknown) {
  const newHeaderRules = [...props.formData.header_rules];
  newHeaderRules[index] = { ...newHeaderRules[index], [key]: value as never };
  updateFormData("header_rules", newHeaderRules);
}

function canonicalHeaderKey(key: string): string {
  if (!key) {
    return key;
  }
  return key
    .split("-")
    .map(part => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join("-");
}

function validateHeaderKeyUniqueness(
  rules: HeaderRuleItem[],
  currentIndex: number,
  key: string
): boolean {
  if (!key.trim()) {
    return true;
  }

  const canonicalKey = canonicalHeaderKey(key.trim());
  return !rules.some(
    (rule, index) => index !== currentIndex && canonicalHeaderKey(rule.key.trim()) === canonicalKey
  );
}
</script>

<template>
  <div class="config-section">
    <h5 class="config-title-with-tooltip">
      {{ t("keys.customHeaders") }}
      <n-tooltip trigger="hover" placement="top">
        <template #trigger>
          <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
        </template>
        <div>
          {{ t("keys.headerRulesTooltip1") }}
          <br />
          {{ t("keys.supportedVariables") }}：
          <br />
          • ${CLIENT_IP} - {{ t("keys.clientIpVar") }}
          <br />
          • ${GROUP_NAME} - {{ t("keys.groupNameVar") }}
          <br />
          • ${API_KEY} - {{ t("keys.apiKeyVar") }}
          <br />
          • ${TIMESTAMP_MS} - {{ t("keys.timestampMsVar") }}
          <br />
          • ${TIMESTAMP_S} - {{ t("keys.timestampSVar") }}
        </div>
      </n-tooltip>
    </h5>

    <div class="header-rules-items">
      <n-form-item
        v-for="(headerRule, index) in formData.header_rules"
        :key="index"
        class="header-rule-row"
        :label="`${t('keys.header')} ${index + 1}`"
      >
        <template #label>
          <div class="form-label-with-tooltip">
            {{ t("keys.header") }} {{ index + 1 }}
            <n-tooltip trigger="hover" placement="top">
              <template #trigger>
                <n-icon :component="HelpCircleOutline" class="help-icon" />
              </template>
              {{ t("keys.headerTooltip") }}
            </n-tooltip>
          </div>
        </template>
        <div class="header-rule-content">
          <div class="header-name">
            <n-input
              :value="headerRule.key"
              :placeholder="t('keys.headerName')"
              :status="
                !validateHeaderKeyUniqueness(formData.header_rules, index, headerRule.key)
                  ? 'error'
                  : undefined
              "
              @update:value="(value: string) => updateHeaderRule(index, 'key', value)"
            />
            <div
              v-if="!validateHeaderKeyUniqueness(formData.header_rules, index, headerRule.key)"
              class="error-message"
            >
              {{ t("keys.duplicateHeader") }}
            </div>
          </div>
          <div class="header-value" v-if="headerRule.action === 'set'">
            <n-input
              :value="headerRule.value"
              :placeholder="t('keys.headerValuePlaceholder')"
              @update:value="(value: string) => updateHeaderRule(index, 'value', value)"
            />
          </div>
          <div class="header-value removed-placeholder" v-else>
            <span class="removed-text">{{ t("keys.willRemoveFromRequest") }}</span>
          </div>
          <div class="header-action">
            <n-tooltip trigger="hover" placement="top">
              <template #trigger>
                <n-switch
                  :value="headerRule.action"
                  :checked-value="'remove'"
                  :unchecked-value="'set'"
                  size="small"
                  @update:value="
                    (value: 'set' | 'remove') => updateHeaderRule(index, 'action', value)
                  "
                />
              </template>
              {{ t("keys.removeToggleTooltip") }}
            </n-tooltip>
          </div>
          <div class="header-actions">
            <n-button
              @click="() => emit('removeHeaderRule', index)"
              class="btn-delete"
              circle
              size="small"
            >
              <template #icon>
                <n-icon :component="Remove" />
              </template>
            </n-button>
          </div>
        </div>
      </n-form-item>
    </div>

    <div style="margin-top: 12px; padding-left: 120px">
      <n-button @click="() => emit('addHeaderRule')" dashed class="btn-dashed" style="width: 100%">
        <template #icon>
          <n-icon :component="Add" />
        </template>
        {{ t("keys.addHeader") }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.config-section {
  margin-top: 16px;
}

.config-title-with-tooltip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 12px 0;
}

.config-help {
  font-size: 13px;
}

.header-rule-row {
  margin-bottom: 12px;
}

.header-rule-content {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
}

.header-name {
  flex: 0 0 200px;
  position: relative;
}

.header-value {
  flex: 1;
  display: flex;
  align-items: center;
  min-height: 34px;
}

.header-value.removed-placeholder {
  justify-content: center;
  background-color: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 0 12px;
}

.removed-text {
  color: var(--text-tertiary);
  font-style: italic;
  font-size: 13px;
}

.header-action {
  flex: 0 0 50px;
  display: flex;
  justify-content: center;
  align-items: center;
  height: 34px;
}

.header-actions {
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  height: 34px;
}

.error-message {
  position: absolute;
  top: 100%;
  left: 0;
  font-size: 12px;
  color: var(--error-color);
  margin-top: 2px;
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

.btn-dashed {
  border: 1px dashed var(--border-color);
  color: var(--text-secondary);
  background: transparent;
}

.btn-dashed:hover:not(:disabled) {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

@media (max-width: 768px) {
  .header-rule-content {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }

  .header-name,
  .header-value {
    flex: 1;
  }

  .header-actions {
    justify-content: flex-end;
  }
}
</style>
