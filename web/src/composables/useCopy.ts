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
    if (!content || content.trim() === "") {
      message.warning(t("common.noContentToCopy"));
      console.warn("[useCopy] Attempted to copy empty content");
      return false;
    }

    const preview = content.length > 50 ? content.slice(0, 50) + "..." : content;
    console.log(`[useCopy] Attempting to copy: "${preview}" (${content.length} chars)`);

    const success = await copy(content);
    if (success) {
      const successMessage = params
        ? t(successKey, params)
        : t(successKey);
      message.success(`${successMessage}: ${preview}`);
      console.log(`[useCopy] Successfully copied: "${preview}"`);
    } else {
      const errorMessage = params
        ? t(errorKey, params)
        : t(errorKey);
      message.error(`${errorMessage}: ${preview}`);
      console.error(`[useCopy] Failed to copy: "${preview}"`);
    }
    return success;
  }

  return {
    copyWithFeedback,
  };
}
