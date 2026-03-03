import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { ChartData } from "@/types/models";
import type { TimeRangeHours } from "./useChartData";
import { CHART_CONFIG, Y_AXIS, NUMBER_FORMAT } from "@/constants/chart";

const PRECISION_MULTIPLIER = 1000000;

const LABEL_COUNTS: Record<TimeRangeHours, number> = {
  1: 6,
  5: 6,
  24: 6,
  168: 6,
  720: 6,
} as const;

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

    return Array.from({ length: Y_AXIS.tickCount }, (_, i) => {
      const tickValue = min + i * step;
      // 四舍五入以避免浮点精度问题
      return Math.round(tickValue * PRECISION_MULTIPLIER) / PRECISION_MULTIPLIER;
    });
  });

  const formatTimeLabel = (_isoString: string, index: number, totalLabels: number) => {
    const range = timeRange();

    // 根据总标签数动态计算标签位置
    // 在以下位置显示标签：开始、约 25%、约 50%、约 75%、结束（最后一点为 "realtime"）
    const labelPositions = calculateLabelPositions(totalLabels, range);

    // 检查当前索引是否应显示标签
    const labelIndex = labelPositions.findIndex(pos => pos.index === index);
    if (labelIndex === -1) {
      return "";
    }

    // 返回相应的标签文本
    const labelText = labelPositions[labelIndex].text;
    return labelText === "realtime" ? t("dashboard.realtime") : labelText;
  };

  // 计算标签位置的辅助函数
  const calculateLabelPositions = (totalLabels: number, rangeHours: TimeRangeHours) => {
    const positions: Array<{ index: number; text: string }> = [];

    if (totalLabels <= 1) {
      return [{ index: 0, text: t("dashboard.realtime") }];
    }

    // 根据范围定义要显示的标签数量
    const labelCount = getLabelCount(rangeHours);

    // 计算均匀分布的位置
    for (let i = 0; i < labelCount; i++) {
      const index = Math.floor((i / (labelCount - 1)) * (totalLabels - 1));

      // 根据位置和范围生成标签文本
      const text = generateLabelText(i, labelCount, rangeHours);
      positions.push({ index, text });
    }

    return positions;
  };

  // 根据时间范围获取要显示的标签数量
  const getLabelCount = (rangeHours: TimeRangeHours): number => LABEL_COUNTS[rangeHours] ?? 6;

  // 根据位置生成标签文本
  // 所有范围都有 6 个标签，5 个时间间隔 + realtime
  const generateLabelText = (
    positionIndex: number,
    totalLabels: number,
    rangeHours: TimeRangeHours
  ): string => {
    const isRealtime = positionIndex === totalLabels - 1;
    if (isRealtime) {
      return "realtime";
    }

    // 5 个间隔（索引 0-4），根据范围计算值
    const intervalIndex = totalLabels - 1 - positionIndex; // 索引 0-4 对应 5, 4, 3, 2, 1

    if (rangeHours === 1) {
      // 1 小时范围：6 个数据点，6 个标签（1:1 映射）
      // 标签在索引 0,1,2,3,4,5
      const minutesFromEnd = (5 - positionIndex) * 10;
      return t("dashboard.minutesAgo", { value: minutesFromEnd });
    } else if (rangeHours === 5) {
      // 5 小时范围：5, 4, 3, 2, 1 小时
      const hoursFromEnd = intervalIndex;
      return t("dashboard.hoursAgo", { value: hoursFromEnd });
    } else if (rangeHours === 24) {
      // 24 小时范围：20, 16, 12, 8, 4 小时
      const hoursFromEnd = intervalIndex * 4;
      return t("dashboard.hoursAgo", { value: hoursFromEnd });
    } else if (rangeHours === 168) {
      // 1 周范围：5, 4, 3, 2, 1 天
      const daysFromEnd = intervalIndex;
      return t("dashboard.daysAgo", { value: daysFromEnd });
    } else {
      // 1 月范围：25, 20, 15, 10, 5 天
      const daysFromEnd = intervalIndex * 5;
      return t("dashboard.daysAgo", { value: daysFromEnd });
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
    const range = max - min;
    if (range === 0) {
      return padding.top + plotHeight / 2;
    }
    const ratio = (value - min) / range;
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

    // 使用三次贝塞尔曲线实现平滑线条
    const pathCommands: string[] = [];
    const points = data.map((value, index) => ({
      x: getXPosition(index),
      y: getYPosition(value),
    }));

    // 起点
    pathCommands.push(`M ${points[0].x} ${points[0].y}`);

    // 使用三次贝塞尔生成平滑曲线
    for (let i = 0; i < points.length - 1; i++) {
      const p0 = points[Math.max(0, i - 1)];
      const p1 = points[i];
      const p2 = points[i + 1];
      const p3 = points[Math.min(points.length - 1, i + 2)];

      // 如果两个值都为零，使用直线
      if (data[i] === 0 && data[i + 1] === 0) {
        pathCommands.push(`L ${p2.x} ${p2.y}`);
      } else {
        // 计算平滑曲线的控制点，减小张力
        const tension = 0.2; // 减小张力以获得更平滑的曲线
        const cp1x = p1.x + (p2.x - p0.x) * tension;
        let cp1y = p1.y + (p2.y - p0.y) * tension;
        const cp2x = p2.x - (p3.x - p1.x) * tension;
        let cp2y = p2.y - (p3.y - p1.y) * tension;

        // 将控制点限制在 p1.y 和 p2.y 范围之间
        // 这样可以防止曲线低于任一端点
        const upperY = Math.min(p1.y, p2.y); // 屏幕上方（Y 值较小）
        const lowerY = Math.max(p1.y, p2.y); // 屏幕下方（Y 值较大）

        // 控制点不应超出两个端点的范围
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

    // 修复浮点精度问题
    const roundedValue = Math.round(value * PRECISION_MULTIPLIER) / PRECISION_MULTIPLIER;

    if (roundedValue < 1000) {
      // 对于小值，显示适当的小数位数
      if (roundedValue < 10) {
        return roundedValue.toFixed(2);
      }
      if (roundedValue < 100) {
        return roundedValue.toFixed(1);
      }
      return Math.round(roundedValue).toString();
    }

    if (roundedValue < NUMBER_FORMAT.million) {
      return `${(roundedValue / 1000).toFixed(1)}K`;
    }

    return `${(roundedValue / NUMBER_FORMAT.million).toFixed(1)}M`;
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
