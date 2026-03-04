import i18n from "@/locales";
import { useAuthService } from "@/composables/useAuth";
import axios from "axios";
import { useAppStateStore } from "@/stores/appState";

// 定义不需要加载指示器的 API URL 列表
const noLoadingUrls = ["/tasks/status"];

declare module "axios" {
  interface AxiosRequestConfig {
    hideMessage?: boolean;
  }
}

const http = axios.create({
  baseURL: "/api",
  timeout: 60000,
  headers: { "Content-Type": "application/json" },
});

// 请求拦截器
http.interceptors.request.use(config => {
  try {
    const appState = useAppStateStore();
    if (config.url && !noLoadingUrls.includes(config.url)) {
      appState.loading = true;
    }
  } catch {
    // Pinia 尚未初始化，跳过加载状态
  }
  const authKey = localStorage.getItem("authKey");
  if (authKey) {
    config.headers.Authorization = `Bearer ${authKey}`;
  }
  const locale = localStorage.getItem("locale") || "zh-CN";
  config.headers["Accept-Language"] = locale;
  return config;
});

// 响应拦截器
http.interceptors.response.use(
  response => {
    try {
      const appState = useAppStateStore();
      appState.loading = false;
    } catch {
      // Pinia 尚未初始化，跳过
    }
    if (response.config.method !== "get" && !response.config.hideMessage) {
      window.$message.success(response.data.message ?? i18n.global.t("common.operationSuccess"));
    }
    return response.data;
  },
  error => {
    try {
      const appState = useAppStateStore();
      appState.loading = false;
    } catch {
      // Pinia 尚未初始化，跳过
    }
    if (error.response) {
      if (error.response.status === 401) {
        if (window.location.pathname !== "/login") {
          const { logout } = useAuthService();
          logout();
          window.location.href = "/login";
        }
      }
      window.$message.error(
        error.response.data?.message ||
          i18n.global.t("common.requestFailed", { status: error.response.status }),
        {
          keepAliveOnHover: true,
          duration: 5000,
          closable: true,
        }
      );
    } else if (error.request) {
      window.$message.error(i18n.global.t("common.networkError"));
    } else {
      window.$message.error(i18n.global.t("common.requestSetupError"));
    }
    return Promise.reject(error);
  }
);

export default http;
