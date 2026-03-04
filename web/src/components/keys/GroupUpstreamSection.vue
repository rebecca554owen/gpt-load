<script setup lang="ts">
import type { GroupFormData } from "@/types/models";
import { Add, HelpCircleOutline, Remove } from "@vicons/ionicons5";
import { NButton, NFormItem, NIcon, NInput, NInputNumber, NTooltip, useMessage } from "naive-ui";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupUpstreamSection",
});

interface Props {
  formData: GroupFormData;
  upstreamPlaceholder: string;
  isEditMode: boolean;
}

interface Emits {
  (e: "update:formData", value: GroupFormData): void;
  (e: "addUpstream"): void;
  (e: "removeUpstream", index: number): void;
  (e: "upstreamModified"): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();
const message = useMessage();

function updateUpstream(index: number, key: string, value: unknown) {
  const newUpstreams = [...props.formData.upstreams];
  newUpstreams[index] = { ...newUpstreams[index], [key]: value as never };
  emit("update:formData", { ...props.formData, upstreams: newUpstreams });
}

function handleUpstreamInputChange(index: number, value: string) {
  updateUpstream(index, "url", value);
  if (!props.isEditMode && index === 0) {
    emit("upstreamModified");
  }
}

function handleAddUpstream() {
  emit("addUpstream");
}

function handleRemoveUpstream(index: number) {
  if (props.formData.upstreams.length > 1) {
    emit("removeUpstream", index);
  } else {
    message.warning(t("keys.atLeastOneUpstream"));
  }
}
</script>

<template>
  <div class="form-section" style="margin-top: 10px">
    <h4 class="section-title">{{ t("keys.upstreamAddresses") }}</h4>
    <n-form-item
      v-for="(upstream, index) in formData.upstreams"
      :key="index"
      :label="`${t('keys.upstream')} ${index + 1}`"
      :path="`upstreams[${index}].url`"
      :rule="{
        required: true,
        message: '',
        trigger: ['blur', 'input'],
      }"
    >
      <template #label>
        <div class="form-label-with-tooltip">
          {{ t("keys.upstream") }} {{ index + 1 }}
          <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
            <template #trigger>
              <n-icon :component="HelpCircleOutline" class="help-icon" />
            </template>
            {{ t("keys.upstreamTooltip") }}
          </n-tooltip>
        </div>
      </template>
      <div class="upstream-row">
        <div class="upstream-url">
          <n-input
            :value="upstream.url"
            :placeholder="upstreamPlaceholder"
            autocomplete="off"
            @update:value="(value: string) => handleUpstreamInputChange(index, value)"
          />
        </div>
        <div class="upstream-weight">
          <span class="weight-label">{{ t("keys.weight") }}</span>
          <n-tooltip trigger="hover" placement="right" style="width: 100%">
            <template #trigger>
              <n-input-number
                :value="upstream.weight"
                :min="0"
                :placeholder="t('keys.weight')"
                style="width: 100%"
                @update:value="
                  (value: number | null) => updateUpstream(index, 'weight', value || 0)
                "
              />
            </template>
            {{ t("keys.weightTooltip") }}
          </n-tooltip>
        </div>
        <div class="upstream-actions">
          <n-button
            v-if="formData.upstreams.length > 1"
            @click="handleRemoveUpstream(index)"
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

    <n-form-item>
      <n-button @click="handleAddUpstream" dashed class="btn-dashed" style="width: 100%">
        <template #icon>
          <n-icon :component="Add" />
        </template>
        {{ t("keys.addUpstream") }}
      </n-button>
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

.upstream-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.upstream-url {
  flex: 1;
}

.upstream-weight {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 140px;
}

.weight-label {
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
}

.upstream-actions {
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
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
  .section-title {
    font-size: 0.9rem;
  }

  .upstream-row {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }

  .upstream-weight {
    flex: 1;
    flex-direction: column;
    align-items: flex-start;
  }

  .upstream-actions {
    justify-content: flex-end;
  }
}
</style>
