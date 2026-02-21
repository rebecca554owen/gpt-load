// Chart colors mapping
const cssVarMap: Record<string, string> = {
  "dashboard.success_requests": "--color-request-count",
  "dashboard.failed_requests": "--color-error-rate",
  "dashboard.non_cached_prompt_tokens": "--color-prompt-tokens",
  "dashboard.prompt_tokens": "--color-prompt-tokens",
  "dashboard.completion_tokens": "--color-completion-tokens",
  "dashboard.cached_tokens": "--color-cached-tokens",
  "dashboard.total_tokens": "--color-total-tokens",
  "dashboard.input_cached_tokens": "--color-cached-tokens",
  "dashboard.input_non_cached_tokens": "--color-prompt-tokens",
};

// Token speed color palette - top 7 colors
const tokenSpeedColors = [
  "#087EA4", // Rank 1: Deepest teal - fastest
  "#0A9EC1", // Rank 2
  "#0CBEDD", // Rank 3
  "#14B8A6", // Rank 4: Teal
  "#1AB5A0", // Rank 5
  "#26AF94", // Rank 6
  "#32A988", // Rank 7: Lightest teal - slowest
];

export function useChartColors() {
  const getDatasetColor = (
    labelKey: string | undefined,
    label: string,
    speedIndex?: number
  ): string => {
    const key = labelKey || label;

    // Token speed charts use position-based heat gradient
    // Backend already sorts datasets by average speed (descending)
    if (key.startsWith("token_speed.") && speedIndex !== undefined) {
      return tokenSpeedColors[speedIndex] || tokenSpeedColors[tokenSpeedColors.length - 1];
    }

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

    // Only try to translate when label looks like an i18n key:
    // - Contains dot
    // - NOT from token_speed (format: "token_speed.xxx")
    // - NOT a dynamic combination (format: "group - model")
    // - Starts with known prefix like "dashboard."
    if (
      label.includes(".") &&
      !label.startsWith("token_speed.") &&
      !label.includes(" - ") &&
      (label.startsWith("dashboard.") || label.startsWith("common."))
    ) {
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
