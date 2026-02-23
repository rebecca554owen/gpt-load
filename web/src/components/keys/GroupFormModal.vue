<script setup lang="ts">
import { keysApi } from "@/api/keys";

defineOptions({
  name: "GroupFormModal",
});

import { settingsApi } from "@/api/settings";
import ProxyKeysInput from "@/components/common/ProxyKeysInput.vue";
import type { Group, GroupConfigOption, UpstreamInfo } from "@/types/models";
import { Add, Close, HelpCircleOutline, Remove } from "@vicons/ionicons5";
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
} from "naive-ui";
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  group?: Group | null;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success", value: Group): void;
  (e: "switchToGroup", groupId: number): void;
}

// Config item type
interface ConfigItem {
  key: string;
  value: number | string | boolean;
}

// Header rule type
interface HeaderRuleItem {
  key: string;
  value: string;
  action: "set" | "remove";
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

// Form data interface
interface GroupFormData {
  name: string;
  display_name: string;
  description: string;
  upstreams: UpstreamInfo[];
  channel_type: "anthropic" | "gemini" | "openai" | "openai-responses";
  sort: number;
  test_model: string;
  validation_endpoint: string;
  param_overrides: string;
  model_redirect_rules: string;
  model_redirect_strict: boolean;
  model_mapping_strict: boolean;
  config: Record<string, number | string | boolean>;
  configItems: ConfigItem[];
  header_rules: HeaderRuleItem[];
  proxy_keys: string;
  group_type?: string;
}

// Form data
const formData = reactive<GroupFormData>({
  name: "",
  display_name: "",
  description: "",
  upstreams: [
    {
      url: "",
      weight: 1,
    },
  ] as UpstreamInfo[],
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

const channelTypeOptions = ref<{ label: string; value: string }[]>([]);
const configOptions = ref<GroupConfigOption[]>([]);
const channelTypesFetched = ref(false);
const configOptionsFetched = ref(false);

// Track if user has manually modified fields (only used in create mode)
const userModifiedFields = ref({
  test_model: false,
  upstream: false,
});

// Generate placeholder hints dynamically based on channel type
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
      return ""; // Gemini doesn't show this field
    default:
      return t("keys.enterValidationPath");
  }
});

// Form validation rules
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

// Watch modal display state
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

// Watch channel type changes, smart update default values in create mode
watch(
  () => formData.channel_type,
  (_newChannelType, oldChannelType) => {
    if (!props.group && oldChannelType) {
      // Only process in create mode and not during initial setup
      // Check if test model should update (empty or old channel type's default)
      if (
        !userModifiedFields.value.test_model ||
        formData.test_model === getOldDefaultTestModel(oldChannelType)
      ) {
        formData.test_model = testModelPlaceholder.value;
        userModifiedFields.value.test_model = false;
      }

      // Check if first upstream address should update
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

// Get default value for old channel type (for comparison)
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

// Reset form
function resetForm() {
  const isCreateMode = !props.group;
  const defaultChannelType = "openai";

  // Set channel type first so computed properties calculate default values correctly
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

  // Reset user modification status tracking
  if (isCreateMode) {
    userModifiedFields.value = {
      test_model: false,
      upstream: false,
    };
  }
}

// Load group data (edit mode)
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

// Add upstream address
function addUpstream() {
  formData.upstreams.push({
    url: "",
    weight: 1,
  });
}

// Remove upstream address
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

// Add config item
function addConfigItem() {
  formData.configItems.push({
    key: "",
    value: "",
  });
}

// Remove config item
function removeConfigItem(index: number) {
  formData.configItems.splice(index, 1);
}

// Add header rule
function addHeaderRule() {
  formData.header_rules.push({
    key: "",
    value: "",
    action: "set",
  });
}

// Remove header rule
function removeHeaderRule(index: number) {
  formData.header_rules.splice(index, 1);
}

// Normalize header key to canonical format (simulate HTTP standard)
function canonicalHeaderKey(key: string): string {
  if (!key) {
    return key;
  }
  return key
    .split("-")
    .map(part => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join("-");
}

// Validate header key uniqueness (use canonical format comparison)
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

// Set default value when config item's key changes
function handleConfigKeyChange(index: number, key: string) {
  const option = configOptions.value.find(opt => opt.key === key);
  if (option) {
    formData.configItems[index].value = option.default_value;
  }
}

const getConfigOption = (key: string) => {
  return configOptions.value.find(opt => opt.key === key);
};

// Close modal
function handleClose() {
  emit("update:show", false);
}

// Submit form
async function handleSubmit() {
  if (loading.value) {
    return;
  }

  try {
    await formRef.value?.validate();

    loading.value = true;

    // Validate JSON format
    let paramOverrides = {};
    if (formData.param_overrides) {
      try {
        paramOverrides = JSON.parse(formData.param_overrides);
      } catch {
        message.error(t("keys.invalidJsonFormat"));
        return;
      }
    }

    // Validate model redirect rules JSON format
    let modelRedirectRules = {};
    if (formData.model_redirect_rules) {
      try {
        modelRedirectRules = JSON.parse(formData.model_redirect_rules);

        // Validate rule format
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

    // Convert configItems to config object
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

    // Build submit data
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
      // Edit mode
      res = await keysApi.updateGroup(props.group.id, submitData);
    } else {
      // Create mode
      res = await keysApi.createGroup(submitData);
    }

    emit("success", res);
    // 如果是新建模式，发出切换到新分组的事件
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
        <!-- Basic info -->
        <div class="form-section">
          <h4 class="section-title">{{ t("keys.basicInfo") }}</h4>

          <!-- Group name and display name on the same row -->
          <div class="form-row">
            <n-form-item :label="t('keys.groupName')" path="name" class="form-item-half">
              <template #label>
                <div class="form-label-with-tooltip">
                  {{ t("keys.groupName") }}
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    {{ t("keys.groupNameTooltip") }}
                  </n-tooltip>
                </div>
              </template>
              <n-input v-model:value="formData.name" autocomplete="off" placeholder="gemini" />
            </n-form-item>

            <n-form-item :label="t('keys.displayName')" path="display_name" class="form-item-half">
              <template #label>
                <div class="form-label-with-tooltip">
                  {{ t("keys.displayName") }}
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    {{ t("keys.displayNameTooltip") }}
                  </n-tooltip>
                </div>
              </template>
              <n-input
                v-model:value="formData.display_name"
                autocomplete="off"
                placeholder="Google Gemini"
              />
            </n-form-item>
          </div>

          <!-- Channel type and sort order on the same row -->
          <div class="form-row">
            <n-form-item :label="t('keys.channelType')" path="channel_type" class="form-item-half">
              <template #label>
                <div class="form-label-with-tooltip">
                  {{ t("keys.channelType") }}
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    {{ t("keys.channelTypeTooltip") }}
                  </n-tooltip>
                </div>
              </template>
              <n-select
                v-model:value="formData.channel_type"
                :options="channelTypeOptions"
                :placeholder="t('keys.selectChannelType')"
              />
            </n-form-item>

            <n-form-item :label="t('keys.sortOrder')" path="sort" class="form-item-half">
              <template #label>
                <div class="form-label-with-tooltip">
                  {{ t("keys.sortOrder") }}
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    {{ t("keys.sortOrderTooltip") }}
                  </n-tooltip>
                </div>
              </template>
              <n-input-number
                v-model:value="formData.sort"
                :min="0"
                :placeholder="t('keys.sortValue')"
                style="width: 100%"
              />
            </n-form-item>
          </div>

          <!-- Test model and test path on the same row -->
          <div class="form-row">
            <n-form-item :label="t('keys.testModel')" path="test_model" class="form-item-half">
              <template #label>
                <div class="form-label-with-tooltip">
                  {{ t("keys.testModel") }}
                  <n-tooltip trigger="hover" placement="top">
                    <template #trigger>
                      <n-icon :component="HelpCircleOutline" class="help-icon" />
                    </template>
                    {{ t("keys.testModelTooltip") }}
                  </n-tooltip>
                </div>
              </template>
              <n-input
                v-model:value="formData.test_model"
                :placeholder="testModelPlaceholder"
                autocomplete="off"
                @input="() => !props.group && (userModifiedFields.test_model = true)"
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
                  <n-tooltip trigger="hover" placement="top">
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
                v-model:value="formData.validation_endpoint"
                :placeholder="
                  validationEndpointPlaceholder || t('keys.optionalCustomValidationPath')
                "
                autocomplete="off"
              />
            </n-form-item>

            <!-- When gemini channel, test path is hidden, need placeholder div to keep layout -->
            <div v-else class="form-item-half" />
          </div>

          <!-- Proxy keys -->
          <n-form-item :label="t('keys.proxyKeys')" path="proxy_keys">
            <template #label>
              <div class="form-label-with-tooltip">
                {{ t("keys.proxyKeys") }}
                <n-tooltip trigger="hover" placement="top">
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

          <!-- Description takes full row -->
          <n-form-item :label="t('common.description')" path="description">
            <template #label>
              <div class="form-label-with-tooltip">
                {{ t("common.description") }}
                <n-tooltip trigger="hover" placement="top">
                  <template #trigger>
                    <n-icon :component="HelpCircleOutline" class="help-icon" />
                  </template>
                  {{ t("keys.descriptionTooltip") }}
                </n-tooltip>
              </div>
            </template>
            <n-input
              v-model:value="formData.description"
              type="textarea"
              placeholder=""
              :rows="1"
              :autosize="{ minRows: 1, maxRows: 5 }"
              style="resize: none"
            />
          </n-form-item>
        </div>

        <!-- Upstream addresses -->
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
                <n-tooltip trigger="hover" placement="top">
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
                  v-model:value="upstream.url"
                  :placeholder="upstreamPlaceholder"
                  autocomplete="off"
                  @input="() => !props.group && index === 0 && (userModifiedFields.upstream = true)"
                />
              </div>
              <div class="upstream-weight">
                <span class="weight-label">{{ t("keys.weight") }}</span>
                <n-tooltip trigger="hover" placement="top" style="width: 100%">
                  <template #trigger>
                    <n-input-number
                      v-model:value="upstream.weight"
                      :min="0"
                      :placeholder="t('keys.weight')"
                      style="width: 100%"
                    />
                  </template>
                  {{ t("keys.weightTooltip") }}
                </n-tooltip>
              </div>
              <div class="upstream-actions">
                <n-button
                  v-if="formData.upstreams.length > 1"
                  @click="removeUpstream(index)"
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
            <n-button @click="addUpstream" dashed class="btn-dashed" style="width: 100%">
              <template #icon>
                <n-icon :component="Add" />
              </template>
              {{ t("keys.addUpstream") }}
            </n-button>
          </n-form-item>
        </div>

        <!-- Advanced configuration -->
        <div class="form-section" style="margin-top: 10px">
          <n-collapse>
            <n-collapse-item name="advanced">
              <template #header>{{ t("keys.advancedConfig") }}</template>
              <div class="config-section">
                <h5 class="config-title-with-tooltip">
                  {{ t("keys.groupConfig") }}
                  <n-tooltip trigger="hover" placement="top">
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
                        <n-tooltip trigger="hover" placement="top">
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
                          v-model:value="configItem.key"
                          :options="
                            configOptions.map(opt => ({
                              label: opt.name,
                              value: opt.key,
                              disabled:
                                formData.configItems
                                  .map((item: ConfigItem) => item.key)
                                  ?.includes(opt.key) && opt.key !== configItem.key,
                            }))
                          "
                          :placeholder="t('keys.selectConfigParam')"
                          @update:value="value => handleConfigKeyChange(index, value)"
                          clearable
                        />
                      </div>
                      <div class="config-value">
                        <n-tooltip trigger="hover" placement="top">
                          <template #trigger>
                            <n-input-number
                              v-if="typeof configItem.value === 'number'"
                              v-model:value="configItem.value"
                              :placeholder="t('keys.paramValue')"
                              :precision="0"
                              style="width: 100%"
                            />
                            <n-switch
                              v-else-if="typeof configItem.value === 'boolean'"
                              v-model:value="configItem.value"
                              size="small"
                            />
                            <n-input
                              v-else
                              v-model:value="configItem.value"
                              :placeholder="t('keys.paramValue')"
                            />
                          </template>
                          {{
                            getConfigOption(configItem.key)?.description || t("keys.setConfigValue")
                          }}
                        </n-tooltip>
                      </div>
                      <div class="config-actions">
                        <n-button
                          @click="removeConfigItem(index)"
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
                    @click="addConfigItem"
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
                          v-model:value="headerRule.key"
                          :placeholder="t('keys.headerName')"
                          :status="
                            !validateHeaderKeyUniqueness(
                              formData.header_rules,
                              index,
                              headerRule.key
                            )
                              ? 'error'
                              : undefined
                          "
                        />
                        <div
                          v-if="
                            !validateHeaderKeyUniqueness(
                              formData.header_rules,
                              index,
                              headerRule.key
                            )
                          "
                          class="error-message"
                        >
                          {{ t("keys.duplicateHeader") }}
                        </div>
                      </div>
                      <div class="header-value" v-if="headerRule.action === 'set'">
                        <n-input
                          v-model:value="headerRule.value"
                          :placeholder="t('keys.headerValuePlaceholder')"
                        />
                      </div>
                      <div class="header-value removed-placeholder" v-else>
                        <span class="removed-text">{{ t("keys.willRemoveFromRequest") }}</span>
                      </div>
                      <div class="header-action">
                        <n-tooltip trigger="hover" placement="top">
                          <template #trigger>
                            <n-switch
                              v-model:value="headerRule.action"
                              :checked-value="'remove'"
                              :unchecked-value="'set'"
                              size="small"
                            />
                          </template>
                          {{ t("keys.removeToggleTooltip") }}
                        </n-tooltip>
                      </div>
                      <div class="header-actions">
                        <n-button
                          @click="removeHeaderRule(index)"
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
                  <n-button @click="addHeaderRule" dashed class="btn-dashed" style="width: 100%">
                    <template #icon>
                      <n-icon :component="Add" />
                    </template>
                    {{ t("keys.addHeader") }}
                  </n-button>
                </div>
              </div>

              <!-- 模型重定向配置 -->
              <div v-if="formData.group_type !== 'aggregate'" class="config-section">
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
                    <n-switch v-model:value="formData.model_redirect_strict" />
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
                      <div
                        v-if="formData.model_redirect_strict"
                        style="color: var(--warning-color)"
                      >
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
                    v-model:value="formData.model_redirect_rules"
                    type="textarea"
                    :placeholder="modelRedirectTip"
                    :rows="4"
                  />
                  <template #feedback>
                    <div style="font-size: 14px; color: var(--text-placeholder)">
                      {{ t("keys.modelRedirectRulesDescription") }}
                    </div>
                  </template>
                </n-form-item>
              </div>

              <!-- 模型映射严格模式配置 (仅聚合组) -->
              <div v-if="formData.group_type === 'aggregate'" class="config-section">
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
                    v-model:value="formData.param_overrides"
                    type="textarea"
                    placeholder='{"temperature": 0.7}'
                    :rows="4"
                  />
                </n-form-item>
              </div>
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
.group-form-modal {
  /* 继承 modal-mask 样式 */
}

.group-form-card {
  /* 继承 modal-card 和 modal-wide 样式 */
}

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

:deep(.n-input) {
  --n-border-radius: 6px;
}

:deep(.n-select) {
  --n-border-radius: 6px;
}

:deep(.n-input-number) {
  --n-border-radius: 6px;
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

.config-section {
  margin-top: 16px;
}

.config-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 12px 0;
}

.form-label {
  margin-left: 25px;
  margin-right: 10px;
  height: 34px;
  line-height: 34px;
  font-weight: 500;
}

/* Tooltip相关样式 */
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

.section-title-with-tooltip {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.section-help {
  font-size: 16px;
}

.collapse-header-with-tooltip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

.collapse-help {
  font-size: 14px;
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

/* 增强表单样式 */
:deep(.n-form-item-label) {
  font-weight: 500;
  color: var(--text-primary);
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

/* 美化tooltip */
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

/* 折叠面板样式优化 */
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

/* 表单行布局 */
.form-row {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

.form-item-half {
  flex: 1;
  width: 50%;
}

/* 上游地址行布局 */
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

/* 配置项行布局 */
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

@media (max-width: 768px) {
  .group-form-card {
    width: 100vw !important;
  }

  .group-form {
    width: auto !important;
  }

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

  .upstream-row,
  .config-item-content {
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }

  .upstream-weight {
    flex: 1;
    flex-direction: column;
    align-items: flex-start;
  }

  .config-value {
    flex: 1;
  }

  .upstream-actions,
  .config-actions {
    justify-content: flex-end;
  }
}

/* Header规则相关样式 */
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

/* 极光青绿配色系统 - 按钮样式 */
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
</style>
