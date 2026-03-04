import { copy } from "@/utils/clipboard";
import { useMessage } from "naive-ui";

export function useCopy() {
  const message = useMessage();

  async function copyWithFeedback(content: string): Promise<boolean> {
    const success = await copy(content);

    if (success) {
      message.success("已复制到剪贴板");
    } else {
      message.error("复制失败");
    }

    return success;
  }

  return {
    copyWithFeedback,
  };
}
