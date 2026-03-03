import { ref } from "vue";
import { useDebounceFn } from "@vueuse/core";

/**
 * 图表性能优化 composable
 * 处理防抖交互和渲染优化
 */
export function useChartPerformance() {
  // 跟踪渲染状态
  const isRendering = ref(false);
  const renderRequested = ref(false);

  /**
   * 防抖的 tooltip 更新，减少快速鼠标移动时的重新渲染
   */
  const debouncedTooltipUpdate = useDebounceFn((callback: () => void) => {
    callback();
  }, 16); // 60fps = ~16ms per frame

  /**
   * 节流渲染请求，避免不必要的重新渲染
   */
  const requestRender = (callback: () => void) => {
    if (renderRequested.value) {
      return;
    }

    renderRequested.value = true;

    requestAnimationFrame(() => {
      renderRequested.value = false;
      isRendering.value = true;

      try {
        callback();
      } finally {
        isRendering.value = false;
      }
    });
  };

  /**
   * 带缓存键的记忆化路径生成
   */
  const pathCache = new Map<string, string>();

  const getCachedPath = (key: string, generator: () => string): string => {
    const cached = pathCache.get(key);
    if (cached !== undefined) {
      return cached;
    }

    const path = generator();
    pathCache.set(key, path);
    return path;
  };

  const clearPathCache = () => {
    pathCache.clear();
  };

  return {
    isRendering,
    debouncedTooltipUpdate,
    requestRender,
    getCachedPath,
    clearPathCache,
  };
}

/**
 * 大数据集的优化数据采样
 * 在保持曲线形状的同时减少数据点数量
 */
export function useDataSampling() {
  /**
   * 采样数据点以减少渲染负载
   * @param data - 原始数据数组
   * @param maxPoints - 要返回的最大点数
   * @returns 采样后的数据数组
   */
  const sampleData = (data: number[], maxPoints: number): number[] => {
    if (data.length <= maxPoints) {
      return data;
    }

    const step = Math.ceil(data.length / maxPoints);
    const sampled: number[] = [];

    for (let i = 0; i < data.length; i += step) {
      sampled.push(data[i]);
    }

    // Always include the last point
    if (sampled[sampled.length - 1] !== data[data.length - 1]) {
      sampled.push(data[data.length - 1]);
    }

    return sampled;
  };

  /**
   * 根据图表宽度计算最佳最大点数
   * @param chartWidth - 图表宽度（像素）
   * @returns 最佳数据点数量
   */
  const getOptimalMaxPoints = (chartWidth: number): number => {
    // 每两个像素一个点足以获得平滑的曲线
    return Math.ceil(chartWidth / 2);
  };

  return {
    sampleData,
    getOptimalMaxPoints,
  };
}
