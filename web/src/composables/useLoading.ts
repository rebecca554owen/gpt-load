import { ref, type Ref } from "vue";

/**
 * Loading state management composable
 */
export function useLoading() {
  const loading = ref(false);

  /**
   * Async function wrapper with loading state
   * @param fn Async function
   * @returns Wrapped function
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
   * Set loading state
   * @param state Loading state
   */
  function setLoading(state: boolean): void {
    loading.value = state;
  }

  /**
   * Start loading
   */
  function startLoading(): void {
    loading.value = true;
  }

  /**
   * Stop loading
   */
  function stopLoading(): void {
    loading.value = false;
  }

  /**
   * Create independent loading state
   * @returns Loading state related functions
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
 * Default export for convenience
 */
export default useLoading;
