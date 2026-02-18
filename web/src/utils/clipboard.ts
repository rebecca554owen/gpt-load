/**
 * Copy text
 */
export async function copy(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (e) {
      console.error("Failed to copy using navigator.clipboard:", e);
    }
  }

  try {
    const input = document.createElement("input");
    input.style.position = "fixed";
    input.style.opacity = "0";
    input.value = text;
    document.body.appendChild(input);
    input.select();
    const result = document.execCommand("copy");
    document.body.removeChild(input);
    return result;
  } catch (e) {
    console.error("Failed to copy using execCommand fallback:", e);
    return false;
  }
}
