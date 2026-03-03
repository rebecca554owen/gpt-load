import type { ChartData, ChartViewType, DashboardStatsResponse, Group } from "@/types/models";
import http from "@/utils/http";

interface ApiError {
  code: number;
  message: string;
  error?: string;
}

export type ApiResult<T> = { data: T } | ApiError;

/**
 * 获取仪表板基本统计数据
 * @param hours 时间范围（小时）(1/5/24/168/720)
 */
export const getDashboardStats = (hours: number = 5) => {
  return http.get<DashboardStatsResponse>("/dashboard/stats", {
    params: { hours },
  });
};

/**
 * 获取仪表板图表数据
 * @param view 视图类型 (request/token/token_speed)
 * @param hours 时间范围（小时）(1/5/24/168/720)
 */
export const getDashboardChart = (view: ChartViewType = "token", hours: number = 5) => {
  return http.get<ChartData>("/dashboard/chart", {
    params: { view, hours },
  });
};

/**
 * 获取用于过滤的组列表
 */
export const getGroupList = () => {
  return http.get<Group[]>("/groups/list");
};
