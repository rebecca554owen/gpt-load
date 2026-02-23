import { formatDuration } from "@/utils/format";

export interface TestResult {
  duration: number;
}

export interface UseTestButtonStylesOptions {
  testResults: Record<string, TestResult>;
  isTesting: (key: string) => boolean;
  hasFailed: (key: string) => boolean;
}

export type ButtonType = "default" | "success" | "info" | "warning" | "error";

export interface TestButtonStyles {
  getButtonType: (key: string) => ButtonType;
  getButtonClass: (key: string) => string;
  getButtonText: (key: string, defaultText: string) => string;
}

const THRESHOLDS = {
  SUPER_FAST: 200,
  EXCELLENT: 500,
  FAST: 1000,
  NORMAL: 2000,
  SLOW: 3000,
  VERY_SLOW: 4000,
  EXTREMELY_SLOW: 5000,
} as const;

function getButtonTypeByDuration(duration: number): ButtonType {
  if (duration < THRESHOLDS.SUPER_FAST) {
    return "success";
  }
  if (duration < THRESHOLDS.EXCELLENT) {
    return "info";
  }
  if (duration < THRESHOLDS.FAST) {
    return "warning";
  }
  if (duration < THRESHOLDS.NORMAL) {
    return "default";
  }
  if (duration < THRESHOLDS.SLOW) {
    return "warning";
  }
  if (duration < THRESHOLDS.VERY_SLOW) {
    return "error";
  }
  if (duration < THRESHOLDS.EXTREMELY_SLOW) {
    return "error";
  }
  return "default";
}

function getCssClassByDuration(duration: number): string {
  if (duration < THRESHOLDS.SUPER_FAST) {
    return "test-super-fast";
  }
  if (duration < THRESHOLDS.EXCELLENT) {
    return "test-excellent";
  }
  if (duration < THRESHOLDS.FAST) {
    return "test-fast";
  }
  if (duration < THRESHOLDS.NORMAL) {
    return "test-normal";
  }
  if (duration < THRESHOLDS.SLOW) {
    return "test-slow";
  }
  if (duration < THRESHOLDS.VERY_SLOW) {
    return "test-very-slow";
  }
  if (duration < THRESHOLDS.EXTREMELY_SLOW) {
    return "test-extremely-slow";
  }
  return "test-timeout";
}

export function useTestButtonStyles(options: UseTestButtonStylesOptions): TestButtonStyles {
  const { testResults, isTesting, hasFailed } = options;

  function getButtonType(key: string): ButtonType {
    if (isTesting(key)) {
      return "default";
    }
    if (hasFailed(key)) {
      return "error";
    }

    const result = testResults[key];
    if (!result) {
      return "default";
    }

    return getButtonTypeByDuration(result.duration);
  }

  function getButtonClass(key: string): string {
    if (isTesting(key)) {
      return "";
    }
    if (hasFailed(key)) {
      return "test-failed";
    }

    const result = testResults[key];
    if (!result) {
      return "";
    }

    return getCssClassByDuration(result.duration);
  }

  function getButtonText(key: string, defaultText: string): string {
    if (isTesting(key)) {
      return defaultText;
    }

    const result = testResults[key];
    if (!result) {
      return defaultText;
    }

    return formatDuration(result.duration);
  }

  return {
    getButtonType,
    getButtonClass,
    getButtonText,
  };
}

export default useTestButtonStyles;
