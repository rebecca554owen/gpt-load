<script setup lang="ts">
import { getDashboardStats } from "@/api/dashboard";
import BaseInfoCard from "@/components/BaseInfoCard.vue";
import EncryptionMismatchAlert from "@/components/EncryptionMismatchAlert.vue";
import LineChart from "@/components/LineChart.vue";
import SecurityAlert from "@/components/SecurityAlert.vue";
import type { ChartViewType, DashboardStatsResponse } from "@/types/models";
import type { TimeRangeHours } from "@/composables/useChartData";
import { NSpace } from "naive-ui";
import { onMounted, ref, watch } from "vue";

const viewType = ref<ChartViewType>("token_speed");

// 时间范围（小时）：1/5/168/720
const timeRange = ref<TimeRangeHours>(5);

const dashboardStats = ref<DashboardStatsResponse | null>(null);

// 加载统计数据
const loadStats = async () => {
  try {
    // 使用小时数直接调用统计 API（1/5/24/168/720 小时）
    const response = await getDashboardStats(timeRange.value);
    dashboardStats.value = response.data;
  } catch (error) {
    console.error("Failed to load dashboard stats:", error);
  }
};

// 监听时间范围变化
watch(timeRange, async () => {
  try {
    await loadStats();
  } catch (error) {
    console.error("Failed to load stats:", error);
  }
});

onMounted(() => {
  loadStats();
});
</script>

<template>
  <div class="dashboard-container">
    <n-space vertical :size="16">
      <!-- 加密配置错误警告（最高优先级） -->
      <encryption-mismatch-alert />

      <!-- 安全警告横幅 -->
      <security-alert
        v-if="dashboardStats?.security_warnings"
        :warnings="dashboardStats.security_warnings"
      />

      <base-info-card :stats="dashboardStats" />
      <line-chart
        class="dashboard-chart"
        :view-type="viewType"
        :time-range="timeRange"
        @update:view-type="viewType = $event"
        @update:time-range="timeRange = $event"
      />
    </n-space>
  </div>
</template>

<style scoped>
.dashboard-container {
  width: 100%;
}

.dashboard-header-card {
  background: var(--card-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-radius: var(--radius-lg);
  border: 1px solid var(--card-border);
  animation: slideUp var(--transition-base) ease-out;
}

.dashboard-title {
  font-family: var(--font-display);
  font-size: 2rem;
  font-weight: 700;
  margin: 0;
  letter-spacing: -0.03em;
}

.dashboard-subtitle {
  font-size: 1.1rem;
  font-weight: 500;
  color: var(--text-secondary);
}

.dashboard-chart {
  animation: slideUp var(--transition-slower) ease-out 0.1s both;
}
</style>
