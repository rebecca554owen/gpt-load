import type { ChartData, DashboardStatsResponse, Group } from "@/types/models";
import http from "@/utils/http";

/**
 * Get dashboard basic statistics
 * @param days Time range (1/7/30 days)
 */
export const getDashboardStats = (days: number = 1) => {
  return http.get<DashboardStatsResponse>("/dashboard/stats", {
    params: { days },
  });
};

/**
 * Get dashboard chart data
 * @param view View type (request/token)
 * @param hours Time range in hours (1/3/6/24/72/168)
 */
export const getDashboardChart = (view: "request" | "token" = "token", hours: number = 24) => {
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
