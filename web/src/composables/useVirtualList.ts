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
 * Virtual list composable for rendering only visible items
 * Optimized for grid layouts with dynamic item sizing
 */
export function useVirtualList<T = unknown>(
  items: Ref<T[]>,
  options: UseVirtualListOptions
): VirtualListResult<T> {
  const { itemHeight, containerHeight, overscan = 3 } = options;

  const containerRef = ref<HTMLElement | null>(null);
  const scrollTop = ref(0);

  // Get window width for responsive column calculation
  const { width: _windowWidth } = useWindowSize();

  // Calculate number of columns based on container width and item width (280px min)
  const columnCount = computed(() => {
    if (!containerRef.value) {
      return 3;
    }
    const containerWidth = containerRef.value.clientWidth - 32; // Subtract padding
    const itemWidth = 296; // 280px min + 16px gap
    return Math.max(1, Math.floor(containerWidth / itemWidth));
  });

  // Total rows needed to display all items
  const totalRows = computed(() => {
    return Math.ceil(items.value.length / columnCount.value);
  });

  // Total height of the virtual container
  const totalHeight = computed(() => {
    return totalRows.value * itemHeight;
  });

  // Calculate visible range
  const visibleRange = computed(() => {
    const startRow = Math.floor(scrollTop.value / itemHeight);
    const visibleRowCount = Math.ceil(containerHeight / itemHeight);

    return {
      start: Math.max(0, startRow - overscan),
      end: Math.min(totalRows.value, startRow + visibleRowCount + overscan),
    };
  });

  // Get visible items
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

  // Calculate wrapper transform
  const translateY = computed(() => {
    const { start } = visibleRange.value;
    return start * itemHeight;
  });

  // Scroll to specific index
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

  // Handle scroll events
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
 * Grid-based virtual list composable for KeyTable
 * Uses fixed row height for predictable layout
 */
export function useVirtualGrid<T = unknown>(
  items: Ref<T[]>,
  containerHeight: number
): VirtualListResult<T> {
  const itemHeight = 180; // Approximate height of a KeyCard
  const overscan = 2; // Render 2 extra rows above/below viewport

  return useVirtualList(items, {
    itemHeight,
    containerHeight,
    overscan,
  });
}
