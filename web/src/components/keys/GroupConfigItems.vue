<script setup lang="ts">
import type { GroupConfigOption, GroupFormData } from "@/types/models";
import { Add, HelpCircleOutline, Remove } from "@vicons/ionicons5";
import {
  NButton,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NTooltip,
} from "naive-ui";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupConfigItems",
});

interface Props {
  formData: GroupFormData;
  configOptions: GroupConfigOption[];
}

interface Emits {
  (e: "update:formData", value: GroupFormData): void;
  (e: "fetchConfigOptions"): void;
  (e: "addConfigItem"): void;
  (e: "removeConfigItem", index: number): void;
  (e: "configKeyChange", index: number, key: string): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();

function updateFormData(key: keyof GroupFormData, value: unknown) {
  emit("update:formData", { ...props.formData, [key]: value as never });
}

function updateConfigItem(index: number, key: string, value: unknown) {
  const newConfigItems = [...props.formData.configItems];
  newConfigItems[index] = { ...newConfigItems[index], [key]: value as never };
  updateFormData("configItems", newConfigItems);
}

function getConfigOption(key: string) {
  return props.configOptions.find(opt => opt.key === key);
}

function handleConfigKeyChange(index: number, key: string) {
  emit("configKeyChange", index, key);
}
</script>

<template>
  <div class="config-section">
    <h5 class="config-title-with-tooltip">
      {{ t("keys.groupConfig") }}
      <n-tooltip trigger="hover" placement="bottom-start">
        <template #trigger>
          <n-icon :component="HelpCircleOutline" class="help-icon config-help" />
        </template>
        {{ t("keys.groupConfigTooltip") }}
      </n-tooltip>
    </h5>

    <div class="config-items">
      <n-form-item
        v-for="(configItem, index) in formData.configItems"
        :key="index"
        class="config-item-row"
        :label="`${t('keys.config')} ${index + 1}`"
        :path="`configItems[${index}].key`"
        :rule="{
          required: true,
          message: '',
          trigger: ['blur', 'change'],
        }"
      >
        <template #label>
          <div class="form-label-with-tooltip">
            {{ t("keys.config") }} {{ index + 1 }}
            <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
              <template #trigger>
                <n-icon :component="HelpCircleOutline" class="help-icon" />
              </template>
              {{ t("keys.configTooltip") }}
            </n-tooltip>
          </div>
        </template>
        <div class="config-item-content">
          <div class="config-select">
            <n-select
              :value="configItem.key"
              :options="
                configOptions.map(opt => ({
                  label: opt.name,
                  value: opt.key,
                  disabled:
                    formData.configItems.map((item: any) => item.key)?.includes(opt.key) &&
                    opt.key !== configItem.key,
                }))
              "
              :placeholder="t('keys.selectConfigParam')"
              @update:value="(value: string) => handleConfigKeyChange(index, value)"
              clearable
            />
          </div>
          <div class="config-value">
            <n-tooltip trigger="hover" placement="right">
              <template #trigger>
                <n-input-number
                  v-if="typeof configItem.value === 'number'"
                  :value="configItem.value"
                  :placeholder="t('keys.paramValue')"
                  :precision="0"
                  style="width: 100%"
                  @update:value="
                    (value: number | null) => updateConfigItem(index, 'value', value || 0)
                  "
                />
                <n-switch
                  v-else-if="typeof configItem.value === 'boolean'"
                  :value="configItem.value"
                  size="small"
                  @update:value="(value: boolean) => updateConfigItem(index, 'value', value)"
                />
                <n-input
                  v-else
                  :value="configItem.value"
                  :placeholder="t('keys.paramValue')"
                  @update:value="(value: string) => updateConfigItem(index, 'value', value)"
                />
              </template>
              {{ getConfigOption(configItem.key)?.description || t("keys.setConfigValue") }}
            </n-tooltip>
          </div>
          <div class="config-actions">
            <n-button
              @click="() => emit('removeConfigItem', index)"
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
      <n-button
        @click="() => emit('addConfigItem')"
        dashed
        class="btn-dashed"
        style="width: 100%"
        :disabled="formData.configItems.length >= configOptions.length"
      >
        <template #icon>
          <n-icon :component="Add" />
        </template>
        {{ t("keys.addConfigParam") }}
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

.config-item-row {
  margin-bottom: 12px;
}

.config-item-content {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.config-select {
  flex: 0 0 200px;
}

.config-value {
  flex: 1;
}

.config-actions {
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
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
  .config-item-content {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }

  .config-value {
    flex: 1;
  }

  .config-actions {
    justify-content: flex-end;
  }
}
</style>
