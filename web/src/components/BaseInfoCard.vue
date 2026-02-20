<script setup lang="ts">
import AppIcon from "@/components/icons/AppIcon.vue";
import type { DashboardStatsResponse } from "@/types/models";
import { NCard, NGrid, NGridItem, NSpace, NTag } from "naive-ui";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();

// Props
interface Props {
  stats: DashboardStatsResponse | null;
}

const props = defineProps<Props>();

// Unit format state: false = pure number, true = auto format (K/M)
const autoFormat = ref(true);

// Toggle unit format
const toggleFormat = () => {
  autoFormat.value = !autoFormat.value;
};

// Expose toggle function to parent component
defineExpose({
  toggleFormat,
});

// Format value display
const formatValue = (value: number, type: "count" | "rate" = "count"): string => {
  if (type === "rate") {
    return `${value.toFixed(1)}%`;
  }

  // Pure number mode
  if (!autoFormat.value) {
    return value.toLocaleString();
  }

  // Auto format mode
  if (value >= 1000000) {
    return `${(value / 1000000).toFixed(1)}M`;
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)}K`;
  }
  return value.toString();
};

// Format trend display
const formatTrend = (trend: number): string => {
  const sign = trend >= 0 ? "+" : "";
  return `${sign}${trend.toFixed(1)}%`;
};

// Row 1 card configuration
const firstRowCards = computed(() => [
  {
    key: "key-count",
    title: t("dashboard.totalKeys"),
    value: props.stats?.key_count?.value ?? 0,
    trend: props.stats?.key_count?.trend ?? 0,
    trendIsGrowth: props.stats?.key_count?.trend_is_growth ?? true,
    icon: "keys",
    color: "var(--color-key-count)",
  },
  {
    key: "rpm",
    title: t("dashboard.rpm10Min"),
    value: props.stats?.rpm?.value ?? 0,
    trend: props.stats?.rpm?.trend ?? 0,
    trendIsGrowth: props.stats?.rpm?.trend_is_growth ?? true,
    icon: "warning",
    color: "var(--color-rpm)",
  },
  {
    key: "request-count",
    title: t("dashboard.totalRequests"),
    value: props.stats?.request_count?.value ?? 0,
    trend: props.stats?.request_count?.trend ?? 0,
    trendIsGrowth: props.stats?.request_count?.trend_is_growth ?? true,
    icon: "globe",
    color: "var(--color-request-count)",
  },
  {
    key: "error-rate",
    title: t("dashboard.errorRate24h"),
    value: props.stats?.error_rate?.value ?? 0,
    trend: props.stats?.error_rate?.trend ?? 0,
    trendIsGrowth: props.stats?.error_rate?.trend_is_growth ?? true,
    icon: "alert",
    color: "var(--color-error-rate)",
    isRate: true,
  },
]);

// Row 2 card configuration
const secondRowCards = computed(() => [
  {
    key: "non-cached-prompt-tokens",
    title: t("dashboard.nonCachedPromptTokens"),
    value: props.stats?.non_cached_prompt_tokens?.value ?? 0,
    trend: props.stats?.non_cached_prompt_tokens?.trend ?? 0,
    trendIsGrowth: props.stats?.non_cached_prompt_tokens?.trend_is_growth ?? true,
    icon: "edit",
    color: "var(--color-prompt-tokens)",
  },
  {
    key: "cached-tokens",
    title: t("dashboard.cachedTokens"),
    value: props.stats?.cached_tokens?.value ?? 0,
    trend: props.stats?.cached_tokens?.trend ?? 0,
    trendIsGrowth: props.stats?.cached_tokens?.trend_is_growth ?? true,
    icon: "check",
    color: "var(--color-cached-tokens)",
  },
  {
    key: "completion-tokens",
    title: t("dashboard.completionTokens"),
    value: props.stats?.completion_tokens?.value ?? 0,
    trend: props.stats?.completion_tokens?.trend ?? 0,
    trendIsGrowth: props.stats?.completion_tokens?.trend_is_growth ?? true,
    icon: "info",
    color: "var(--color-completion-tokens)",
  },
  {
    key: "total-tokens",
    title: t("dashboard.totalTokens"),
    value: props.stats?.total_tokens?.value ?? 0,
    trend: props.stats?.total_tokens?.trend ?? 0,
    trendIsGrowth: props.stats?.total_tokens?.trend_is_growth ?? true,
    icon: "dashboard",
    color: "var(--color-total-tokens)",
  },
]);
</script>

<template>
  <div class="stats-container">
    <n-space vertical :size="16">
      <!-- Row 1 cards -->
      <n-grid cols="2 s:4" :x-gap="16" :y-gap="16" responsive="screen">
        <n-grid-item v-for="card in firstRowCards" :key="card.key" span="1">
          <n-card :bordered="false" class="stat-card compact" @click="toggleFormat">
            <div class="stat-header">
              <div class="stat-icon" :style="{ '--card-color': card.color }">
                <app-icon :name="card.icon" :size="22" />
              </div>
              <n-tag
                v-if="props.stats && card.trend !== 0"
                :type="card.trendIsGrowth ? 'success' : 'error'"
                size="small"
                class="stat-trend"
              >
                {{ props.stats ? formatTrend(card.trend) : "--" }}
              </n-tag>
            </div>

            <div class="stat-content">
              <div class="stat-value">
                {{ props.stats ? formatValue(card.value, card.isRate ? "rate" : "count") : "--" }}
              </div>
              <div class="stat-title">{{ card.title }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill"
                :style="{
                  background: card.color,
                }"
              />
              <div class="stat-bar-shine" />
            </div>
          </n-card>
        </n-grid-item>
      </n-grid>

      <!-- Row 2 cards - Token details -->
      <n-grid cols="2 s:4" :x-gap="16" :y-gap="16" responsive="screen">
        <n-grid-item v-for="card in secondRowCards" :key="card.key" span="1">
          <n-card :bordered="false" class="stat-card compact" @click="toggleFormat">
            <div class="stat-header">
              <div class="stat-icon" :style="{ '--card-color': card.color }">
                <app-icon :name="card.icon" :size="22" />
              </div>
              <n-tag
                v-if="props.stats && card.trend !== 0"
                :type="card.trendIsGrowth ? 'success' : 'error'"
                size="small"
                class="stat-trend"
              >
                {{ props.stats ? formatTrend(card.trend) : "--" }}
              </n-tag>
            </div>

            <div class="stat-content">
              <div class="stat-value">
                {{ props.stats ? formatValue(card.value) : "--" }}
              </div>
              <div class="stat-title">{{ card.title }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill"
                :style="{
                  background: card.color,
                }"
              />
              <div class="stat-bar-shine" />
            </div>
          </n-card>
        </n-grid-item>
      </n-grid>
    </n-space>
  </div>
</template>

<style scoped>
.stats-container {
  width: 100%;
  animation: slideUp var(--transition-slow) ease-out;
}

.stat-card {
  background: var(--card-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-radius: var(--radius-lg);
  border: 1px solid var(--card-border);
  position: relative;
  overflow: hidden;
  transition: all var(--transition-base);
  cursor: pointer;
}

.stat-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--border-subtle);
  transition: opacity var(--transition-base);
  opacity: 0;
}

.stat-card:hover::before {
  opacity: 1;
}

.stat-card.compact {
  padding: 14px;
}

.stat-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.stat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--card-color) 12%, transparent);
  color: var(--card-color);
  transition: all var(--transition-base);
}

.stat-card:hover .stat-icon {
  background: var(--card-color);
  color: white;
  box-shadow: 0 4px 12px color-mix(in srgb, var(--card-color) 40%, transparent);
}

.stat-trend {
  font-weight: 600;
}

.stat-content {
  margin-bottom: 10px;
}

.stat-value {
  font-family: var(--font-display);
  font-size: 1.6rem;
  font-weight: 700;
  line-height: 1.1;
  color: var(--text-primary);
  margin-bottom: 4px;
  letter-spacing: -0.02em;
}

.stat-title {
  font-size: 0.8rem;
  color: var(--text-tertiary);
  font-weight: 500;
}

.stat-bar {
  width: 100%;
  height: 3px;
  background: var(--border-subtle);
  border-radius: var(--radius-xs);
  overflow: hidden;
  position: relative;
}

.stat-bar-fill {
  height: 100%;
  width: 100%;
  border-radius: var(--radius-xs);
  position: relative;
  z-index: 1;
}

.stat-bar-shine {
  position: absolute;
  top: 0;
  left: 0;
  height: 100%;
  width: 100%;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.4) 50%,
    transparent 100%
  );
  border-radius: var(--radius-xs);
  animation: shine 2s ease-in-out infinite;
  z-index: 2;
}

@keyframes shine {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Responsive grid */
:deep(.n-grid-item) {
  min-width: 0;
}
</style>
