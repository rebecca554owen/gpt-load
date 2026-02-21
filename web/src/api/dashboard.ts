import type { ChartData, ChartViewType, DashboardStatsResponse, Group } from "@/types/models";
import http from "@/utils/http";

/**
 * Get dashboard basic statistics
 * @param hours Time range in hours (1/5/24/168/720)
 */
export const getDashboardStats = (hours: number = 5) => {
  return http.get<DashboardStatsResponse>("/dashboard/stats", {
    params: { hours },
  });
};

/**
 * Get dashboard chart data
 * @param view View type (request/token/token_speed)
 * @param hours Time range in hours (1/5/24/168/720)
 */
export const getDashboardChart = (view: ChartViewType = "token", hours: number = 5) => {
  return http.get<ChartData>("/dashboard/chart", {
    params: { view, hours },
  });
};

/**
 * Get group list for filtering
 */
export const getGroupList = () => {
  return http.get<Group[]>("/groups/list");
};
