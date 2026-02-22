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

// Token speed color palette - Cyber Speed Heat Gradient (Top 10)
// From hottest (fastest) to coolest (slowest): Orange → Gold → Lime → Cyan → Blue
const tokenSpeedColors = [
  "#FF6B35", // Rank 1: Incinerator Orange - blazing fast
  "#FF8C42", // Rank 2: Solar Flare
  "#FFB347", // Rank 3: Golden Burst
  "#FFD23F", // Rank 4: Electric Gold
  "#7ED957", // Rank 5: Neon Lime
  "#00E676", // Rank 6: Matrix Green
  "#00D9FF", // Rank 7: Arctic Cyan
  "#00B4D8", // Rank 8: Glacial Blue
  "#4A90E2", // Rank 9: Ocean Depth
  "#5C6BC0", // Rank 10: Deep Twilight - cool cruise
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
