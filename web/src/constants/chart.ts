export const CHART_CONFIG = {
  width: 800,
  height: 260,
  padding: { top: 40, right: 40, bottom: 60, left: 80 } as const,
} as const;

export const PAGINATION = {
  defaultPageSize: 12,
  logPageSize: 15,
  keyPageSizes: [12, 24, 60, 120] as const,
  pageSizes: [12, 24, 48, 96] as const,
} as const;

export const TIME_RANGES = {
  hour1: 1,
  hour5: 5,
  week1: 168,
  month1: 720,
} as const;

export type TimeRangeValue = 1 | 5 | 168 | 720;

export interface TimeRangeOption {
  label: string;
  value: TimeRangeValue;
  type?: string;
  [key: string]: unknown;
}

export const NUMBER_FORMAT = {
  million: 1_000_000,
  thousand: 1_000,
} as const;

export const CHART_LABEL_STEPS = {
  dayRange: 2,
  weekRange: 12,
  monthRange: 48,
} as const;

export const Y_AXIS = {
  tickCount: 5,
  defaultMax: 100,
  minPaddingRatio: 0.1,
} as const;

export const HOVER_DISTANCE = 50;

export const ANIMATION_DELAY = 100;
