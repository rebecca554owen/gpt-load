<script setup lang="ts">
import { computed, ref } from "vue";

defineOptions({
  name: "LineChart",
});

import { useI18n } from "vue-i18n";
import type { ChartDataset, ChartViewType } from "@/types/models";
import { NRadioGroup, NRadio, NSelect } from "naive-ui";
import { useChartColors } from "@/composables/useChartColors";
import { useChartData, type TimeRangeHours } from "@/composables/useChartData";
import { useChartAnimation } from "@/composables/useChartAnimation";
import { useChartInteraction } from "@/composables/useChartInteraction";
import { useChartRendering } from "@/composables/useChartRendering";
import { CHART_CONFIG } from "@/constants/chart";

interface Props {
  viewType: ChartViewType;
  timeRange: TimeRangeHours;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  "update:viewType": [value: ChartViewType];
  "update:timeRange": [value: TimeRangeHours];
}>();

const { t } = useI18n();
const { getDatasetColor: getColor, isErrorDataset: checkIsErrorDataset } = useChartColors();

const chartSvg = ref<SVGElement>();

const getDatasetColor = (dataset: ChartDataset, speedIndex?: number): string => {
  return getColor(dataset.label_key || "", dataset.label, speedIndex);
};

const isErrorDataset = (label: string): boolean => {
  return checkIsErrorDataset(label, t);
};

// Track token speed dataset index for color assignment
const isTokenSpeedView = computed(() => props.viewType === "token_speed");

const {
  chartData,
  loading: _loading,
  hiddenDatasetIndices,
  toggleDataset,
  timeRangeOptions,
} = useChartData(
  () => props.viewType,
  () => props.timeRange,
  () => {
    startAnimation();
  }
);

const {
  chartWidth,
  chartHeight,
  padding,
  plotWidth,
  plotHeight,
  dataRange: _dataRange,
  yTicks,
  visibleLabels,
  getXPosition,
  getYPosition,
  generateLinePath,
  generateAreaPath,
  formatNumber,
  formatTimeLabel,
} = useChartRendering(
  () => chartData.value,
  () => props.timeRange,
  () => hiddenDatasetIndices.value,
  CHART_CONFIG.width,
  CHART_CONFIG.height,
  isTokenSpeedView.value ? CHART_CONFIG.paddingWithLegend : CHART_CONFIG.padding
);

const { startAnimation } = useChartAnimation(plotWidth, plotHeight);

const { hoveredPoint, tooltipData, tooltipPosition, handleMouseMove, hideTooltip } =
  useChartInteraction(
    () => chartData.value,
    () => chartSvg.value,
    getXPosition,
    getYPosition,
    getDatasetColor,
    () => hiddenDatasetIndices.value
  );

const translateLabel = (label: string): string => {
  if (/[一-龥]/.test(label)) {
    return label;
  }
  // For token speed labels in format "group_name - model_name", try to translate
  if (label.includes(" - ")) {
    return label; // Return as-is since it's a dynamic combination
  }
  return t(label);
};

const datasetsWithColor = computed(() => {
  if (!chartData.value) {
    return [];
  }
  return chartData.value.datasets.map((dataset, index) => ({
    ...dataset,
    color: isTokenSpeedView.value ? getDatasetColor(dataset, index) : getDatasetColor(dataset),
  }));
});
</script>

<template>
  <div class="chart-container">
    <div class="chart-header">
      <div class="chart-title-section">
        <!-- Top-left: View type toggle -->
        <n-radio-group
          :value="viewType"
          @update:value="
            (value: 'request' | 'token' | 'token_speed') => emit('update:viewType', value)
          "
          size="small"
          class="view-toggle"
        >
          <n-radio value="token_speed">{{ t("dashboard.tokenSpeedView") }}</n-radio>
          <n-radio value="token">{{ t("dashboard.tokenView") }}</n-radio>
          <n-radio value="request">{{ t("dashboard.requestView") }}</n-radio>
        </n-radio-group>
      </div>
      <!-- Top-right: Time range selector -->
      <n-select
        :value="timeRange"
        @update:value="(value: number) => emit('update:timeRange', value as TimeRangeHours)"
        :options="timeRangeOptions"
        size="small"
        style="width: 140px"
      />
    </div>

    <div v-if="chartData" class="chart-content">
      <div class="chart-wrapper">
        <div class="chart-legend" :class="{ 'legend-speed': viewType === 'token_speed' }">
          <div
            v-for="(dataset, index) in datasetsWithColor"
            :key="dataset.label"
            class="legend-item"
            :class="{ 'legend-item-hidden': hiddenDatasetIndices.has(index) }"
            @click="toggleDataset(index)"
          >
            <div
              v-if="viewType === 'token_speed'"
              class="legend-text-prefix"
              :style="{ color: dataset.color }"
            >
              {{ index + 1 }}.
            </div>
            <div v-else class="legend-indicator" :style="{ backgroundColor: dataset.color }" />
            <span class="legend-label">{{ translateLabel(dataset.label) }}</span>
          </div>
        </div>
        <svg
          ref="chartSvg"
          viewBox="0 0 800 260"
          class="chart-svg"
          role="img"
          :aria-label="t('dashboard.chartAriaLabel', { viewType: t(`dashboard.${viewType}View`) })"
          @mousemove="handleMouseMove"
          @mouseleave="hideTooltip"
        >
          <!-- Background grid -->
          <defs>
            <pattern id="grid" width="40" height="30" patternUnits="userSpaceOnUse">
              <path
                d="M 40 0 L 0 0 0 30"
                fill="none"
                :stroke="`var(--chart-grid)`"
                stroke-width="1"
                opacity="0.3"
              />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />

          <!-- Y-axis tick lines and labels -->
          <g class="y-axis">
            <line
              :x1="padding.left"
              :y1="padding.top"
              :x2="padding.left"
              :y2="chartHeight - padding.bottom"
              :stroke="`var(--chart-axis)`"
              stroke-width="2"
            />
            <g v-for="(tick, index) in yTicks" :key="index">
              <line
                :x1="padding.left - 5"
                :y1="getYPosition(tick)"
                :x2="padding.left"
                :y2="getYPosition(tick)"
                :stroke="`var(--chart-text)`"
                stroke-width="1"
              />
              <text
                :x="padding.left - 10"
                :y="getYPosition(tick) + 4"
                text-anchor="end"
                class="axis-label"
              >
                {{ formatNumber(tick) }}
              </text>
            </g>
          </g>

          <!-- X-axis tick lines and labels -->
          <g class="x-axis">
            <line
              :x1="padding.left"
              :y1="chartHeight - padding.bottom"
              :x2="chartWidth - padding.right"
              :y2="chartHeight - padding.bottom"
              :stroke="`var(--chart-axis)`"
              stroke-width="2"
            />
            <g v-for="(label, index) in visibleLabels" :key="index">
              <line
                :x1="getXPosition(label.index)"
                :y1="chartHeight - padding.bottom"
                :x2="getXPosition(label.index)"
                :y2="chartHeight - padding.bottom + 5"
                :stroke="`var(--chart-text)`"
                stroke-width="1"
              />
              <text
                :x="getXPosition(label.index)"
                :y="chartHeight - padding.bottom + 18"
                text-anchor="middle"
                class="axis-label"
              >
                {{ label.text }}
              </text>
            </g>
          </g>

          <!-- Data lines -->
          <g
            v-for="(dataset, datasetIndex) in datasetsWithColor"
            :key="dataset.label"
            v-show="!hiddenDatasetIndices.has(datasetIndex)"
          >
            <!-- Gradient definition -->
            <defs>
              <linearGradient :id="`gradient-${datasetIndex}`" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" :stop-color="dataset.color" stop-opacity="0.3" />
                <stop offset="100%" :stop-color="dataset.color" stop-opacity="0.05" />
              </linearGradient>
            </defs>

            <!-- Fill area -->
            <path
              :d="generateAreaPath(dataset.data)"
              :fill="`url(#gradient-${datasetIndex})`"
              class="area-path"
              :class="{ 'area-path-speed': viewType === 'token_speed' }"
              :style="{ opacity: isErrorDataset(dataset.label) ? 0.3 : 0.6 }"
            />

            <!-- Main line -->
            <path
              :d="generateLinePath(dataset.data)"
              :stroke="dataset.color"
              :stroke-width="isErrorDataset(dataset.label) ? 1 : 2"
              fill="none"
              class="line-path"
              :style="{
                opacity: isErrorDataset(dataset.label) ? 0.75 : 1,
                filter: 'drop-shadow(0 1px 3px rgba(0,0,0,0.1))',
              }"
            />

            <!-- Data points -->
            <g v-for="(value, pointIndex) in dataset.data" :key="pointIndex">
              <!-- Non-zero value points -->
              <circle
                v-if="value > 0"
                :cx="getXPosition(pointIndex)"
                :cy="getYPosition(value)"
                :r="isErrorDataset(dataset.label) ? 2 : 3"
                :fill="dataset.color"
                :stroke="dataset.color"
                stroke-width="1"
                class="data-point"
                :class="{
                  'point-hover': hoveredPoint?.pointIndex === pointIndex,
                }"
                :style="{ opacity: isErrorDataset(dataset.label) ? 0.8 : 1 }"
              />
              <!-- Zero value points - only shown for total_tokens to indicate no data in this period -->
              <circle
                v-else-if="dataset.label_key === 'dashboard.total_tokens'"
                :cx="getXPosition(pointIndex)"
                :cy="getYPosition(0)"
                r="1.5"
                :fill="dataset.color"
                class="data-point-zero"
              />
            </g>
          </g>

          <!-- Hover indicator line -->
          <line
            v-if="hoveredPoint"
            :x1="getXPosition(hoveredPoint.pointIndex)"
            :y1="padding.top"
            :x2="getXPosition(hoveredPoint.pointIndex)"
            :y2="chartHeight - padding.bottom"
            stroke="#999"
            stroke-width="1"
            stroke-dasharray="5,5"
            opacity="0.7"
          />
        </svg>

        <!-- Tooltip -->
        <div
          v-if="tooltipData"
          class="chart-tooltip"
          :class="{ 'tooltip-speed': viewType === 'token_speed' }"
          :style="{
            left: (tooltipPosition.x / 800) * 100 + '%',
            top: (tooltipPosition.y / 260) * 100 + '%',
          }"
        >
          <div class="tooltip-time">
            {{
              formatTimeLabel(tooltipData.time, tooltipData.index, chartData?.labels.length || 0)
            }}
          </div>
          <div
            v-for="(dataset, idx) in tooltipData.datasets"
            :key="dataset.label"
            class="tooltip-value"
          >
            <span v-if="viewType === 'token_speed'" class="tooltip-rank-text">{{ idx + 1 }}.</span>
            <span v-else class="tooltip-color" :style="{ backgroundColor: dataset.color }" />
            <span class="tooltip-label-text">{{ translateLabel(dataset.label) }}</span>
            <span class="tooltip-value-number">{{ formatNumber(dataset.value) }}</span>
            <span v-if="viewType === 'token_speed' && dataset.value > 0" class="tooltip-unit">token/s</span>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="chart-loading">
      <p>{{ t("common.loading") }}</p>
    </div>
  </div>
</template>

<style scoped>
.chart-container {
  padding: 20px;
  border-radius: 16px;
  backdrop-filter: blur(4px);
  border: 1px solid var(--border-color-light);
}

/* Light theme - keep original purple gradient design */
:root:not(.dark) .chart-container {
  background: var(--primary-gradient);
  color: white;
}

/* Dark theme - use dark blue-purple gradient outer background */
:root.dark .chart-container {
  background: var(--chart-bg-dark-gradient);
  box-shadow: var(--shadow-md);
  border: 1px solid var(--chart-border-dark);
  color: var(--chart-text-dark);
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  gap: 16px;
}

.chart-title-section {
  flex: 1;
  display: flex;
  align-items: center;
}

/* Light theme - View toggle button */
:root:not(.dark) .view-toggle {
  background: rgba(255, 255, 255, 0.3);
  border-radius: 8px;
  padding: 4px 8px;
}

:root:not(.dark) .view-toggle :deep(.n-radio) {
  color: white;
}

:root:not(.dark) .view-toggle :deep(.n-radio__dot) {
  border-color: rgba(255, 255, 255, 0.5);
}

:root:not(.dark) .view-toggle :deep(.n-radio.n-radio--checked .n-radio__dot) {
  background: white;
  border-color: white;
}

/* Dark theme - View toggle button */
:root.dark .view-toggle {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 8px;
  padding: 4px 8px;
}

.chart-legend {
  position: absolute;
  top: 0px;
  left: 115px;
  right: 0px;
  z-index: 10;
  display: flex;
  justify-content: center;
  gap: 12px;
  padding: 2px;
  backdrop-filter: blur(8px);
  border-radius: 24px;
  flex-wrap: wrap;
}

/* Light theme */
:root:not(.dark) .chart-legend {
  background: rgba(255, 255, 255, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.5);
}

/* Dark theme */
:root.dark .chart-legend {
  background: var(--overlay-bg);
  border: 1px solid var(--border-color);
}

/* Token speed legend style */
.legend-speed {
  gap: 12px;
  padding: 8px 16px;
  justify-content: flex-start;
  flex-wrap: wrap;
  max-width: calc(100% - 130px);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 13px;
  padding: 8px 16px;
  border-radius: 20px;
  transition: all 0.2s ease;
  cursor: pointer;
  user-select: none;
}

/* Light theme */
:root:not(.dark) .legend-item {
  color: var(--chart-grid-light);
  background: rgba(255, 255, 255, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.7);
}

/* Dark theme */
:root.dark .legend-item {
  color: var(--text-primary);
  background: var(--bg-tertiary);
  border: 1px solid var(--border-color);
}

/* Hidden state */
.legend-item-hidden {
  opacity: 0.4;
  text-decoration: line-through;
}

/* Light theme hover effect */
:root:not(.dark) .legend-item:hover {
  background: rgba(255, 255, 255, 0.9);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* 暗黑主题悬停效果 */
:root.dark .legend-item:hover {
  background: var(--primary-color);
  color: white;
  transform: translateY(-1px);
  box-shadow: var(--shadow-lg);
}

.legend-indicator {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  position: relative;
}

.legend-indicator::after {
  content: "";
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 6px;
  height: 6px;
  background: rgba(255, 255, 255, 0.3);
  border-radius: 50%;
}

.legend-label {
  font-size: 13px;
  color: inherit;
}

.legend-text-prefix {
  font-weight: 700;
  font-size: 13px;
  flex-shrink: 0;
}

.chart-wrapper {
  position: relative;
  display: flex;
  justify-content: center;
}

.chart-svg {
  width: 100%;
  height: auto;
  border-radius: 8px;
}

/* 浅色主题 - 白色背景 */
:root:not(.dark) .chart-svg {
  background: white;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  border: 1px solid #e0e0e0;
}

/* 暗黑主题 - 深色背景 */
:root.dark .chart-svg {
  background: var(--card-bg-solid);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.2);
  border: 1px solid var(--border-color);
}

.axis-label {
  fill: var(--chart-text);
  font-size: 12px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
}

.line-path {
  transition: all 0.3s ease;
}

.area-path {
  opacity: 0.6;
  transition: opacity 0.3s ease;
}

.data-point {
  cursor: pointer;
  transition: all 0.2s ease;
}

.data-point:hover,
.point-hover {
  r: 5;
  filter: drop-shadow(0 0 6px rgba(0, 0, 0, 0.3));
}

.data-point-zero {
  cursor: default;
  transition: opacity 0.2s ease;
}

.data-point-zero:hover {
  opacity: 0.8;
}

.chart-tooltip {
  position: absolute;
  background: rgba(0, 0, 0, 0.9);
  color: white;
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 13px;
  pointer-events: none;
  transform: translate(-50%, -100%);
  z-index: 1000;
  backdrop-filter: blur(8px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.1);
  min-width: 140px;
  max-width: 240px;
}

/* Light theme tooltip */
:root:not(.dark) .chart-tooltip {
  background: rgba(255, 255, 255, 0.95);
  color: #1a1a2e;
  border: 1px solid rgba(0, 0, 0, 0.1);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}

.tooltip-speed {
  min-width: 200px;
  max-width: 320px;
}

:root:not(.dark) .tooltip-speed {
  background: rgba(255, 255, 255, 0.98);
}

:root.dark .tooltip-speed {
  background: rgba(20, 20, 30, 0.95);
}

.tooltip-time {
  font-weight: 700;
  margin-bottom: 8px;
  text-align: center;
  color: var(--chart-axis-light);
  font-size: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
  padding-bottom: 6px;
}

:root:not(.dark) .tooltip-time {
  color: #666;
  border-bottom-color: rgba(0, 0, 0, 0.1);
}

.tooltip-value {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  margin-bottom: 4px;
  font-size: 12px;
}

.tooltip-rank-text {
  font-weight: 700;
  color: #888;
  min-width: 20px;
}

:root:not(.dark) .tooltip-rank-text {
  color: #999;
}

.tooltip-label-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tooltip-value-number {
  font-weight: 700;
  font-feature-settings: "tnum";
  color: #a8e6cf;
}

:root:not(.dark) .tooltip-value-number {
  color: #059669;
}

.tooltip-unit {
  font-size: 11px;
  opacity: 0.7;
  font-weight: 500;
}

.tooltip-value:last-child {
  margin-bottom: 0;
}

.tooltip-color {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.chart-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 260px;
  color: white;
}

.chart-loading p {
  margin-top: 16px;
  font-size: 16px;
  opacity: 0.8;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .chart-container {
    padding: 16px;
  }

  .chart-header {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }

  .chart-title-section {
    justify-content: center;
  }

  .chart-wrapper {
    flex-direction: column;
    align-items: center;
  }

  .chart-legend {
    position: relative;
    transform: none;
    left: auto;
    top: auto;
    margin-top: 8px;
    margin-bottom: 12px;
    background: transparent;
    backdrop-filter: none;
    border: none;
    width: 100%;
    flex-wrap: wrap;
    gap: 8px;
    justify-content: center;
  }

  .legend-item {
    padding: 4px 10px;
    font-size: 12px;
    color: var(--text-primary);
    background: var(--card-bg-solid);
    border: 1px solid var(--border-color);
    gap: 6px;
  }

  .legend-rank-badge {
    padding-left: 24px;
  }

  .legend-rank {
    width: 16px;
    height: 16px;
    font-size: 10px;
  }

  .chart-svg {
    width: 100%;
    height: auto;
  }
}

/* 动画效果 */
@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.chart-container {
  animation: fadeInUp 0.6s ease-out;
}

.legend-item {
  animation: fadeInUp 0.6s ease-out;
}

.legend-item:nth-child(2) {
  animation-delay: 0.1s;
}

.legend-item:nth-child(3) {
  animation-delay: 0.2s;
}
</style>
