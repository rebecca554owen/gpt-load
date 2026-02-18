import { ref } from "vue";
import type { DialogReactive } from "naive-ui";

export interface ConfirmActionOptions {
  title: string;
  content: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => Promise<void> | void;
  onError?: (error: unknown) => void;
}

export function useConfirmDialog() {
  const isLoading = ref(false);

  async function confirmAction(
    dialog: (options: Record<string, unknown>) => DialogReactive,
    options: ConfirmActionOptions
  ) {
    if (isLoading.value) {
      return;
    }

    const d = dialog({
      type: "warning",
      title: options.title,
      content: options.content,
      positiveText: options.confirmText,
      negativeText: options.cancelText,
      onPositiveClick: async () => {
        isLoading.value = true;
        d.loading = true;

        try {
          await options.onConfirm();
        } catch (error) {
          options.onError?.(error);
        } finally {
          d.loading = false;
          isLoading.value = false;
        }
      },
    } as Record<string, unknown>);
  }

  function withLoading<T>(fn: () => Promise<T>): () => Promise<T> {
    return async () => {
      isLoading.value = true;
      try {
        return await fn();
      } finally {
        isLoading.value = false;
      }
    };
  }

  return {
    isLoading,
    confirmAction,
    withLoading,
  };
}
