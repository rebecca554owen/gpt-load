import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { ChartData } from "@/types/models";
import type { TimeRangeHours } from "./useChartData";
import { CHART_CONFIG, Y_AXIS, NUMBER_FORMAT } from "@/constants/chart";

interface Padding {
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export function useChartRendering(
  chartData: () => ChartData | null,
  timeRange: () => TimeRangeHours,
  hiddenDatasetIndices: () => Set<number>,
  chartWidth = CHART_CONFIG.width,
  chartHeight = CHART_CONFIG.height,
  padding: Padding = { ...CHART_CONFIG.padding }
) {
  const { t } = useI18n();
  const plotWidth = chartWidth - padding.left - padding.right;
  const plotHeight = chartHeight - padding.top - padding.bottom;

  const dataRange = computed(() => {
    const currentData = chartData();
    if (!currentData || currentData.datasets.length === 0) {
      return { min: 0, max: Y_AXIS.defaultMax };
    }

    const allValues = currentData.datasets
      .filter((_dataset, datasetIndex) => !hiddenDatasetIndices().has(datasetIndex))
      .flatMap(dataset => dataset.data);

    const max = Math.max(...allValues, 0);
    const min = Math.min(...allValues, 0);

    if (max === 0 && min === 0) {
      return { min: 0, max: Y_AXIS.defaultMax };
    }

    const paddingValue = Math.max((max - min) * Y_AXIS.minPaddingRatio, 1);
    return {
      min: Math.max(0, min - paddingValue),
      max: max + paddingValue,
    };
  });

  const yTicks = computed(() => {
    const { min, max } = dataRange.value;
    const range = max - min;
    const step = range / (Y_AXIS.tickCount - 1);

    return Array.from({ length: Y_AXIS.tickCount }, (_, i) => min + i * step);
  });

  const formatTimeLabel = (_isoString: string, index: number, totalLabels: number) => {
    const range = timeRange();

    // Calculate label positions dynamically based on total labels
    // Show labels at: start, ~25%, ~50%, ~75%, end (with "realtime" at the last point)
    const labelPositions = calculateLabelPositions(totalLabels, range);

    // Check if current index should show a label
    const labelIndex = labelPositions.findIndex(pos => pos.index === index);
    if (labelIndex === -1) {
      return "";
    }

    // Return the corresponding label text
    const labelText = labelPositions[labelIndex].text;
    return labelText === "realtime" ? t("dashboard.realtime") : labelText;
  };

  // Helper function to calculate label positions dynamically
  const calculateLabelPositions = (totalLabels: number, rangeHours: TimeRangeHours) => {
    const positions: Array<{ index: number; text: string }> = [];

    if (totalLabels <= 1) {
      return [{ index: 0, text: t("dashboard.realtime") }];
    }

    // Define how many labels to show based on range
    const labelCount = getLabelCount(rangeHours);

    // Calculate evenly distributed positions
    for (let i = 0; i < labelCount; i++) {
      const index = Math.floor((i / (labelCount - 1)) * (totalLabels - 1));

      // Generate label text based on position and range
      const text = generateLabelText(i, labelCount, rangeHours);
      positions.push({ index, text });
    }

    return positions;
  };

  // Get the number of labels to show based on time range
  const getLabelCount = (rangeHours: TimeRangeHours): number => {
    if (rangeHours === 1) return 6; // 60min, 50min, 40min, 30min, 20min, 10min, realtime
    if (rangeHours === 5) return 5; // 5h, 4h, 3h, 2h, realtime
    if (rangeHours === 24) return 7; // 24h, 20h, 16h, 12h, 8h, 4h, realtime
    if (rangeHours === 168) return 7; // 6d, 5d, 4d, 3d, 2d, 1d, realtime
    return 6; // 30d, 24d, 18d, 12d, 6d, realtime
  };

  // Generate label text based on position
  const generateLabelText = (positionIndex: number, totalLabels: number, rangeHours: TimeRangeHours): string => {
    const isRealtime = positionIndex === totalLabels - 1;
    if (isRealtime) {
      return "realtime";
    }

    if (rangeHours === 1) {
      // 1 hour range: show minutes (60, 50, 40, 30, 20, 10)
      const minutesFromEnd = (totalLabels - 1 - positionIndex) * 10;
      return `${minutesFromEnd}分钟前`;
    } else if (rangeHours === 5) {
      // 5 hour range: show hours (5, 4, 3, 2)
      const hoursFromEnd = totalLabels - 1 - positionIndex;
      return `${hoursFromEnd}小时前`;
    } else if (rangeHours === 24) {
      // 24 hour range: show hours (24, 20, 16, 12, 8, 4)
      const hoursFromEnd = Math.floor((totalLabels - 1 - positionIndex) * 4);
      return `${hoursFromEnd}小时前`;
    } else if (rangeHours === 168) {
      // 1 week range: show days (6, 5, 4, 3, 2, 1)
      const daysFromEnd = totalLabels - 1 - positionIndex;
      return `${daysFromEnd}天前`;
    } else {
      // 1 month range: show days (30, 24, 18, 12, 6)
      const daysFromEnd = Math.floor((totalLabels - 1 - positionIndex) * 6);
      return `${daysFromEnd}天前`;
    }
  };

  const visibleLabels = computed(() => {
    const currentData = chartData();
    if (!currentData) {
      return [];
    }

    const labels = currentData.labels;

    const result = labels
      .map((label, index) => ({ text: formatTimeLabel(label, index, labels.length), index }))
      .filter(label => label.text !== "");

    return result;
  });

  const getXPosition = (index: number): number => {
    const currentData = chartData();
    if (!currentData) {
      return 0;
    }
    const totalPoints = currentData.labels.length;
    if (totalPoints <= 1) {
      return padding.left + plotWidth / 2;
    }
    return padding.left + (index / (totalPoints - 1)) * plotWidth;
  };

  const getYPosition = (value: number): number => {
    const { min, max } = dataRange.value;
    const ratio = (value - min) / (max - min);
    return padding.top + (1 - ratio) * plotHeight;
  };

  const generateLinePath = (data: number[]): string => {
    if (data.length === 0) {
      return "";
    }

    if (data.length === 1) {
      const x = getXPosition(0);
      const y = getYPosition(data[0]);
      return `M ${x} ${y}`;
    }

    // Use cubic bezier curves for smooth lines
    const pathCommands: string[] = [];
    const points = data.map((value, index) => ({
      x: getXPosition(index),
      y: getYPosition(value),
    }));

    // Start point
    pathCommands.push(`M ${points[0].x} ${points[0].y}`);

    // Generate smooth curve using cubic bezier
    for (let i = 0; i < points.length - 1; i++) {
      const p0 = points[Math.max(0, i - 1)];
      const p1 = points[i];
      const p2 = points[i + 1];
      const p3 = points[Math.min(points.length - 1, i + 2)];

      // Use straight line if values are both zero
      if (data[i] === 0 && data[i + 1] === 0) {
        pathCommands.push(`L ${p2.x} ${p2.y}`);
      } else {
        // Calculate control points for smooth curve with reduced tension
        const tension = 0.2; // Reduced tension for smoother curves
        const cp1x = p1.x + (p2.x - p0.x) * tension;
        let cp1y = p1.y + (p2.y - p0.y) * tension;
        const cp2x = p2.x - (p3.x - p1.x) * tension;
        let cp2y = p2.y - (p3.y - p1.y) * tension;

        // Clamp control points to stay between p1.y and p2.y range
        // This prevents curves from going below either endpoint
        const upperY = Math.min(p1.y, p2.y); // Higher on screen (smaller Y value)
        const lowerY = Math.max(p1.y, p2.y); // Lower on screen (larger Y value)

        // Control points should not extend beyond the range of the two endpoints
        cp1y = Math.max(upperY, Math.min(lowerY, cp1y));
        cp2y = Math.max(upperY, Math.min(lowerY, cp2y));

        pathCommands.push(`C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${p2.x} ${p2.y}`);
      }
    }

    return pathCommands.join(" ");
  };

  const generateAreaPath = (data: number[]): string => {
    if (data.length === 0) {
      return "";
    }

    const firstX = getXPosition(0);
    const lastX = getXPosition(data.length - 1);

    return [
      generateLinePath(data),
      `L ${lastX} ${getYPosition(0)}`,
      `L ${firstX} ${getYPosition(0)}`,
      "Z",
    ].join(" ");
  };

  const formatNumber = (value: number): string => {
    if (value === 0) {
      return "0";
    }

    if (value < 1000) {
      return value.toString();
    }

    if (value < NUMBER_FORMAT.million) {
      return `${(value / 1000).toFixed(1)}K`;
    }

    return `${(value / NUMBER_FORMAT.million).toFixed(1)}M`;
  };

  return {
    chartWidth,
    chartHeight,
    padding,
    plotWidth,
    plotHeight,
    dataRange,
    yTicks,
    visibleLabels,
    getXPosition,
    getYPosition,
    generateLinePath,
    generateAreaPath,
    formatNumber,
    formatTimeLabel,
  };
}
