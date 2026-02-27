<script setup lang="ts">
import { NStatistic, NTooltip, NGradientText, NDivider, NGrid, NGridItem } from "naive-ui";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import type { GroupStatsResponse, SubGroupInfo } from "@/types/models";

import { formatNumber, formatPercentage } from "@/utils/format";

interface Props {
  stats?: GroupStatsResponse | null;
  loading?: boolean;
  isAggregateGroup?: boolean;
  subGroups?: SubGroupInfo[];
}

const props = withDefaults(defineProps<Props>(), {
  stats: null,
  loading: false,
  isAggregateGroup: false,
  subGroups: () => [],
});

const { t } = useI18n();

function countSubGroupsBy(filter: (sg: SubGroupInfo) => boolean): number {
  return props.subGroups?.filter(filter).length || 0;
}

const activeSubGroupsCount = computed(() =>
  countSubGroupsBy(sg => sg.weight > 0 && sg.active_keys > 0)
);
const disabledSubGroupsCount = computed(() => countSubGroupsBy(sg => sg.weight === 0));
const unavailableSubGroupsCount = computed(() =>
  countSubGroupsBy(sg => sg.weight > 0 && sg.active_keys === 0)
);
</script>

<template>
  <div class="stats-summary">
    <n-grid cols="2 s:4" :x-gap="12" :y-gap="12" responsive="screen">
      <n-grid-item span="1">
        <n-statistic
          v-if="isAggregateGroup"
          :label="`${t('keys.subGroups')}：${props.subGroups?.length || 0}`"
        >
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="success" size="20">
                {{ activeSubGroupsCount }}
              </n-gradient-text>
            </template>
            {{ t("keys.activeSubGroups") }}
          </n-tooltip>
          <n-divider vertical />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="warning" size="20">
                {{ disabledSubGroupsCount }}
              </n-gradient-text>
            </template>
            {{ t("keys.disabledSubGroups") }}
          </n-tooltip>
          <n-divider vertical />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ unavailableSubGroupsCount }}
              </n-gradient-text>
            </template>
            {{ t("keys.unavailableSubGroups") }}
          </n-tooltip>
        </n-statistic>

        <n-statistic v-else :label="`${t('keys.keyCount')}：${stats?.key_stats?.total_keys ?? 0}`">
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="success" size="20">
                {{ stats?.key_stats?.active_keys ?? 0 }}
              </n-gradient-text>
            </template>
            {{ t("keys.activeKeyCount") }}
          </n-tooltip>
          <n-divider vertical />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ stats?.key_stats?.invalid_keys ?? 0 }}
              </n-gradient-text>
            </template>
            {{ t("keys.invalidKeyCount") }}
          </n-tooltip>
        </n-statistic>
      </n-grid-item>
      <n-grid-item span="1">
        <n-statistic
          :label="`${t('keys.stats24Hour')}：${formatNumber(stats?.stats_24_hour?.total_requests ?? 0)}`"
        >
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ formatNumber(stats?.stats_24_hour?.failed_requests ?? 0) }}
              </n-gradient-text>
            </template>
            {{ t("keys.stats24HourFailed") }}
          </n-tooltip>
          <n-divider vertical />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ formatPercentage(stats?.stats_24_hour?.failure_rate ?? 0) }}
              </n-gradient-text>
            </template>
            {{ t("keys.stats24HourFailureRate") }}
          </n-tooltip>
        </n-statistic>
      </n-grid-item>
      <n-grid-item span="1">
        <n-statistic
          :label="`${t('keys.stats7Day')}：${formatNumber(stats?.stats_7_day?.total_requests ?? 0)}`"
        >
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ formatNumber(stats?.stats_7_day?.failed_requests ?? 0) }}
              </n-gradient-text>
            </template>
            {{ t("keys.stats7DayFailed") }}
          </n-tooltip>
          <n-divider vertical />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ formatPercentage(stats?.stats_7_day?.failure_rate ?? 0) }}
              </n-gradient-text>
            </template>
            {{ t("keys.stats7DayFailureRate") }}
          </n-tooltip>
        </n-statistic>
      </n-grid-item>
      <n-grid-item span="1">
        <n-statistic
          :label="`${t('keys.stats30Day')}：${formatNumber(stats?.stats_30_day?.total_requests ?? 0)}`"
        >
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ formatNumber(stats?.stats_30_day?.failed_requests ?? 0) }}
              </n-gradient-text>
            </template>
            {{ t("keys.stats30DayFailed") }}
          </n-tooltip>
          <n-divider vertical />
          <n-tooltip trigger="hover">
            <template #trigger>
              <n-gradient-text type="error" size="20">
                {{ formatPercentage(stats?.stats_30_day?.failure_rate ?? 0) }}
              </n-gradient-text>
            </template>
            {{ t("keys.stats30DayFailureRate") }}
          </n-tooltip>
        </n-statistic>
      </n-grid-item>
    </n-grid>
  </div>
</template>

<style scoped>
.stats-summary {
  margin-bottom: 12px;
  text-align: center;
}
</style>
