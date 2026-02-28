import { copy } from "@/utils/clipboard";
import { useMessage } from "naive-ui";
import { useI18n } from "vue-i18n";

export function useCopy() {
  const message = useMessage();
  const { t } = useI18n();

  async function copyWithFeedback(
    content: string,
    successKey: string = "common.copySuccess",
    errorKey: string = "common.copyFailed",
    params?: Record<string, unknown>
  ): Promise<boolean> {
    const success = await copy(content);
    if (success) {
      message.success(params ? t(successKey, params) : t(successKey));
    } else {
      message.error(params ? t(errorKey, params) : t(errorKey));
    }
    return success;
  }

  return {
    copyWithFeedback,
  };
}
