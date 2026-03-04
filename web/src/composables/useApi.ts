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
 * 用于 API 错误处理和成功通知的通用 composable
 */
export function useApi() {
  const message = useMessage();
  const { t } = useI18n();

  /**
   * 解析响应数据错误
   * @param data 响应数据
   * @returns 解析后的错误消息字符串或 null
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
   * 解析错误对象
   * @param errorObj 错误对象
   * @returns 解析后的错误消息字符串或 null
   */
  function parseErrorObject(errorObj: Partial<AxiosError>): string | null {
    // 检查对象的 message 属性
    if (errorObj.message) {
      return errorObj.message;
    }

    // 检查状态码相关错误
    if (errorObj.response?.status) {
      const status = errorObj.response.status;
      return t("common.requestFailed", { status });
    }

    return null;
  }

  /**
   * 解析 API 错误消息
   * @param error 错误对象
   * @returns 解析后的错误消息字符串
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

      // 检查响应数据中的消息
      if (errorObj.response?.data) {
        const responseError = parseResponseDataError(errorObj.response.data);
        if (responseError) {
          return responseError;
        }
      }

      // 检查错误对象属性
      const objectError = parseErrorObject(errorObj);
      if (objectError) {
        return objectError;
      }
    }

    return t("common.operationFailed");
  }

  /**
   * 处理 API 错误
   * @param error 错误对象
   * @param options 配置选项
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
   * 处理 API 成功
   * @param msg 成功消息
   */
  function handleApiSuccess(msg?: string) {
    message.success(msg || t("common.operationSuccess"));
  }

  /**
   * 处理 API 警告
   * @param msg 警告消息
   */
  function handleApiWarning(msg?: string) {
    message.warning(msg || t("common.operationWarning"));
  }

  /**
   * 处理 API 信息
   * @param msg 信息消息
   */
  function handleApiInfo(msg?: string) {
    message.info(msg || t("common.operationInfo"));
  }

  /**
   * 通用 API 请求包装器
   * @param apiCall API 调用函数
   * @param options 配置选项
   * @returns API 调用结果
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
   * 批量 API 请求处理器
   * @param apiCalls API 调用数组
   * @param options 配置选项
   * @returns 处理结果
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
    // 错误处理
    parseApiError,
    handleApiError,

    // 成功处理
    handleApiSuccess,
    handleApiWarning,
    handleApiInfo,

    // 包装器
    withApiCall,
    withBatchApiCall,
  };
}

/**
 * 便捷默认导出
 */
export default useApi;
