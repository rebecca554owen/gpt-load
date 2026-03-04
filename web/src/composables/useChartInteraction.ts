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
      .filter(item => !hiddenDatasetIndices().has(item.index) && item.value > 0)
      .sort((a, b) => b.value - a.value); // 按值降序排序

    if (closestTimeIndex >= 0) {
      hoveredPoint.value = {
        datasetIndex: 0,
        pointIndex: closestTimeIndex,
        x: mouseX,
        y: mouseY,
      };

      const avgY =
        datasetsAtTime.reduce((sum, item) => sum + getYPosition(item.value), 0) /
        datasetsAtTime.length;

      // 计算带有视口边界检测的 tooltip 位置
      // token_speed 视图的 tooltip 宽度：280px（组 - 模型标签），其他视图：180px
      const isTokenSpeedView = currentData.datasets.some(d => d.label && d.label.includes(" - "));
      const tooltipWidth = isTokenSpeedView ? 280 : 180;

      // 计算相对于视口的位置
      const svgRect = svg.getBoundingClientRect();
      const xInSvg = getXPosition(closestTimeIndex);

      // CSS 使用 transform: translate(-50%, -100%)，所以我们需要考虑该偏移
      // tooltip 中心（50%）不应超过视口边界
      const tooltipHalfWidth = (tooltipWidth / 2 / svgRect.width) * CHART_CONFIG.width;

      // 检查 SVG 边界（考虑 transform 偏移）
      const maxX = CHART_CONFIG.width - tooltipHalfWidth;
      const minX = tooltipHalfWidth;

      let adjustedXInSvg = xInSvg;
      if (xInSvg > maxX) {
        adjustedXInSvg = maxX;
      } else if (xInSvg < minX) {
        adjustedXInSvg = minX;
      }

      // 还要检查视口边界以确保安全
      const xInViewport = svgRect.left + (xInSvg / CHART_CONFIG.width) * svgRect.width;
      const viewportWidth = window.innerWidth;
      const padding = 16;

      // 如果 tooltip 在视口右侧溢出，向左移动
      if (xInViewport + tooltipWidth / 2 > viewportWidth - padding) {
        const maxViewportX = viewportWidth - padding - tooltipWidth / 2;
        const adjustedViewportX = Math.min(xInViewport, maxViewportX);
        adjustedXInSvg = ((adjustedViewportX - svgRect.left) / svgRect.width) * CHART_CONFIG.width;
      }
      // 如果 tooltip 在视口左侧溢出，向右移动
      else if (xInViewport - tooltipWidth / 2 < padding) {
        const minViewportX = padding + tooltipWidth / 2;
        const adjustedViewportX = Math.max(xInViewport, minViewportX);
        adjustedXInSvg = ((adjustedViewportX - svgRect.left) / svgRect.width) * CHART_CONFIG.width;
      }

      tooltipPosition.value = {
        x: adjustedXInSvg,
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
