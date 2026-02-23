<script setup lang="ts">
import { keysApi } from "@/api/keys";
import ProxyKeysInput from "@/components/common/ProxyKeysInput.vue";
import { type ChannelType, type Group } from "@/types/models";
import { Close, HelpCircleOutline } from "@vicons/ionicons5";
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
  NSwitch,
  NTooltip,
  useMessage,
  type FormRules,
  type SelectOption,
} from "naive-ui";
import { reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  group?: Group | null;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success", value: Group): void;
}

const props = withDefaults(defineProps<Props>(), {
  group: null,
});

const emit = defineEmits<Emits>();

const { t } = useI18n();
const message = useMessage();
const loading = ref(false);
const formRef = ref();

// 渠道类型选项
const channelTypeOptions: SelectOption[] = [
  { label: "OpenAI (Chat Completions)", value: "openai" },
  { label: "OpenAI (Responses API)", value: "openai-responses" },
  { label: "Gemini", value: "gemini" },
  { label: "Anthropic", value: "anthropic" },
];

// 默认表单数据
const defaultFormData = {
  name: "",
  display_name: "",
  description: "",
  channel_type: "openai" as ChannelType,
  sort: 1,
  proxy_keys: "",
  model_mapping_strict: false,
};

// 表单数据
const formData = reactive({ ...defaultFormData });

// 表单验证规则
const rules: FormRules = {
  name: [
    {
      required: true,
      message: t("keys.enterGroupName"),
      trigger: ["blur", "input"],
    },
    {
      pattern: /^[a-z0-9_-]{1,100}$/,
      message: t("keys.groupNamePattern"),
      trigger: ["blur", "input"],
    },
  ],
  channel_type: [
    {
      required: true,
      message: t("keys.selectChannelType"),
      trigger: ["blur", "change"],
    },
  ],
};

// 监听弹窗显示状态
watch(
  () => props.show,
  show => {
    if (show) {
      // 新建模式重置表单，编辑模式加载数据
      if (props.group) {
        loadGroupData();
      } else {
        resetForm();
      }
    }
  }
);

// 重置表单
function resetForm() {
  Object.assign(formData, defaultFormData);
}

// 加载分组数据（编辑模式）
function loadGroupData() {
  if (!props.group) {
    return;
  }

  Object.assign(formData, {
    name: props.group.name || "",
    display_name: props.group.display_name || "",
    description: props.group.description || "",
    channel_type: props.group.channel_type || "openai",
    sort: props.group.sort || 1,
    proxy_keys: props.group.proxy_keys || "",
    model_mapping_strict: props.group.model_mapping_strict || false,
  });
}

// 关闭弹窗
function handleClose() {
  emit("update:show", false);
}

// 提交表单
async function handleSubmit() {
  if (loading.value) {
    return;
  }

  try {
    await formRef.value?.validate();

    loading.value = true;

    // 构建提交数据
    const submitData = {
      name: formData.name,
      display_name: formData.display_name,
      description: formData.description,
      channel_type: formData.channel_type,
      sort: formData.sort,
      proxy_keys: formData.proxy_keys,
      group_type: "aggregate" as const,
      model_mapping_strict: formData.model_mapping_strict,
    };

    let result: Group;
    if (props.group) {
      // 编辑模式
      if (!props.group?.id) {
        message.error(t("keys.invalidGroup"));
        return;
      }
      result = await keysApi.updateGroup(props.group.id, submitData);
    } else {
      // 新建模式
      result = await keysApi.createGroup(submitData);
    }

    emit("success", result);
    handleClose();
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <n-modal
    :show="show"
    @update:show="handleClose"
    class="aggregate-group-modal modal-mask"
    :mask-closable="true"
    :closable="false"
  >
    <n-card
      class="aggregate-group-card modal-card modal-aggregate"
      :title="group ? t('keys.editAggregateGroup') : t('keys.createAggregateGroup')"
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
        label-width="160px"
      >
        <!-- 基础信息 -->
        <div class="form-section">
          <h4 class="section-title">{{ t("keys.basicInfo") }}</h4>

          <n-form-item :label="t('keys.groupName')" path="name">
            <n-input
              v-model:value="formData.name"
              :placeholder="t('keys.groupNamePlaceholder')"
              clearable
            />
          </n-form-item>

          <n-form-item :label="t('keys.displayName')">
            <n-input
              v-model:value="formData.display_name"
              :placeholder="t('keys.displayNamePlaceholder')"
              clearable
            />
          </n-form-item>

          <n-form-item :label="t('keys.channelType')" path="channel_type">
            <n-select
              v-model:value="formData.channel_type"
              :options="channelTypeOptions"
              :placeholder="t('keys.selectChannelType')"
              :disabled="!!props.group"
            />
          </n-form-item>

          <n-form-item :label="t('keys.sortOrder')">
            <n-input-number
              v-model:value="formData.sort"
              :placeholder="t('keys.sortValue')"
              style="width: 100%"
            />
          </n-form-item>

          <n-form-item :label="t('keys.proxyKeys')">
            <proxy-keys-input v-model="formData.proxy_keys" />
          </n-form-item>

          <n-form-item :label="t('common.description')">
            <n-input
              v-model:value="formData.description"
              type="textarea"
              placeholder=""
              :rows="1"
              :autosize="{ minRows: 1, maxRows: 5 }"
              style="resize: none"
            />
          </n-form-item>

          <n-form-item path="model_mapping_strict">
            <template #label>
              <div style="display: flex; align-items: center; gap: 4px">
                {{ t("groups.modelMappingStrict") }}
                <n-tooltip trigger="hover" placement="top">
                  <template #trigger>
                    <n-icon :component="HelpCircleOutline" style="cursor: help" />
                  </template>
                  {{ t("groups.modelMappingStrictTip") }}
                </n-tooltip>
              </div>
            </template>
            <div style="display: flex; align-items: center; gap: 12px">
              <n-switch v-model:value="formData.model_mapping_strict" />
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
      </n-form>

      <template #footer>
        <div class="modal-footer">
          <n-button @click="handleClose" class="btn-cancel">{{ t("common.cancel") }}</n-button>
          <n-button
            :class="group ? 'btn-update' : 'btn-create'"
            @click="handleSubmit"
            :loading="loading"
          >
            {{ group ? t("common.update") : t("common.create") }}
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.aggregate-group-modal {
  /* 继承 modal-mask 样式 */
}

.form-section {
  margin-bottom: 24px;
}

.form-section:first-child {
  margin-bottom: 24px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--n-text-color);
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--n-border-color);
}

.btn-create {
  background: var(--btn-create-bg);
  color: #fff;
  border: none;
  transition: all 0.2s;
}

.btn-create:hover {
  background: var(--btn-create-hover);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
}

.btn-edit {
  background: var(--btn-edit-bg);
  color: #fff;
  border: none;
  transition: all 0.2s;
}

.btn-edit:hover {
  background: var(--btn-edit-hover);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.3);
}

.btn-view {
  background: var(--btn-view-bg);
  color: var(--btn-view-color);
  border: 1px solid var(--btn-view-border);
  transition: all 0.2s;
}

.btn-view:hover {
  background: var(--btn-view-hover);
  border-color: var(--btn-view-color);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
}
</style>
