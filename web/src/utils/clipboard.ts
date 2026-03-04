import copyToClipboard from "copy-to-clipboard";

/**
 * 复制文本到剪贴板
 * 使用 copy-to-clipboard 库，支持 HTTP/HTTPS 全环境
 */
export async function copy(text: string): Promise<boolean> {
  try {
    if (!text || text.trim() === "") {
      return false;
    }

    return copyToClipboard(text);
  } catch {
    return false;
  }
}
