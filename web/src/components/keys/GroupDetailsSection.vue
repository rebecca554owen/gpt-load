<script setup lang="ts">
import type { Group, GroupConfigOption, ParentAggregateGroup } from "@/types/models";
import { maskProxyKeys } from "@/utils/display";
import { CopyOutline, EyeOffOutline, EyeOutline, HelpCircleOutline } from "@vicons/ionicons5";
import {
  NCollapse,
  NCollapseItem,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NTag,
  NTooltip,
  NButton,
  NButtonGroup,
} from "naive-ui";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  group?: Group | null;
  configOptions?: GroupConfigOption[];
  parentAggregateGroups?: ParentAggregateGroup[];
}

interface Emits {
  (e: "navigate-to-group", groupId: number): void;
}

const props = withDefaults(defineProps<Props>(), {
  group: null,
  configOptions: () => [],
  parentAggregateGroups: () => [],
});

const emit = defineEmits<Emits>();
const { t } = useI18n();

const showProxyKeys = ref(false);

const proxyKeysDisplay = computed(() => {
  if (!props.group?.proxy_keys) {
    return "-";
  }
  if (showProxyKeys.value) {
    return props.group.proxy_keys.replace(/,/g, "\n");
  }
  return maskProxyKeys(props.group.proxy_keys);
});

const hasAdvancedConfig = computed(() => {
  return (
    (props.group?.config && Object.keys(props.group.config).length > 0) ||
    props.group?.param_overrides ||
    (props.group?.header_rules && props.group.header_rules.length > 0)
  );
});

const isAggregateGroup = computed(() => {
  return props.group?.group_type === "aggregate";
});

async function copyProxyKeys() {
  if (!props.group?.proxy_keys) {
    return;
  }
  const keysToCopy = props.group.proxy_keys.replace(/,/g, "\n");
  try {
    await navigator.clipboard.writeText(keysToCopy);
    window.$message.success(t("keys.proxyKeysCopied"));
  } catch {
    window.$message.error(t("keys.copyFailed"));
  }
}

function getConfigDisplayName(key: string): string {
  const option = props.configOptions?.find(opt => opt.key === key);
  return option?.name || key;
}

function getConfigDescription(key: string): string {
  const option = props.configOptions?.find(opt => opt.key === key);
  return option?.description || t("keys.noDescription");
}

function handleNavigateToGroup(groupId: number) {
  emit("navigate-to-group", groupId);
}
</script>

<template>
  <div class="details-section">
    <n-collapse accordion>
      <n-collapse-item :title="t('keys.detailInfo')" name="details">
        <div class="details-content">
          <div class="detail-section">
            <h4 class="section-title">{{ t("keys.basicInfo") }}</h4>
            <n-form label-placement="left" label-width="140px" label-align="right">
              <n-grid cols="1 m:2">
                <n-grid-item>
                  <n-form-item :label="`${t('keys.groupName')}：`">
                    {{ group?.name }}
                  </n-form-item>
                </n-grid-item>
                <n-grid-item>
                  <n-form-item :label="`${t('keys.displayName')}：`">
                    {{ group?.display_name }}
                  </n-form-item>
                </n-grid-item>
                <n-grid-item>
                  <n-form-item :label="`${t('keys.channelType')}：`">
                    {{ group?.channel_type }}
                  </n-form-item>
                </n-grid-item>
                <n-grid-item>
                  <n-form-item :label="`${t('keys.sortOrder')}：`">
                    {{ group?.sort }}
                  </n-form-item>
                </n-grid-item>
                <n-grid-item v-if="!isAggregateGroup">
                  <n-form-item :label="`${t('keys.testModel')}：`">
                    {{ group?.test_model }}
                  </n-form-item>
                </n-grid-item>
                <n-grid-item v-if="!isAggregateGroup && group?.channel_type !== 'gemini'">
                  <n-form-item :label="`${t('keys.testPath')}：`">
                    {{ group?.validation_endpoint }}
                  </n-form-item>
                </n-grid-item>
                <n-grid-item :span="2">
                  <n-form-item :label="`${t('keys.proxyKeys')}：`">
                    <div class="proxy-keys-content">
                      <span class="key-text">{{ proxyKeysDisplay }}</span>
                      <n-button-group size="small" class="key-actions" v-if="group?.proxy_keys">
                        <n-tooltip trigger="hover">
                          <template #trigger>
                            <n-button quaternary circle @click="showProxyKeys = !showProxyKeys">
                              <template #icon>
                                <n-icon :component="showProxyKeys ? EyeOffOutline : EyeOutline" />
                              </template>
                            </n-button>
                          </template>
                          {{ showProxyKeys ? t("keys.hideKeys") : t("keys.showKeys") }}
                        </n-tooltip>
                        <n-tooltip trigger="hover">
                          <template #trigger>
                            <n-button quaternary circle @click="copyProxyKeys">
                              <template #icon>
                                <n-icon :component="CopyOutline" />
                              </template>
                            </n-button>
                          </template>
                          {{ t("keys.copyKeys") }}
                        </n-tooltip>
                      </n-button-group>
                    </div>
                  </n-form-item>
                </n-grid-item>
                <n-grid-item :span="2">
                  <n-form-item :label="`${t('common.description')}：`">
                    <div class="description-content">
                      {{ group?.description || "-" }}
                    </div>
                  </n-form-item>
                </n-grid-item>
              </n-grid>
            </n-form>
          </div>

          <div class="detail-section" v-if="!isAggregateGroup && parentAggregateGroups.length > 0">
            <h4 class="section-title">{{ t("keys.aggregateReferences") }}</h4>
            <n-form label-placement="left" label-width="140px">
              <n-form-item
                v-for="(parent, index) in parentAggregateGroups"
                :key="parent.group_id"
                class="aggregate-ref-item"
                :label="`${t('keys.aggregateGroup')} ${index + 1}:`"
              >
                <span class="aggregate-weight">
                  <n-tag size="small" type="info">
                    {{ t("keys.weight") }}: {{ parent.weight }}
                  </n-tag>
                </span>
                <n-input
                  class="aggregate-name"
                  :value="parent.display_name || parent.name"
                  readonly
                  size="small"
                  style="margin-left: 5px; margin-right: 8px"
                />
                <n-button
                  round
                  tertiary
                  type="default"
                  size="tiny"
                  @click="handleNavigateToGroup(parent.group_id)"
                  :title="t('keys.viewGroupInfo')"
                >
                  <template #icon>
                    <n-icon :component="EyeOutline" />
                  </template>
                  {{ t("common.view") }}
                </n-button>
              </n-form-item>
            </n-form>
          </div>

          <div class="detail-section" v-if="!isAggregateGroup">
            <h4 class="section-title">{{ t("keys.upstreamAddresses") }}</h4>
            <n-form label-placement="left" label-width="140px">
              <n-form-item
                v-for="(upstream, index) in group?.upstreams ?? []"
                :key="index"
                class="upstream-item"
                :label="`${t('keys.upstream')} ${index + 1}:`"
              >
                <span class="upstream-weight">
                  <n-tag size="small" type="info">
                    {{ t("keys.weight") }}: {{ upstream.weight }}
                  </n-tag>
                </span>
                <n-input class="upstream-url" :value="upstream.url" readonly size="small" />
              </n-form-item>
            </n-form>
          </div>

          <div class="detail-section" v-if="!isAggregateGroup && hasAdvancedConfig">
            <h4 class="section-title">{{ t("keys.advancedConfig") }}</h4>
            <n-form label-placement="left">
              <n-form-item v-for="(value, key) in group?.config || {}" :key="key">
                <template #label>
                  <n-tooltip trigger="hover" :delay="300" placement="right-start">
                    <template #trigger>
                      <span class="config-label">
                        {{ getConfigDisplayName(key) }}:
                        <n-icon :component="HelpCircleOutline" size="14" class="config-help-icon" />
                      </span>
                    </template>
                    <div class="config-tooltip">
                      <div class="tooltip-title">{{ getConfigDisplayName(key) }}</div>
                      <div class="tooltip-description">{{ getConfigDescription(key) }}</div>
                      <div class="tooltip-key">{{ t("keys.configKey") }}: {{ key }}</div>
                    </div>
                  </n-tooltip>
                </template>
                {{ value || "-" }}
              </n-form-item>
              <n-form-item
                v-if="group?.header_rules && group.header_rules.length > 0"
                :label="`${t('keys.customHeaders')}：`"
                :span="2"
              >
                <div class="header-rules-display">
                  <div
                    v-for="(rule, index) in group.header_rules"
                    :key="index"
                    class="header-rule-item"
                  >
                    <n-tag :type="rule.action === 'remove' ? 'error' : 'default'" size="small">
                      {{ rule.key }}
                    </n-tag>
                    <span class="header-separator">:</span>
                    <span class="header-value" v-if="rule.action === 'set'">
                      {{ rule.value || t("keys.emptyValue") }}
                    </span>
                    <span class="header-removed" v-else>{{ t("common.delete") }}</span>
                  </div>
                </div>
              </n-form-item>
              <n-form-item
                v-if="group?.model_redirect_rules"
                :label="`${t('keys.modelRedirectPolicy')}：`"
                :span="2"
              >
                <n-tag :type="group?.model_redirect_strict ? 'warning' : 'success'" size="small">
                  {{
                    group?.model_redirect_strict
                      ? t("keys.modelRedirectStrictMode")
                      : t("keys.modelRedirectLooseMode")
                  }}
                </n-tag>
              </n-form-item>
              <n-form-item
                v-if="group?.model_redirect_rules"
                :label="`${t('keys.modelRedirectRules')}：`"
                :span="2"
              >
                <pre class="config-json">{{
                  JSON.stringify(group?.model_redirect_rules || {}, null, 2)
                }}</pre>
              </n-form-item>
              <n-form-item
                v-if="group?.param_overrides"
                :label="`${t('keys.paramOverrides')}：`"
                :span="2"
              >
                <pre class="config-json">{{
                  JSON.stringify(group?.param_overrides || "", null, 2)
                }}</pre>
              </n-form-item>
            </n-form>
          </div>
        </div>
      </n-collapse-item>
    </n-collapse>
  </div>
</template>

<style scoped>
.details-section {
  margin-top: 12px;
}

.details-content {
  margin-top: 12px;
}

.detail-section {
  margin-bottom: 24px;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 12px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--border-color);
}

.upstream-url {
  font-family: monospace;
  font-size: 0.9rem;
  color: var(--text-primary);
  margin-left: 5px;
}

.upstream-weight {
  min-width: 70px;
}

.config-json {
  background: var(--bg-secondary);
  border-radius: var(--border-radius-sm);
  padding: 12px;
  font-size: 0.8rem;
  color: var(--text-primary);
  margin: 8px 0;
  overflow-x: auto;
}

.description-content {
  white-space: pre-wrap;
  word-wrap: break-word;
  line-height: 1.5;
  min-height: 20px;
  color: var(--text-primary);
}

.aggregate-weight {
  min-width: 70px;
}

.aggregate-name {
  font-family: monospace;
  font-size: 0.9rem;
  color: var(--text-primary);
  width: 200px;
}

.proxy-keys-content {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  width: 100%;
  gap: 8px;
}

.key-text {
  flex-grow: 1;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
  padding-top: 4px;
  color: var(--text-primary);
}

.key-actions {
  flex-shrink: 0;
}

.config-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: help;
}

.config-help-icon {
  color: var(--text-tertiary);
  transition: color 0.2s ease;
}

.config-label:hover .config-help-icon {
  color: var(--primary-color);
}

.config-tooltip {
  max-width: 300px;
  padding: 8px 0;
}

.tooltip-title {
  font-weight: 600;
  color: white;
  margin-bottom: 4px;
  font-size: 0.9rem;
}

.tooltip-description {
  color: rgba(255, 255, 255, 0.9);
  margin-bottom: 6px;
  line-height: 1.4;
  font-size: 0.85rem;
}

.tooltip-key {
  color: rgba(255, 255, 255, 0.8);
  font-size: 0.75rem;
  font-family: monospace;
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
  display: inline-block;
}

.header-rules-display {
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: var(--bg-secondary);
  border-radius: var(--border-radius-sm);
  padding: 8px;
}

.header-rule-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.875rem;
}

.header-separator {
  color: var(--text-secondary);
  font-weight: 500;
}

.header-value {
  color: var(--text-primary);
  font-family: monospace;
  background: var(--bg-secondary);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.8rem;
}

.header-removed {
  color: var(--error-color, #dc2626);
  font-style: italic;
  font-size: 0.8rem;
}

:deep(.n-form-item-feedback-wrapper) {
  min-height: 0;
}
</style>
