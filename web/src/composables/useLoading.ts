import { ref, type Ref } from "vue";

/**
 * 加载状态管理 composable
 */
export function useLoading() {
  const loading = ref(false);

  /**
   * 带加载状态的异步函数包装器
   * @param fn 异步函数
   * @returns 包装后的函数
   */
  async function withLoading<T>(fn: () => Promise<T>): Promise<T> {
    loading.value = true;
    try {
      return await fn();
    } finally {
      loading.value = false;
    }
  }

  /**
   * 设置加载状态
   * @param state 加载状态
   */
  function setLoading(state: boolean): void {
    loading.value = state;
  }

  /**
   * 开始加载
   */
  function startLoading(): void {
    loading.value = true;
  }

  /**
   * 停止加载
   */
  function stopLoading(): void {
    loading.value = false;
  }

  /**
   * 创建独立的加载状态
   * @returns 加载状态相关函数
   */
  function createLoadingState(): {
    loading: Ref<boolean>;
    withLoading: <T>(fn: () => Promise<T>) => Promise<T>;
    setLoading: (state: boolean) => void;
    startLoading: () => void;
    stopLoading: () => void;
  } {
    const localLoading = ref(false);

    const localWithLoading = async <T>(fn: () => Promise<T>): Promise<T> => {
      localLoading.value = true;
      try {
        return await fn();
      } finally {
        localLoading.value = false;
      }
    };

    const localSetLoading = (state: boolean): void => {
      localLoading.value = state;
    };

    const localStartLoading = (): void => {
      localLoading.value = true;
    };

    const localStopLoading = (): void => {
      localLoading.value = false;
    };

    return {
      loading: localLoading,
      withLoading: localWithLoading,
      setLoading: localSetLoading,
      startLoading: localStartLoading,
      stopLoading: localStopLoading,
    };
  }

  return {
    loading,
    withLoading,
    setLoading,
    startLoading,
    stopLoading,
    createLoadingState,
  };
}

/**
 * 便捷默认导出
 */
export default useLoading;
