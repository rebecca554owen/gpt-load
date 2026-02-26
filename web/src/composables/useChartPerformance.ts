import { ref } from "vue";
import { useDebounceFn } from "@vueuse/core";

/**
 * Chart performance optimization composable
 * Handles debounced interactions and rendering optimizations
 */
export function useChartPerformance() {
  // Track rendering state
  const isRendering = ref(false);
  const renderRequested = ref(false);

  /**
   * Debounced tooltip update to reduce re-renders during rapid mouse movements
   */
  const debouncedTooltipUpdate = useDebounceFn((callback: () => void) => {
    callback();
  }, 16); // 60fps = ~16ms per frame

  /**
   * Throttle render requests to avoid unnecessary re-renders
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
   * Memoized path generation with cache key
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
 * Optimized data sampling for large datasets
 * Reduces number of points while preserving curve shape
 */
export function useDataSampling() {
  /**
   * Sample data points to reduce rendering load
   * @param data - Original data array
   * @param maxPoints - Maximum number of points to return
   * @returns Sampled data array
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
   * Calculate optimal max points based on chart width
   * @param chartWidth - Width of the chart in pixels
   * @returns Optimal number of data points
   */
  const getOptimalMaxPoints = (chartWidth: number): number => {
    // One point per 2 pixels is enough for smooth curves
    return Math.ceil(chartWidth / 2);
  };

  return {
    sampleData,
    getOptimalMaxPoints,
  };
}
