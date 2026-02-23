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

const tokenSpeedColors = [
  "#FF6B35",
  "#FF8C42",
  "#FFB347",
  "#FFD23F",
  "#7ED957",
  "#00E676",
  "#00D9FF",
  "#00B4D8",
  "#4A90E2",
  "#5C6BC0",
];

export function useChartColors() {
  const getDatasetColor = (
    labelKey: string | undefined,
    label: string,
    speedIndex?: number
  ): string => {
    const key = labelKey || label;

    if (key.startsWith("token_speed.") && speedIndex !== undefined) {
      return tokenSpeedColors[speedIndex] || tokenSpeedColors[tokenSpeedColors.length - 1];
    }

    if (cssVarMap[key]) {
      return `var(${cssVarMap[key]})`;
    }
    return "var(--color-request-count)";
  };

  const isErrorDataset = (label: string, t: (key: string) => string): boolean => {
    if (
      label.includes("失败") ||
      label.includes("Failed") ||
      label.includes("失敗") ||
      label.includes("Error") ||
      label.includes("エラー")
    ) {
      return true;
    }

    if (
      label.includes(".") &&
      !label.startsWith("token_speed.") &&
      !label.includes(" - ") &&
      (label.startsWith("dashboard.") || label.startsWith("common."))
    ) {
      try {
        const translatedLabel = t(label);
        if (
          translatedLabel.includes("失败") ||
          translatedLabel.includes("Error") ||
          translatedLabel.includes("エラー")
        ) {
          return true;
        }
      } catch {
        // Ignore translation errors
      }
    }

    return false;
  };

  return {
    getDatasetColor,
    isErrorDataset,
  };
}
