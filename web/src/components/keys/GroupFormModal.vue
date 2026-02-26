<script setup lang="ts">
import { keysApi } from "@/api/keys";
import { settingsApi } from "@/api/settings";
import GroupAdvancedConfig from "@/components/keys/GroupAdvancedConfig.vue";
import GroupBasicInfo from "@/components/keys/GroupBasicInfo.vue";
import GroupChannelConfig from "@/components/keys/GroupChannelConfig.vue";
import GroupConfigItems from "@/components/keys/GroupConfigItems.vue";
import GroupHeaderRules from "@/components/keys/GroupHeaderRules.vue";
import GroupUpstreamSection from "@/components/keys/GroupUpstreamSection.vue";
import ProxyKeysInput from "@/components/common/ProxyKeysInput.vue";
import type {
  ConfigItem,
  Group,
  GroupConfigOption,
  GroupFormData,
  HeaderRuleItem,
  UpstreamInfo,
} from "@/types/models";
import { Close, HelpCircleOutline } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NCollapse,
  NCollapseItem,
  NFormItem,
  NIcon,
  NModal,
  NTooltip,
  useMessage,
  type FormRules,
  type SelectOption,
} from "naive-ui";
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

defineOptions({
  name: "GroupFormModal",
});

interface Props {
  show: boolean;
  group?: Group | null;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success", value: Group): void;
  (e: "switchToGroup", groupId: number): void;
}

const props = withDefaults(defineProps<Props>(), {
  group: null,
});

const emit = defineEmits<Emits>();

const { t } = useI18n();
const message = useMessage();
const loading = ref(false);
const formRef = ref();
const modelRedirectTip = `{
  "gpt-5": "gpt-5-2025-08-07",
  "gemini-2.5-flash": "gemini-2.5-flash-preview-09-2025"
}`;

const formData = reactive<GroupFormData>({
  name: "",
  display_name: "",
  description: "",
  upstreams: [
    {
      url: "",
      weight: 1,
    },
  ],
  channel_type: "openai",
  sort: 1,
  test_model: "",
  validation_endpoint: "",
  param_overrides: "",
  model_redirect_rules: "",
  model_redirect_strict: false,
  model_mapping_strict: false,
  config: {},
  configItems: [] as ConfigItem[],
  header_rules: [] as HeaderRuleItem[],
  proxy_keys: "",
  group_type: "standard",
});

const channelTypeOptions = ref<SelectOption[]>([]);
const configOptions = ref<GroupConfigOption[]>([]);
const channelTypesFetched = ref(false);
const configOptionsFetched = ref(false);

const userModifiedFields = ref({
  test_model: false,
  upstream: false,
});

const testModelPlaceholder = computed(() => {
  switch (formData.channel_type) {
    case "openai":
    case "openai-responses":
      return "gpt-5-mini";
    case "gemini":
      return "gemini-2.5-flash-lite";
    case "anthropic":
      return "claude-haiku-4-5";
    default:
      return t("keys.enterModelName");
  }
});

const upstreamPlaceholder = computed(() => {
  switch (formData.channel_type) {
    case "openai":
    case "openai-responses":
      return "https://api.openai.com";
    case "gemini":
      return "https://generativelanguage.googleapis.com";
    case "anthropic":
      return "https://api.anthropic.com";
    default:
      return t("keys.enterUpstreamUrl");
  }
});

const validationEndpointPlaceholder = computed(() => {
  switch (formData.channel_type) {
    case "openai":
      return "/v1/chat/completions";
    case "openai-responses":
      return "/v1/responses";
    case "anthropic":
      return "/v1/messages";
    case "gemini":
      return "";
    default:
      return t("keys.enterValidationPath");
  }
});

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
  test_model: [
    {
      required: true,
      message: t("keys.enterTestModel"),
      trigger: ["blur", "input"],
    },
  ],
  upstreams: [
    {
      type: "array",
      min: 1,
      message: t("keys.atLeastOneUpstream"),
      trigger: ["blur", "change"],
    },
  ],
};

watch(
  () => props.show,
  show => {
    if (show) {
      if (!channelTypesFetched.value) {
        fetchChannelTypes();
      }
      if (!configOptionsFetched.value) {
        fetchGroupConfigOptions();
      }
      resetForm();
      if (props.group) {
        loadGroupData();
      }
    }
  }
);

watch(
  () => formData.channel_type,
  (_newChannelType, oldChannelType) => {
    if (!props.group && oldChannelType) {
      if (
        !userModifiedFields.value.test_model ||
        formData.test_model === getOldDefaultTestModel(oldChannelType)
      ) {
        formData.test_model = testModelPlaceholder.value;
        userModifiedFields.value.test_model = false;
      }

      if (
        formData.upstreams.length > 0 &&
        (!userModifiedFields.value.upstream ||
          formData.upstreams[0].url === getOldDefaultUpstream(oldChannelType))
      ) {
        formData.upstreams[0].url = upstreamPlaceholder.value;
        userModifiedFields.value.upstream = false;
      }
    }
  }
);

function getOldDefaultTestModel(channelType: string): string {
  switch (channelType) {
    case "openai":
    case "openai-responses":
      return "gpt-5-mini";
    case "gemini":
      return "gemini-2.5-flash-lite";
    case "anthropic":
      return "claude-haiku-4-5";
    default:
      return "";
  }
}

function getOldDefaultUpstream(channelType: string): string {
  switch (channelType) {
    case "openai":
    case "openai-responses":
      return "https://api.openai.com";
    case "gemini":
      return "https://generativelanguage.googleapis.com";
    case "anthropic":
      return "https://api.anthropic.com";
    default:
      return "";
  }
}

function resetForm() {
  const isCreateMode = !props.group;
  const defaultChannelType = "openai";

  formData.channel_type = defaultChannelType;

  Object.assign(formData, {
    name: "",
    display_name: "",
    description: "",
    upstreams: [
      {
        url: isCreateMode ? upstreamPlaceholder.value : "",
        weight: 1,
      },
    ],
    channel_type: defaultChannelType,
    sort: 1,
    test_model: isCreateMode ? testModelPlaceholder.value : "",
    validation_endpoint: "",
    param_overrides: "",
    model_redirect_rules: "",
    model_redirect_strict: false,
    config: {},
    configItems: [],
    header_rules: [],
    proxy_keys: "",
    group_type: "standard",
  });

  if (isCreateMode) {
    userModifiedFields.value = {
      test_model: false,
      upstream: false,
    };
  }
}

function loadGroupData() {
  if (!props.group) {
    return;
  }

  const configItems = Object.entries(props.group.config || {}).map(([key, value]) => {
    return {
      key,
      value,
    };
  });
  Object.assign(formData, {
    name: props.group.name || "",
    display_name: props.group.display_name || "",
    description: props.group.description || "",
    upstreams: props.group.upstreams?.length
      ? [...props.group.upstreams]
      : [{ url: "", weight: 1 }],
    channel_type: props.group.channel_type || "openai",
    sort: props.group.sort || 1,
    test_model: props.group.test_model || "",
    validation_endpoint: props.group.validation_endpoint || "",
    param_overrides: JSON.stringify(props.group.param_overrides || {}, null, 2),
    model_redirect_rules: JSON.stringify(props.group.model_redirect_rules || {}, null, 2),
    model_redirect_strict: props.group.model_redirect_strict || false,
    model_mapping_strict: props.group.model_mapping_strict || false,
    config: {},
    configItems,
    header_rules: (props.group.header_rules || []).map((rule: HeaderRuleItem) => ({
      key: rule.key || "",
      value: rule.value || "",
      action: (rule.action as "set" | "remove") || "set",
    })),
    proxy_keys: props.group.proxy_keys || "",
    group_type: props.group.group_type || "standard",
  });
}

async function fetchChannelTypes() {
  const options = (await settingsApi.getChannelTypes()) || [];
  channelTypeOptions.value =
    options?.map((type: string) => ({
      label: type,
      value: type,
    })) || [];
  channelTypesFetched.value = true;
}

function addUpstream() {
  formData.upstreams.push({
    url: "",
    weight: 1,
  });
}

function removeUpstream(index: number) {
  if (formData.upstreams.length > 1) {
    formData.upstreams.splice(index, 1);
  } else {
    message.warning(t("keys.atLeastOneUpstream"));
  }
}

async function fetchGroupConfigOptions() {
  const options = await keysApi.getGroupConfigOptions();
  configOptions.value = options || [];
  configOptionsFetched.value = true;
}

function addConfigItem() {
  formData.configItems.push({
    key: "",
    value: "",
  });
}

function removeConfigItem(index: number) {
  formData.configItems.splice(index, 1);
}

function handleConfigKeyChange(index: number, key: string) {
  const option = configOptions.value.find(opt => opt.key === key);
  if (option) {
    formData.configItems[index].value = option.default_value;
  }
}

function addHeaderRule() {
  formData.header_rules.push({
    key: "",
    value: "",
    action: "set",
  });
}

function removeHeaderRule(index: number) {
  formData.header_rules.splice(index, 1);
}

function handleClose() {
  emit("update:show", false);
}

async function handleSubmit() {
  if (loading.value) {
    return;
  }

  try {
    await formRef.value?.validate();

    loading.value = true;

    let paramOverrides = {};
    if (formData.param_overrides) {
      try {
        paramOverrides = JSON.parse(formData.param_overrides);
      } catch {
        message.error(t("keys.invalidJsonFormat"));
        return;
      }
    }

    let modelRedirectRules = {};
    if (formData.model_redirect_rules) {
      try {
        modelRedirectRules = JSON.parse(formData.model_redirect_rules);

        for (const [key, value] of Object.entries(modelRedirectRules)) {
          if (typeof key !== "string" || typeof value !== "string") {
            message.error(t("keys.modelRedirectInvalidFormat"));
            return;
          }
          if (key.trim() === "" || (value as string).trim() === "") {
            message.error(t("keys.modelRedirectEmptyModel"));
            return;
          }
        }
      } catch {
        message.error(t("keys.modelRedirectInvalidJson"));
        return;
      }
    }

    const config: Record<string, number | string | boolean> = {};
    formData.configItems.forEach((item: ConfigItem) => {
      if (item.key && item.key.trim()) {
        const option = configOptions.value.find(opt => opt.key === item.key);
        if (option && typeof option.default_value === "number" && typeof item.value === "string") {
          const numValue = Number(item.value);
          config[item.key] = isNaN(numValue) ? 0 : numValue;
        } else {
          config[item.key] = item.value;
        }
      }
    });

    const submitData = {
      name: formData.name,
      display_name: formData.display_name,
      description: formData.description,
      upstreams: formData.upstreams.filter((upstream: UpstreamInfo) => upstream.url.trim()),
      channel_type: formData.channel_type,
      sort: formData.sort,
      test_model: formData.test_model,
      validation_endpoint: formData.validation_endpoint,
      param_overrides: paramOverrides,
      model_redirect_rules: modelRedirectRules,
      model_redirect_strict: formData.model_redirect_strict,
      model_mapping_strict: formData.model_mapping_strict,
      config,
      header_rules: formData.header_rules
        .filter((rule: HeaderRuleItem) => rule.key.trim())
        .map((rule: HeaderRuleItem) => ({
          key: rule.key.trim(),
          value: rule.value,
          action: rule.action,
        })),
      proxy_keys: formData.proxy_keys,
    };

    let res: Group;
    if (props.group?.id) {
      res = await keysApi.updateGroup(props.group.id, submitData);
    } else {
      res = await keysApi.createGroup(submitData);
    }

    emit("success", res);
    if (!props.group?.id && res.id) {
      emit("switchToGroup", res.id);
    }
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
    class="group-form-modal modal-mask"
    :mask-closable="true"
    :closable="false"
  >
    <n-card
      class="group-form-card modal-card modal-wide"
      :title="group ? t('keys.editGroup') : t('keys.createGroup')"
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
        label-width="120px"
        require-mark-placement="right-hanging"
        class="group-form"
      >
        <group-basic-info
          :form-data="formData"
          @update:form-data="value => Object.assign(formData, value)"
        />

        <group-channel-config
          :form-data="formData"
          :channel-type-options="channelTypeOptions"
          :test-model-placeholder="testModelPlaceholder"
          :validation-endpoint-placeholder="validationEndpointPlaceholder"
          :is-edit-mode="!!group"
          @update:form-data="value => Object.assign(formData, value)"
          @fetch-channel-types="fetchChannelTypes"
          @test-model-modified="() => (userModifiedFields.test_model = true)"
        />

        <group-upstream-section
          :form-data="formData"
          :upstream-placeholder="upstreamPlaceholder"
          :is-edit-mode="!!group"
          @update:form-data="value => Object.assign(formData, value)"
          @add-upstream="addUpstream"
          @remove-upstream="removeUpstream"
          @upstream-modified="() => (userModifiedFields.upstream = true)"
        />

        <n-form-item :label="t('keys.proxyKeys')" path="proxy_keys">
          <template #label>
            <div class="form-label-with-tooltip">
              {{ t("keys.proxyKeys") }}
              <n-tooltip trigger="hover" placement="right-start" :show-arrow="true">
                <template #trigger>
                  <n-icon :component="HelpCircleOutline" class="help-icon" />
                </template>
                {{ t("keys.proxyKeysTooltip") }}
              </n-tooltip>
            </div>
          </template>
          <proxy-keys-input
            v-model="formData.proxy_keys"
            :placeholder="t('keys.multiKeysPlaceholder')"
            size="medium"
          />
        </n-form-item>

        <div class="form-section" style="margin-top: 10px">
          <n-collapse>
            <n-collapse-item name="advanced">
              <template #header>{{ t("keys.advancedConfig") }}</template>

              <group-config-items
                :form-data="formData"
                :config-options="configOptions"
                @update:form-data="value => Object.assign(formData, value)"
                @fetch-config-options="fetchGroupConfigOptions"
                @add-config-item="addConfigItem"
                @remove-config-item="removeConfigItem"
                @config-key-change="handleConfigKeyChange"
              />

              <group-header-rules
                :form-data="formData"
                @update:form-data="value => Object.assign(formData, value)"
                @add-header-rule="addHeaderRule"
                @remove-header-rule="removeHeaderRule"
              />

              <group-advanced-config
                :form-data="formData"
                :model-redirect-tip="modelRedirectTip"
                @update:form-data="value => Object.assign(formData, value)"
              />
            </n-collapse-item>
          </n-collapse>
        </div>
      </n-form>

      <template #footer>
        <div class="modal-footer">
          <n-button @click="handleClose" class="btn-cancel">{{ t("common.cancel") }}</n-button>
          <n-button
            @click="handleSubmit"
            :loading="loading"
            :class="group ? 'btn-update' : 'btn-create'"
          >
            {{ group ? t("common.update") : t("common.create") }}
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.form-section {
  margin-bottom: 24px;
}

:deep(.n-form-item-label) {
  font-weight: 500;
  color: var(--text-primary);
}

:deep(.n-form-item-blank) {
  flex-grow: 1;
}

:deep(.n-input) {
  --n-border-radius: 8px;
  --n-border: 1px solid var(--border-color);
  --n-border-hover: 1px solid var(--primary-color);
  --n-border-focus: 1px solid var(--primary-color);
  --n-box-shadow-focus: 0 0 0 2px var(--primary-color-suppl);
}

:deep(.n-select) {
  --n-border-radius: 8px;
}

:deep(.n-input-number) {
  --n-border-radius: 8px;
}

:deep(.n-button) {
  --n-border-radius: 8px;
}

:deep(.n-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 10px 20px;
}

:deep(.n-card__content) {
  max-height: calc(100vh - 68px - 61px - 50px);
  overflow-y: auto;
}

:deep(.n-form-item-feedback-wrapper) {
  min-height: 10px;
}

:deep(.n-tooltip__trigger) {
  display: inline-flex;
  align-items: center;
}

:deep(.n-tooltip) {
  --n-font-size: 13px;
  --n-border-radius: 8px;
}

:deep(.n-tooltip .n-tooltip__content) {
  max-width: 320px;
  line-height: 1.5;
}

:deep(.n-tooltip .n-tooltip__content div) {
  white-space: pre-line;
}

:deep(.n-collapse-item__header) {
  font-weight: 500;
  color: var(--text-primary);
}

:deep(.n-collapse-item) {
  --n-title-padding: 16px 0;
}

:deep(.n-base-selection-label) {
  height: 40px;
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

.btn-create {
  background: var(--btn-create-bg);
  color: white;
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
  color: white;
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

.btn-dashed {
  border: 1px dashed var(--border-color);
  color: var(--text-secondary);
  background: transparent;
}

.btn-dashed:hover:not(:disabled) {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.btn-icon-delete {
  background: var(--btn-delete-bg);
  color: white;
  border: none;
}

.btn-icon-delete:hover {
  background: var(--btn-delete-hover);
}

@media (max-width: 768px) {
  .group-form-card {
    width: 100vw !important;
  }

  .group-form {
    width: auto !important;
  }
}
</style>
