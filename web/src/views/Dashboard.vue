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

// Time range in hours: 1/5/168/720
const timeRange = ref<TimeRangeHours>(5);

const dashboardStats = ref<DashboardStatsResponse | null>(null);

// Load statistics
const loadStats = async () => {
  try {
    // Use hours directly for stats API (1/5/24/168/720 hours)
    const response = await getDashboardStats(timeRange.value);
    dashboardStats.value = response.data;
  } catch (error) {
    console.error("Failed to load dashboard stats:", error);
  }
};

// Watch time range changes
watch(timeRange, () => {
  loadStats();
});

onMounted(() => {
  loadStats();
});
</script>

<template>
  <div class="dashboard-container">
    <n-space vertical :size="16">
      <!-- Encryption config error alert (highest priority) -->
      <encryption-mismatch-alert />

      <!-- Security warning banner -->
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
