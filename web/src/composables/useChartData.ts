import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import type { ChartData } from "@/types/models";
import { getDashboardChart } from "@/api/dashboard";
import { ANIMATION_DELAY } from "@/constants/chart";

// Time range options in hours
export type TimeRangeHours = 1 | 5 | 24 | 168 | 720;

export function useChartData(
  viewType: () => "request" | "token",
  timeRange: () => TimeRangeHours,
  onAnimationReady: () => void
) {
  const { t } = useI18n();

  const chartData = ref<ChartData | null>(null);
  const loading = ref(true);
  const hiddenDatasetIndices = ref<Set<number>>(new Set());

  const toggleDataset = (index: number) => {
    if (hiddenDatasetIndices.value.has(index)) {
      hiddenDatasetIndices.value.delete(index);
    } else {
      hiddenDatasetIndices.value.add(index);
    }
  };

  const timeRangeOptions = computed(() => [
    { label: t("dashboard.last1Hour"), value: 1 as const },
    { label: t("dashboard.last5Hours"), value: 5 as const },
    { label: t("dashboard.last24Hours"), value: 24 as const },
    { label: t("dashboard.last1Week"), value: 168 as const },
    { label: t("dashboard.last1Month"), value: 720 as const },
  ]);

  const fetchChartData = async () => {
    try {
      loading.value = true;
      const response = await getDashboardChart(viewType(), timeRange());
      chartData.value = response.data;

      setTimeout(() => {
        onAnimationReady();
      }, ANIMATION_DELAY);
    } catch (error) {
      console.error("Failed to fetch chart data:", error);
    } finally {
      loading.value = false;
    }
  };

  watch(
    () => [viewType(), timeRange()],
    () => {
      fetchChartData();
    }
  );

  onMounted(() => {
    fetchChartData();
  });

  return {
    chartData,
    loading,
    hiddenDatasetIndices,
    toggleDataset,
    timeRangeOptions,
    fetchChartData,
  };
}
