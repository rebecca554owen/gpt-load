const cssVarMap: Record<string, string> = {
  "dashboard.success_requests": "--color-request-count",
  "dashboard.failed_requests": "--color-error-rate",
  "dashboard.prompt_tokens": "--color-prompt-tokens",
  "dashboard.uncached_prompt_tokens": "--color-prompt-tokens",
  "dashboard.completion_tokens": "--color-completion-tokens",
  "dashboard.cached_tokens": "--color-cached-tokens",
  "dashboard.total_tokens": "--color-total-tokens",
  "dashboard.input_cached_tokens": "--color-cached-tokens",
  "dashboard.input_non_cached_tokens": "--color-prompt-tokens",
};

export function useChartColors() {
  const getDatasetColor = (labelKey: string | undefined, label: string): string => {
    const key = labelKey || label;
    if (cssVarMap[key]) {
      return `var(${cssVarMap[key]})`;
    }
    return "var(--color-request-count)";
  };

  const isErrorDataset = (label: string, t: (key: string) => string): boolean => {
    // First check if the original label contains error keywords
    if (
      label.includes("失败") ||
      label.includes("Failed") ||
      label.includes("失敗") ||
      label.includes("Error") ||
      label.includes("エラー")
    ) {
      return true;
    }

    // Only try to translate when label looks like an i18n key (contains dot)
    if (label.includes(".")) {
      try {
        const translatedLabel = t(label);
        // Check if translated text contains error keywords
        if (
          translatedLabel.includes("失败") ||
          translatedLabel.includes("Error") ||
          translatedLabel.includes("エラー")
        ) {
          return true;
        }
      } catch {
        // Translation failed, ignore
      }
    }

    return false;
  };

  return {
    getDatasetColor,
    isErrorDataset,
    cssVarMap,
  };
}
