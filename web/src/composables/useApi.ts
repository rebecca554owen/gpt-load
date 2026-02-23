import { useMessage } from "naive-ui";
import { useI18n } from "vue-i18n";

interface ErrorResponseData {
  message?: string;
}

interface AxiosError {
  message?: string;
  response?: {
    data?: ErrorResponseData | Record<string, unknown>;
    status?: number;
  };
}

/**
 * Common composable for API error handling and success notifications
 */
export function useApi() {
  const message = useMessage();
  const { t } = useI18n();

  /**
   * Parse response data error
   * @param data Response data
   * @returns Parsed error message string or null
   */
  function parseResponseDataError(
    data: ErrorResponseData | Record<string, unknown>
  ): string | null {
    if (data && typeof data === "object" && "message" in data) {
      return (data as ErrorResponseData).message || "";
    }

    if (typeof data === "string") {
      return data;
    }

    try {
      return JSON.stringify(data);
    } catch {
      return String(data);
    }
  }

  /**
   * Parse error object
   * @param errorObj Error object
   * @returns Parsed error message string or null
   */
  function parseErrorObject(errorObj: Partial<AxiosError>): string | null {
    // Check message property of object
    if (errorObj.message) {
      return errorObj.message;
    }

    // Check status code related errors
    if (errorObj.response?.status) {
      const status = errorObj.response.status;
      return t("common.requestFailed", { status });
    }

    return null;
  }

  /**
   * Parse API error message
   * @param error Error object
   * @returns Parsed error message string
   */
  function parseApiError(error: unknown): string {
    if (!error) {
      return t("common.operationFailed");
    }

    if (typeof error === "string") {
      return error;
    }

    if (typeof error === "object" && error !== null) {
      const errorObj = error as Partial<AxiosError>;

      // Check message in response data
      if (errorObj.response?.data) {
        const responseError = parseResponseDataError(errorObj.response.data);
        if (responseError) {
          return responseError;
        }
      }

      // Check error object properties
      const objectError = parseErrorObject(errorObj);
      if (objectError) {
        return objectError;
      }
    }

    return t("common.operationFailed");
  }

  /**
   * Handle API error
   * @param error Error object
   * @param options Configuration options
   */
  function handleApiError(
    error: unknown,
    options?: {
      duration?: number;
      keepAliveOnHover?: boolean;
      closable?: boolean;
    }
  ) {
    const errorMessage = parseApiError(error);

    message.error(errorMessage, {
      duration: options?.duration || 5000,
      keepAliveOnHover: options?.keepAliveOnHover ?? true,
      closable: options?.closable ?? true,
    });
  }

  /**
   * Handle API success
   * @param msg Success message
   */
  function handleApiSuccess(msg?: string) {
    message.success(msg || t("common.operationSuccess"));
  }

  /**
   * Handle API warning
   * @param msg Warning message
   */
  function handleApiWarning(msg?: string) {
    message.warning(msg || t("common.operationWarning"));
  }

  /**
   * Handle API info
   * @param msg Info message
   */
  function handleApiInfo(msg?: string) {
    message.info(msg || t("common.operationInfo"));
  }

  /**
   * Generic API request wrapper
   * @param apiCall API call function
   * @param options Configuration options
   * @returns API call result
   */
  async function withApiCall<T>(
    apiCall: () => Promise<T>,
    options?: {
      successMessage?: string;
      errorMessage?: string;
      showSuccess?: boolean;
      showError?: boolean;
      onSuccess?: (result: T) => void;
      onError?: (error: unknown) => void;
    }
  ): Promise<T | null> {
    try {
      const result = await apiCall();

      if (options?.showSuccess !== false) {
        handleApiSuccess(options?.successMessage);
      }

      if (options?.onSuccess) {
        options.onSuccess(result);
      }

      return result;
    } catch (error) {
      if (options?.showError !== false) {
        handleApiError(error);
      }

      if (options?.onError) {
        options.onError(error);
      }

      return null;
    }
  }

  /**
   * Batch API request handler
   * @param apiCalls Array of API calls
   * @param options Configuration options
   * @returns Processing results
   */
  async function withBatchApiCall<T>(
    apiCalls: Array<() => Promise<T>>,
    options?: {
      successMessage?: string;
      errorMessage?: string;
      stopOnFirstError?: boolean;
      showSuccess?: boolean;
      showError?: boolean;
    }
  ): Promise<{
    results: Array<T | null>;
    errors: unknown[];
    successCount: number;
    errorCount: number;
  }> {
    const results: Array<T | null> = [];
    const errors: unknown[] = [];
    let successCount = 0;
    let errorCount = 0;

    for (const apiCall of apiCalls) {
      try {
        const result = await apiCall();
        results.push(result);
        successCount++;
      } catch (error) {
        results.push(null);
        errors.push(error);
        errorCount++;

        if (options?.stopOnFirstError) {
          break;
        }
      }
    }

    if (errorCount > 0 && options?.showError !== false) {
      handleApiError(errors[0], {
        duration: 8000,
        keepAliveOnHover: true,
      });
    } else if (successCount > 0 && options?.showSuccess !== false) {
      handleApiSuccess(
        options?.successMessage ||
          t("common.batchOperationSuccess", {
            success: successCount,
            total: apiCalls.length,
          })
      );
    }

    return {
      results,
      errors,
      successCount,
      errorCount,
    };
  }

  return {
    // Error handling
    parseApiError,
    handleApiError,

    // Success handling
    handleApiSuccess,
    handleApiWarning,
    handleApiInfo,

    // Wrappers
    withApiCall,
    withBatchApiCall,
  };
}

/**
 * Default export for convenience
 */
export default useApi;
