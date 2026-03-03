import { computed, ref, type Ref } from "vue";
import { useWindowSize } from "@vueuse/core";

interface UseVirtualListOptions {
  itemHeight: number;
  containerHeight: number;
  overscan?: number;
}

interface VirtualListItem<T = unknown> {
  index: number;
  data: T;
}

interface VirtualListResult<T = unknown> {
  list: Ref<VirtualListItem<T>[]>;
  containerProps: {
    ref: Ref<HTMLElement | null>;
    onScroll: (event: Event) => void;
    style: {
      height: string;
      overflow: string;
    };
  };
  wrapperProps: {
    style: {
      height: string;
      transform: string;
    };
  };
  scrollTo: (index: number) => void;
}

/**
 * 虚拟列表 composable，仅渲染可见项
 * 为动态项目大小的网格布局优化
 */
export function useVirtualList<T = unknown>(
  items: Ref<T[]>,
  options: UseVirtualListOptions
): VirtualListResult<T> {
  const { itemHeight, containerHeight, overscan = 3 } = options;

  const containerRef = ref<HTMLElement | null>(null);
  const scrollTop = ref(0);

  // 获取窗口宽度以进行响应式列计算
  const { width: _windowWidth } = useWindowSize();

  // 根据容器宽度和项目宽度（最小 280px）计算列数
  const columnCount = computed(() => {
    if (!containerRef.value) {
      return 3;
    }
    const containerWidth = containerRef.value.clientWidth - 32; // 减去内边距
    const itemWidth = 296; // 280px 最小值 + 16px 间隙
    return Math.max(1, Math.floor(containerWidth / itemWidth));
  });

  // 显示所有项目所需的总行数
  const totalRows = computed(() => {
    return Math.ceil(items.value.length / columnCount.value);
  });

  // 虚拟容器的总高度
  const totalHeight = computed(() => {
    return totalRows.value * itemHeight;
  });

  // 计算可见范围
  const visibleRange = computed(() => {
    const startRow = Math.floor(scrollTop.value / itemHeight);
    const visibleRowCount = Math.ceil(containerHeight / itemHeight);

    return {
      start: Math.max(0, startRow - overscan),
      end: Math.min(totalRows.value, startRow + visibleRowCount + overscan),
    };
  });

  // 获取可见项目
  const list = computed(() => {
    const { start, end } = visibleRange.value;
    const result: VirtualListItem<T>[] = [];

    for (let row = start; row < end; row++) {
      for (let col = 0; col < columnCount.value; col++) {
        const itemIndex = row * columnCount.value + col;
        if (itemIndex < items.value.length) {
          result.push({
            index: itemIndex,
            data: items.value[itemIndex],
          });
        }
      }
    }

    return result;
  });

  // 计算包装器变换
  const translateY = computed(() => {
    const { start } = visibleRange.value;
    return start * itemHeight;
  });

  // 滚动到指定索引
  const scrollTo = (index: number) => {
    if (!containerRef.value) {
      return;
    }
    const row = Math.floor(index / columnCount.value);
    const targetScrollTop = row * itemHeight;
    containerRef.value.scrollTo({
      top: targetScrollTop,
      behavior: "smooth",
    });
  };

  // 处理滚动事件
  const handleScroll = (event: Event) => {
    const target = event.target as HTMLElement;
    scrollTop.value = target.scrollTop;
  };

  return {
    list,
    containerProps: {
      ref: containerRef,
      onScroll: handleScroll,
      style: {
        height: `${containerHeight}px`,
        overflow: "auto",
      },
    },
    wrapperProps: {
      style: {
        height: `${totalHeight.value}px`,
        transform: `translateY(${translateY.value}px)`,
      },
    },
    scrollTo,
  };
}

/**
 * 基于网格的虚拟列表 composable，用于 KeyTable
 * 使用固定的行高度以获得可预测的布局
 */
export function useVirtualGrid<T = unknown>(
  items: Ref<T[]>,
  containerHeight: number
): VirtualListResult<T> {
  const itemHeight = 180; // KeyCard 的近似高度
  const overscan = 2; // 在视口上方/下方额外渲染 2 行

  return useVirtualList(items, {
    itemHeight,
    containerHeight,
    overscan,
  });
}
