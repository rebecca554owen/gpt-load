/**
 * Copy text
 */
const FALLBACK_TEXTAREA_STYLES: Partial<CSSStyleDeclaration> = {
  position: "fixed",
  top: "0",
  left: "0",
  width: "2em",
  height: "2em",
  padding: "0",
  border: "none",
  outline: "none",
  boxShadow: "none",
  background: "transparent",
  opacity: "0.01",
};

export async function copy(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (e) {
      console.error("Failed to copy using navigator.clipboard:", e);
    }
  }

  return copyWithFallback(text);
}

function copyWithFallback(text: string): boolean {
  const textarea = document.createElement("textarea");
  textarea.value = text;
  Object.assign(textarea.style, FALLBACK_TEXTAREA_STYLES);
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  textarea.setSelectionRange(0, textarea.value.length);
  const result = document.execCommand("copy");
  document.body.removeChild(textarea);
  return result;
}
