import { ref } from "vue";
import type { ChartData, ChartDataset } from "@/types/models";
import { CHART_CONFIG, HOVER_DISTANCE } from "@/constants/chart";

interface HoveredPoint {
  datasetIndex: number;
  pointIndex: number;
  x: number;
  y: number;
}

interface TooltipData {
  time: string;
  index: number;
  datasets: Array<{
    label: string;
    value: number;
    color: string;
  }>;
}

interface TooltipPosition {
  x: number;
  y: number;
}

export function useChartInteraction(
  chartData: () => ChartData | null,
  chartSvg: () => SVGElement | undefined,
  getXPosition: (index: number) => number,
  getYPosition: (value: number) => number,
  getDatasetColor: (dataset: ChartDataset) => string,
  hiddenDatasetIndices: () => Set<number>
) {
  const hoveredPoint = ref<HoveredPoint | null>(null);
  const tooltipData = ref<TooltipData | null>(null);
  const tooltipPosition = ref<TooltipPosition>({ x: 0, y: 0 });

  const handleMouseMove = (event: MouseEvent) => {
    const currentData = chartData();
    const svg = chartSvg();

    if (!currentData || !svg) {
      return;
    }

    const rect = svg.getBoundingClientRect();
    const scaleX = CHART_CONFIG.width / rect.width;
    const scaleY = CHART_CONFIG.height / rect.height;

    const mouseX = (event.clientX - rect.left) * scaleX;
    const mouseY = (event.clientY - rect.top) * scaleY;

    let closestXDistance = Infinity;
    let closestTimeIndex = -1;

    currentData.labels.forEach((_, pointIndex) => {
      const x = getXPosition(pointIndex);
      const xDistance = Math.abs(mouseX - x);

      if (xDistance < closestXDistance) {
        closestXDistance = xDistance;
        closestTimeIndex = pointIndex;
      }
    });

    if (closestXDistance > HOVER_DISTANCE) {
      hoveredPoint.value = null;
      tooltipData.value = null;
      return;
    }

    const datasetsAtTime = currentData.datasets
      .map((dataset, index) => ({
        label: dataset.label,
        value: dataset.data[closestTimeIndex],
        color: getDatasetColor(dataset),
        index,
      }))
      .filter(item => !hiddenDatasetIndices().has(item.index))
      .sort((a, b) => b.value - a.value); // Sort by value descending

    if (closestTimeIndex >= 0) {
      hoveredPoint.value = {
        datasetIndex: 0,
        pointIndex: closestTimeIndex,
        x: mouseX,
        y: mouseY,
      };

      const x = getXPosition(closestTimeIndex);
      const avgY =
        datasetsAtTime.reduce((sum, item) => sum + getYPosition(item.value), 0) /
        datasetsAtTime.length;

      // Calculate tooltip position with boundary detection
      // Tooltip width is approximately 240px for token_speed view, 180px for others
      const tooltipWidth = currentData.datasets.length > 5 ? 240 : 180;
      const tooltipHalfWidth = tooltipWidth / 2;

      let adjustedX = x;
      // If tooltip would overflow on the right side, shift it left
      if (x + tooltipHalfWidth > CHART_CONFIG.width) {
        adjustedX = CHART_CONFIG.width - tooltipHalfWidth - 10;
      }
      // If tooltip would overflow on the left side, shift it right
      else if (x - tooltipHalfWidth < 0) {
        adjustedX = tooltipHalfWidth + 10;
      }

      tooltipPosition.value = {
        x: adjustedX,
        y: avgY - 20,
      };

      tooltipData.value = {
        time: currentData.labels[closestTimeIndex],
        index: closestTimeIndex,
        datasets: datasetsAtTime,
      };
    } else {
      hoveredPoint.value = null;
      tooltipData.value = null;
    }
  };

  const hideTooltip = () => {
    hoveredPoint.value = null;
    tooltipData.value = null;
  };

  return {
    hoveredPoint,
    tooltipData,
    tooltipPosition,
    handleMouseMove,
    hideTooltip,
  };
}
